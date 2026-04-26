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
import android.app.Application
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import com.novpn.R
import com.novpn.data.AppRoutingMode
import com.novpn.data.DeviceIdentityStore
import com.novpn.data.NetworkDiagnosticsRunner
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
import java.util.concurrent.atomic.AtomicInteger


class NoVpnService : VpnService() {
    private val tun2ProxyBridge by lazy { Tun2ProxyBridge(applicationContext) }
    private val profileRepository by lazy { ProfileRepository(this) }
    private val seedStore by lazy { ObfuscationSeedStore(this) }
    private val deviceIdentityStore by lazy { DeviceIdentityStore(this) }
    private val xrayConfigWriter by lazy { AndroidXrayConfigWriter(this) }
    private val obfuscatorConfigWriter by lazy { ObfuscatorConfigWriter(this) }
    private val runtimeManager by lazy { EmbeddedRuntimeManager(this) }
    private val runtimeStatusStore by lazy { VpnRuntimeStatusStore(this) }
    private val preflightChecker by lazy { RuntimePreflightChecker(this) }
    private val diagnosticsRunner by lazy { NetworkDiagnosticsRunner() }
    private val worker: ExecutorService = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "novpn-service-worker").apply { isDaemon = true }
    }
    private val mainHandler by lazy { Handler(Looper.getMainLooper()) }
    private val coreLock = Any()
    private val latestStartId = AtomicInteger(0)
    private var tunnelInterface: ParcelFileDescriptor? = null
    private var activeBridgeProxy: RuntimeLocalProxyConfig? = null
    @Volatile
    private var coreSessionActive = false
    private val runtimeHealthWatchdog = object : Runnable {
        override fun run() {
            worker.execute {
                monitorRuntimeHealth()
            }
            if (coreSessionActive) {
                mainHandler.postDelayed(this, RUNTIME_HEALTH_CHECK_INTERVAL_MS)
            }
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        latestStartId.updateAndGet { maxOf(it, startId) }

        when (intent?.action) {
            ACTION_START -> {
                val profileId = intent.getStringExtra(EXTRA_PROFILE_ID)
                    ?.takeIf { it.isNotBlank() }
                    ?: run {
                        runtimeStatusStore.markFailed(
                            status = getString(R.string.runtime_start_failed),
                            detail = getString(R.string.runtime_profile_incomplete)
                        )
                        mainHandler.post {
                            stopServiceForStartId(startId)
                        }
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
                runtimeManager.appendAppLog(
                    "service",
                    "Received START command startId=$startId profileId=$profileId bypassRu=$bypassRu " +
                        "routing=${appRoutingMode.storageValue} selectedApps=${selectedPackages.size} " +
                        "traffic=${trafficStrategy.storageValue} pattern=${patternStrategy.storageValue}"
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
                        stopCore()
                        if (!isLatestCommand(startId)) {
                            return@onFailure
                        }
                        runtimeStatusStore.markFailed(
                            status = getString(R.string.runtime_start_failed),
                            detail = buildFailureDetail(it)
                        )
                        mainHandler.post {
                            stopServiceForStartId(startId)
                        }
                    }
                }
                return START_STICKY
            }

            ACTION_STOP -> {
                runtimeManager.appendAppLog("service", "Received STOP command startId=$startId")
                runtimeStatusStore.markStopping()
                worker.execute {
                    stopCore()
                    runtimeStatusStore.markStopped(getString(R.string.service_stopped))
                    RuntimeLocalProxySession.update(null)
                    mainHandler.post {
                        stopServiceForStartId(startId)
                    }
                }
                return START_NOT_STICKY
            }

            else -> return START_NOT_STICKY
        }
    }

    override fun onDestroy() {
        runtimeManager.appendAppLog("service", "Service onDestroy invoked")
        runCatching { stopCore() }
        runtimeStatusStore.markStopped(getString(R.string.service_stopped))
        RuntimeLocalProxySession.update(null)
        worker.shutdownNow()
        super.onDestroy()
        maybeTerminateDedicatedProcess()
    }

    override fun onRevoke() {
        runtimeManager.appendAppLog("service", "VPN permission revoked by system")
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
            .setBlocking(false)
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
        synchronized(coreLock) {
            stopCoreLocked()
            preflightChecker.evaluate(profileId).requireReady()
            runtimeManager.appendAppLog("service", "Preflight passed for profileId=$profileId")

            val profile = profileRepository.loadProfile(profileId)
            profile.requireRuntimeReady()
            val effectiveProfile = profile.withObfuscationSeed(
                seedStore.loadOrSaveDefault(profile.obfuscation.seed)
            ).withRuntimeStrategies(trafficStrategy, patternStrategy)
            val useSimplifiedBridgePath = shouldUseSimplifiedBridgePath(appRoutingMode, selectedPackages)
            val localProxy = RuntimeLocalProxyFactory.createProtected(udpEnabled = true)
            val xrayInboundProxy = RuntimeLocalProxyFactory.createProtected(udpEnabled = true)
            var bridgeProxy = if (useSimplifiedBridgePath) xrayInboundProxy else localProxy
            coreSessionActive = true
            runtimeManager.appendAppLog(
                "service",
                "Starting core for profile=${profile.name}, server=${effectiveProfile.server.address}:${effectiveProfile.server.port}, " +
                    "simplifiedBridgePath=$useSimplifiedBridgePath"
            )

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
                bridgeProxy = resolveBridgeProxy(
                    useSimplifiedBridgePath = useSimplifiedBridgePath,
                    localProxy = localProxy,
                    xrayInboundProxy = xrayInboundProxy
                )
                diagnosticsRunner.verifyTunnel(
                    profile = effectiveProfile,
                    proxy = bridgeProxy,
                    apiBaseFallback = profileRepository.bootstrapApiBase(),
                    startupProbe = true
                )
                runtimeManager.appendAppLog("service", "Tunnel diagnostics check passed")
                activeBridgeProxy = bridgeProxy
                tunnelInterface = establishTunnel(
                    appRoutingMode = appRoutingMode,
                    packageNames = selectedPackages,
                    upstreamAddress = effectiveProfile.server.address
                )
                tunnelInterface?.let {
                    runtimeManager.appendAppLog("service", "Android VPN tunnel established, starting tun2proxy")
                    tun2ProxyBridge.start(it, bridgeProxy, TUN_MTU)
                    tun2ProxyBridge.confirmStarted()
                }
                    ?: throw IllegalStateException("Failed to establish Android VPN tunnel interface.")
            } catch (error: Exception) {
                val detail = buildFailureDetail(error)
                runtimeManager.appendAppLog("service", "Core start failed: $detail")
                stopCoreLocked()
                throw IllegalStateException(detail, error)
            }

            runtimeManager.appendAppLog("service", "Core runtime started successfully")
            runtimeStatusStore.markRunning(
                status = getString(R.string.runtime_active_profile, effectiveProfile.name),
                detail = getString(R.string.runtime_running_detail)
            )
            RuntimeLocalProxySession.update(bridgeProxy)
            startForegroundRuntime(getString(R.string.runtime_active_profile, effectiveProfile.name))
            scheduleRuntimeHealthWatchdog()
        }
    }

    private fun resolveBridgeProxy(
        useSimplifiedBridgePath: Boolean,
        localProxy: RuntimeLocalProxyConfig,
        xrayInboundProxy: RuntimeLocalProxyConfig
    ): RuntimeLocalProxyConfig {
        if (useSimplifiedBridgePath) {
            tun2ProxyBridge.waitForLocalProxy(xrayInboundProxy)
            runtimeManager.appendAppLog("service", "Using direct local Xray bridge for this VPN session")
            return xrayInboundProxy
        }

        try {
            tun2ProxyBridge.waitForLocalProxy(localProxy)
            val udpAssociateFailure = diagnosticsRunner.probeUdpAssociateSupport(localProxy)
            if (udpAssociateFailure == null) {
                runtimeManager.appendAppLog("service", "Using obfuscator SOCKS bridge (UDP ASSOCIATE probe passed)")
                return localProxy
            }

            runtimeManager.appendAppLog(
                "service",
                "Local obfuscator SOCKS bridge failed UDP probe: $udpAssociateFailure. Falling back to direct local Xray bridge"
            )
        } catch (error: Exception) {
            runtimeManager.appendAppLog(
                "service",
                "Local obfuscator SOCKS bridge unavailable: ${error.message ?: error.javaClass.simpleName}. " +
                    "Falling back to direct local Xray bridge"
            )
        }

        tun2ProxyBridge.waitForLocalProxy(xrayInboundProxy)
        return xrayInboundProxy
    }

    private fun stopCore() {
        synchronized(coreLock) {
            stopCoreLocked()
        }
    }

    private fun stopCoreLocked() {
        cancelRuntimeHealthWatchdog()
        if (!coreSessionActive && tunnelInterface == null && !runtimeManager.isRunning()) {
            activeBridgeProxy = null
            return
        }
        runtimeManager.appendAppLog("service", "Stopping core runtime")
        coreSessionActive = false
        val tunnelToClose = tunnelInterface
        tunnelInterface = null
        if (tunnelToClose != null) {
            runtimeManager.appendAppLog("service", "Closing Android VPN tunnel interface before tun2proxy stop")
        }
        runCatching { tunnelToClose?.close() }
        tun2ProxyBridge.stop()
        runtimeManager.stop()
        activeBridgeProxy = null
        RuntimeLocalProxySession.update(null)
    }

    private fun scheduleRuntimeHealthWatchdog() {
        mainHandler.removeCallbacks(runtimeHealthWatchdog)
        mainHandler.postDelayed(runtimeHealthWatchdog, RUNTIME_HEALTH_CHECK_INTERVAL_MS)
    }

    private fun cancelRuntimeHealthWatchdog() {
        mainHandler.removeCallbacks(runtimeHealthWatchdog)
    }

    private fun monitorRuntimeHealth() {
        synchronized(coreLock) {
            if (!coreSessionActive) {
                return
            }
            val bridgeProxy = activeBridgeProxy
            val runtimeFailure = runtimeManager.healthFailureDetail()
            val proxyHealthy = bridgeProxy != null && tun2ProxyBridge.isLocalProxyReachable(bridgeProxy)
            if (runtimeFailure == null && proxyHealthy) {
                return
            }

            val detail = runtimeFailure ?: "Local VPN bridge stopped accepting connections."
            runtimeManager.appendAppLog("service", "Runtime health watchdog detected failure: $detail")
            runtimeStatusStore.markFailed(
                status = getString(R.string.runtime_start_failed),
                detail = detail
            )
            stopCoreLocked()
            mainHandler.post {
                stopServiceForStartId(latestStartId.get())
            }
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

    private fun shouldUseSimplifiedBridgePath(
        appRoutingMode: AppRoutingMode,
        selectedPackages: List<String>
    ): Boolean {
        // The simplified bridge is session-wide. Once enabled by a single selected app
        // (for example YouTube), every routed app bypasses the obfuscator and uses the
        // direct local Xray path. That is too broad for allow-list mode and can break
        // unrelated apps such as Telegram. Keep the obfuscator -> Xray chain as the
        // default session path, then let runtime capability probing fall back to the
        // direct local Xray bridge when the packaged obfuscator cannot carry UDP.
        return false
    }

    private fun resolveInstalledSimplifiedRoutePackages(): Set<String> {
        val installedPackages = runCatching {
            packageManager.getInstalledPackages(0).map { it.packageName }
        }.getOrDefault(emptyList())

        return installedPackages.filterTo(linkedSetOf()) { packageName ->
            isSimplifiedRoutePackage(packageName)
        }
    }

    private fun isSimplifiedRoutePackage(packageName: String): Boolean {
        if (packageName in SIMPLIFIED_ROUTE_EXACT_PACKAGES) {
            return true
        }
        if (SIMPLIFIED_ROUTE_PREFIXES.any { prefix -> packageName.startsWith(prefix) }) {
            return true
        }
        return packageName.contains(".youtube") ||
            packageName.contains("youtube.") ||
            packageName.contains(".brawlstars") ||
            packageName.contains("brawlstars.")
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

    private fun isLatestCommand(startId: Int): Boolean {
        return startId == latestStartId.get()
    }

    private fun stopServiceForStartId(startId: Int) {
        if (!stopSelfResult(startId)) {
            return
        }
        runCatching {
            stopForeground(STOP_FOREGROUND_REMOVE)
        }
    }

    private fun maybeTerminateDedicatedProcess() {
        val processName = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            Application.getProcessName()
        } else {
            return
        }
        if (!processName.endsWith(DEDICATED_PROCESS_SUFFIX)) {
            return
        }
        runtimeManager.appendAppLog("service", "Terminating dedicated VPN process $processName")
        android.os.Process.killProcess(android.os.Process.myPid())
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
        private const val RUNTIME_HEALTH_CHECK_INTERVAL_MS = 2_500L
        private const val DEDICATED_PROCESS_SUFFIX = ":vpncore"
        private val SIMPLIFIED_ROUTE_EXACT_PACKAGES = setOf(
            "com.google.android.youtube",
            "com.google.android.apps.youtube.music",
            "com.google.android.apps.youtube.kids",
            "com.google.android.youtube.tv",
            "com.google.android.youtube.googletv",
            "com.supercell.brawlstars"
        )
        private val SIMPLIFIED_ROUTE_PREFIXES = setOf(
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
