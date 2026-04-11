package com.novpn.vpn

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.net.IpPrefix
import android.net.VpnService
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.ParcelFileDescriptor
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import com.novpn.R
import com.novpn.data.AppRoutingMode
import com.novpn.data.DeviceIdentityStore
import com.novpn.data.PatternMaskingStrategy
import com.novpn.data.ProfileRepository
import com.novpn.data.TrafficObfuscationStrategy
import com.novpn.data.requireRuntimeReady
import com.novpn.data.withObfuscationSeed
import com.novpn.data.withRuntimeStrategies
import com.novpn.obfs.ObfuscationSeedStore
import com.novpn.obfs.SessionObfuscationPlanner
import com.novpn.ui.MainActivity
import com.novpn.xray.AndroidXrayConfigWriter
import java.net.InetAddress
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors


class NoVpnService : VpnService() {
    private val tun2ProxyBridge by lazy { Tun2ProxyBridge() }
    private val profileRepository by lazy { ProfileRepository(this) }
    private val seedStore by lazy { ObfuscationSeedStore(this) }
    private val deviceIdentityStore by lazy { DeviceIdentityStore(this) }
    private val xrayConfigWriter by lazy { AndroidXrayConfigWriter(this) }
    private val obfuscatorConfigWriter by lazy { ObfuscatorConfigWriter(this) }
    private val runtimeManager by lazy { EmbeddedRuntimeManager(this) }
    private val runtimeStatusStore by lazy { VpnRuntimeStatusStore(this) }
    private val preflightChecker by lazy { RuntimePreflightChecker(this) }
    private val worker: ExecutorService = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "novpn-service-worker").apply { isDaemon = true }
    }
    private val mainHandler by lazy { Handler(Looper.getMainLooper()) }
    private val coreLock = Any()
    private var tunnelInterface: ParcelFileDescriptor? = null
    @Volatile
    private var coreSessionActive = false

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                val profileId = intent.getStringExtra(EXTRA_PROFILE_ID)
                    ?.takeIf { it.isNotBlank() }
                    ?: run {
                        runtimeStatusStore.markFailed(
                            status = getString(R.string.runtime_start_failed),
                            detail = getString(R.string.runtime_profile_incomplete)
                        )
                        stopSelf()
                        return START_NOT_STICKY
                    }
                val bypassRu = intent.getBooleanExtra(EXTRA_BYPASS_RU, true)
                val appRoutingMode = AppRoutingMode.fromStorage(intent.getStringExtra(EXTRA_APP_ROUTING_MODE))
                val selectedPackages = intent.getStringArrayListExtra(EXTRA_SELECTED_PACKAGES).orEmpty()
                val trafficStrategy = TrafficObfuscationStrategy.fromStorage(
                    intent.getStringExtra(EXTRA_TRAFFIC_STRATEGY)
                )
                val patternStrategy = PatternMaskingStrategy.fromStorage(
                    intent.getStringExtra(EXTRA_PATTERN_STRATEGY)
                )
                startForegroundRuntime(getString(R.string.runtime_starting))
                runtimeStatusStore.markStarting(
                    status = getString(R.string.runtime_starting),
                    detail = getString(R.string.runtime_starting_detail)
                )
                worker.execute {
                    runCatching {
                        startCore(
                            profileId = profileId,
                            bypassRu = bypassRu,
                            appRoutingMode = appRoutingMode,
                            selectedPackages = selectedPackages,
                            trafficStrategy = trafficStrategy,
                            patternStrategy = patternStrategy
                        )
                    }.onFailure {
                        runtimeStatusStore.markFailed(
                            status = getString(R.string.runtime_start_failed),
                            detail = buildFailureDetail(it)
                        )
                        stopCore()
                        mainHandler.post {
                            stopForeground(STOP_FOREGROUND_REMOVE)
                            stopSelf()
                        }
                    }
                }
            }

            ACTION_STOP -> {
                worker.execute {
                    runtimeStatusStore.markStopped(getString(R.string.service_stopped))
                    RuntimeLocalProxySession.update(null)
                    stopCore()
                    mainHandler.post {
                        stopForeground(STOP_FOREGROUND_REMOVE)
                        stopSelf()
                    }
                }
            }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        runCatching { stopCore() }
        runtimeStatusStore.markStopped(getString(R.string.service_stopped))
        RuntimeLocalProxySession.update(null)
        worker.shutdownNow()
        super.onDestroy()
    }

    override fun onRevoke() {
        worker.execute {
            stopCore()
            runtimeStatusStore.markStopped(
                status = getString(R.string.status_permission_required),
                detail = getString(R.string.status_permission_denied_detail)
            )
            RuntimeLocalProxySession.update(null)
        }
        stopSelf()
    }

    fun establishTunnel(
        appRoutingMode: AppRoutingMode,
        packageNames: List<String>,
        upstreamAddress: String
    ): ParcelFileDescriptor? {
        val mtu = TUN_MTU
        val builder = Builder()
            .setSession(getString(R.string.tunnel_session_name))
            .setMtu(mtu)
            .setBlocking(true)
            .addAddress(TUN_IPV4_ADDRESS, TUN_IPV4_PREFIX_LENGTH)
            .addAddress(TUN_IPV6_ADDRESS, TUN_IPV6_PREFIX_LENGTH)
            .addRoute("0.0.0.0", 0)
            .addRoute("::", 0)
            .addDnsServer(TUN_DNS_PRIMARY)
            .addDnsServer(TUN_DNS_SECONDARY)
            .allowFamily(android.system.OsConstants.AF_INET)
            .allowFamily(android.system.OsConstants.AF_INET6)
            .allowBypass()

        applyUpstreamBypassRoutes(builder, upstreamAddress)
        applyApplicationRouting(builder, appRoutingMode, packageNames)
        return builder.establish()
    }

    private fun startCore(
        profileId: String,
        bypassRu: Boolean,
        appRoutingMode: AppRoutingMode,
        selectedPackages: List<String>,
        trafficStrategy: TrafficObfuscationStrategy,
        patternStrategy: PatternMaskingStrategy
    ) {
        stopCore()
        preflightChecker.evaluate(profileId).requireReady()

        val profile = profileRepository.loadProfile(profileId)
        profile.requireRuntimeReady()
        val effectiveProfile = profile.withObfuscationSeed(
            seedStore.loadOrSaveDefault(profile.obfuscation.seed)
        ).withRuntimeStrategies(trafficStrategy, patternStrategy)
        val useSimplifiedYoutubePath = shouldUseSimplifiedYoutubePath(
            appRoutingMode = appRoutingMode,
            selectedPackages = selectedPackages
        )
        val localProxy = RuntimeLocalProxyFactory.createProtected(udpEnabled = false)
        val xrayInboundProxy = RuntimeLocalProxyFactory.createProtected(udpEnabled = useSimplifiedYoutubePath)
        val bridgeProxy = if (useSimplifiedYoutubePath) xrayInboundProxy else localProxy
        coreSessionActive = true

        try {
            val sessionPlan = SessionObfuscationPlanner.build(
                profile = effectiveProfile,
                deviceId = deviceIdentityStore.deviceId()
            )
            val xrayConfig = xrayConfigWriter.write(
                effectiveProfile,
                bypassRu,
                xrayInboundProxy,
                sessionPlan
            )
            val obfuscatorConfig = obfuscatorConfigWriter.write(
                effectiveProfile,
                xrayConfig,
                localProxy,
                xrayInboundProxy,
                sessionPlan
            )
            runtimeManager.start(xrayConfig, obfuscatorConfig)
            tun2ProxyBridge.waitForLocalProxy(bridgeProxy)
            tunnelInterface = establishTunnel(
                appRoutingMode = appRoutingMode,
                packageNames = selectedPackages,
                upstreamAddress = effectiveProfile.server.address
            )
            tunnelInterface?.let {
                tun2ProxyBridge.start(it, bridgeProxy, TUN_MTU)
                tun2ProxyBridge.confirmStarted()
            }
                ?: throw IllegalStateException("Failed to establish Android VPN tunnel interface.")
        } catch (error: Exception) {
            throw IllegalStateException(buildFailureDetail(error), error)
        }

        runtimeStatusStore.markRunning(
            status = getString(R.string.runtime_active_profile, effectiveProfile.name),
            detail = getString(R.string.runtime_running_detail)
        )
        RuntimeLocalProxySession.update(bridgeProxy)
        startForegroundRuntime(getString(R.string.runtime_active_profile, effectiveProfile.name))
    }

    private fun stopCore() {
        synchronized(coreLock) {
            if (!coreSessionActive && tunnelInterface == null && !runtimeManager.isRunning()) {
                return
            }
            coreSessionActive = false
            tun2ProxyBridge.stop()
            runtimeManager.stop()
            runCatching { tunnelInterface?.close() }
            tunnelInterface = null
            RuntimeLocalProxySession.update(null)
        }
    }

    private fun applyApplicationRouting(
        builder: Builder,
        mode: AppRoutingMode,
        packageNames: List<String>
    ) {
        when (mode) {
            AppRoutingMode.EXCLUDE_SELECTED -> {
                (packageNames + packageName).distinct().forEach { candidatePackage ->
                    if (isInstalled(candidatePackage)) {
                        builder.addDisallowedApplication(candidatePackage)
                    }
                }
            }

            AppRoutingMode.ONLY_SELECTED -> {
                packageNames.distinct().forEach { candidatePackage ->
                    if (isInstalled(candidatePackage)) {
                        builder.addAllowedApplication(candidatePackage)
                    }
                }
            }
        }
    }

    private fun shouldUseSimplifiedYoutubePath(
        appRoutingMode: AppRoutingMode,
        selectedPackages: List<String>
    ): Boolean {
        val youtubePackages = resolveInstalledYoutubePackages()
        if (youtubePackages.isEmpty()) {
            return false
        }

        val selectedSet = selectedPackages.toSet()
        return when (appRoutingMode) {
            AppRoutingMode.EXCLUDE_SELECTED -> youtubePackages.any { it !in selectedSet }
            AppRoutingMode.ONLY_SELECTED -> youtubePackages.any { it in selectedSet }
        }
    }

    private fun resolveInstalledYoutubePackages(): Set<String> {
        val installedPackages = runCatching {
            packageManager.getInstalledPackages(0).map { it.packageName }
        }.getOrDefault(emptyList())

        return installedPackages.filterTo(linkedSetOf()) { packageName ->
            isYoutubePackage(packageName)
        }
    }

    private fun isYoutubePackage(packageName: String): Boolean {
        if (packageName in YOUTUBE_EXACT_PACKAGES) {
            return true
        }
        if (YOUTUBE_PREFIXES.any { prefix -> packageName.startsWith(prefix) }) {
            return true
        }
        return packageName.contains(".youtube") || packageName.contains("youtube.")
    }

    private fun isInstalled(packageName: String): Boolean {
        return try {
            packageManager.getPackageInfo(packageName, 0)
            true
        } catch (_: PackageManager.NameNotFoundException) {
            false
        }
    }

    private fun applyUpstreamBypassRoutes(builder: Builder, upstreamAddress: String) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) {
            return
        }

        runCatching {
            InetAddress.getAllByName(upstreamAddress)
        }.getOrDefault(emptyArray()).forEach { address ->
            val prefixLength = when (address.address.size) {
                4 -> 32
                16 -> 128
                else -> return@forEach
            }
            builder.excludeRoute(IpPrefix(address, prefixLength))
        }
    }

    private fun startForegroundRuntime(contentText: String) {
        ensureNotificationChannel()
        val notification = buildNotification(contentText)
        ServiceCompat.startForeground(
            this,
            NOTIFICATION_ID,
            notification,
            ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE
        )
    }

    private fun buildFailureDetail(error: Throwable): String {
        val baseMessage = error.message ?: error.javaClass.simpleName
        val diagnostics = runtimeManager.diagnosticsSummary()
        return if (diagnostics.isBlank()) {
            baseMessage
        } else {
            "$baseMessage\n$diagnostics"
        }
    }

    private fun buildNotification(contentText: String) =
        NotificationCompat.Builder(this, NOTIFICATION_CHANNEL_ID)
            .setSmallIcon(android.R.drawable.stat_sys_warning)
            .setContentTitle(getString(R.string.notification_title))
            .setContentText(contentText)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setOngoing(true)
            .setContentIntent(
                PendingIntent.getActivity(
                    this,
                    1,
                    Intent(this, MainActivity::class.java),
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
                )
            )
            .build()

    private fun ensureNotificationChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
            return
        }

        val manager = getSystemService(NotificationManager::class.java)
        val channel = NotificationChannel(
            NOTIFICATION_CHANNEL_ID,
            getString(R.string.notification_channel_name),
            NotificationManager.IMPORTANCE_LOW
        )
        manager.createNotificationChannel(channel)
    }

    companion object {
        private const val ACTION_START = "com.novpn.vpn.START"
        private const val ACTION_STOP = "com.novpn.vpn.STOP"
        private const val EXTRA_PROFILE_ID = "extra_profile_id"
        private const val EXTRA_BYPASS_RU = "extra_bypass_ru"
        private const val EXTRA_APP_ROUTING_MODE = "extra_app_routing_mode"
        private const val EXTRA_SELECTED_PACKAGES = "extra_selected_packages"
        private const val EXTRA_TRAFFIC_STRATEGY = "extra_traffic_strategy"
        private const val EXTRA_PATTERN_STRATEGY = "extra_pattern_strategy"
        private const val NOTIFICATION_CHANNEL_ID = "novpn_runtime"
        private const val NOTIFICATION_ID = 1001
        private const val TUN_MTU = 1500
        private const val TUN_IPV4_ADDRESS = "198.18.0.1"
        private const val TUN_IPV4_PREFIX_LENGTH = 15
        private const val TUN_IPV6_ADDRESS = "fdfe:dcba:9876::1"
        private const val TUN_IPV6_PREFIX_LENGTH = 126
        private const val TUN_DNS_PRIMARY = "1.1.1.1"
        private const val TUN_DNS_SECONDARY = "8.8.8.8"
        private val YOUTUBE_EXACT_PACKAGES = setOf(
            "com.google.android.youtube",
            "com.google.android.apps.youtube.music",
            "com.google.android.apps.youtube.kids",
            "com.google.android.youtube.tv",
            "com.google.android.youtube.googletv"
        )
        private val YOUTUBE_PREFIXES = setOf(
            "com.google.android.apps.youtube.",
            "com.vanced.",
            "app.revanced.",
            "app.rvx."
        )

        fun startIntent(
            context: Context,
            profileId: String,
            bypassRu: Boolean,
            appRoutingMode: AppRoutingMode,
            selectedPackages: List<String>,
            trafficStrategy: TrafficObfuscationStrategy,
            patternStrategy: PatternMaskingStrategy
        ): Intent {
            return Intent(context, NoVpnService::class.java).apply {
                action = ACTION_START
                putExtra(EXTRA_PROFILE_ID, profileId)
                putExtra(EXTRA_BYPASS_RU, bypassRu)
                putExtra(EXTRA_APP_ROUTING_MODE, appRoutingMode.storageValue)
                putStringArrayListExtra(EXTRA_SELECTED_PACKAGES, ArrayList(selectedPackages))
                putExtra(EXTRA_TRAFFIC_STRATEGY, trafficStrategy.storageValue)
                putExtra(EXTRA_PATTERN_STRATEGY, patternStrategy.storageValue)
            }
        }

        fun stopIntent(context: Context): Intent {
            return Intent(context, NoVpnService::class.java).apply {
                action = ACTION_STOP
            }
        }
    }
}
