package com.novpn.vpn

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import com.novpn.data.ProfileRepository
import com.novpn.obfs.ObfuscationSeedStore
import com.novpn.ui.MainActivity
import com.novpn.xray.AndroidXrayConfigWriter


class NoVpnService : VpnService() {
    private val profileRepository by lazy { ProfileRepository(this) }
    private val seedStore by lazy { ObfuscationSeedStore(this) }
    private val xrayConfigWriter by lazy { AndroidXrayConfigWriter(this) }
    private val obfuscatorConfigWriter by lazy { ObfuscatorConfigWriter(this) }
    private val runtimeManager by lazy { EmbeddedRuntimeManager(this) }
    private var tunnelInterface: ParcelFileDescriptor? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                val profileAsset = intent.getStringExtra(EXTRA_PROFILE_ASSET)
                    ?: profileRepository.defaultProfileAsset()
                val bypassRu = intent.getBooleanExtra(EXTRA_BYPASS_RU, true)
                val excludedPackages = intent.getStringArrayListExtra(EXTRA_EXCLUDED_PACKAGES).orEmpty()
                startForegroundRuntime("Starting VPN runtime")
                startCore(profileAsset, bypassRu, excludedPackages)
            }

            ACTION_STOP -> {
                stopCore()
                stopForeground(STOP_FOREGROUND_REMOVE)
                stopSelf()
            }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        stopCore()
        super.onDestroy()
    }

    override fun onRevoke() {
        stopCore()
        stopSelf()
    }

    fun establishTunnel(disallowedPackages: List<String>): ParcelFileDescriptor? {
        val builder = Builder()
            .setSession("NoVPN")
            .addAddress("172.19.0.2", 32)
            .addRoute("0.0.0.0", 0)
            .addDnsServer("1.1.1.1")
            .allowFamily(android.system.OsConstants.AF_INET)
            .allowBypass()

        applyDisallowedApplications(builder, disallowedPackages)
        return builder.establish()
    }

    private fun startCore(
        profileAsset: String,
        bypassRu: Boolean,
        excludedPackages: List<String>
    ) {
        stopCore()

        val profile = profileRepository.loadProfile(profileAsset)
        seedStore.loadOrSaveDefault(profile.obfuscation.seed)

        val xrayConfig = xrayConfigWriter.write(profile, bypassRu)
        val obfuscatorConfig = obfuscatorConfigWriter.write(profile, xrayConfig)
        tunnelInterface = establishTunnel(excludedPackages)
        runtimeManager.start(xrayConfig, obfuscatorConfig)

        startForegroundRuntime("VPN runtime active: ${profile.name}")
    }

    private fun stopCore() {
        runtimeManager.stop()
        tunnelInterface?.close()
        tunnelInterface = null
    }

    private fun applyDisallowedApplications(
        builder: Builder,
        packageNames: List<String>
    ) {
        packageNames.distinct().forEach { packageName ->
            if (isInstalled(packageName)) {
                builder.addDisallowedApplication(packageName)
            }
        }
    }

    private fun isInstalled(packageName: String): Boolean {
        return try {
            packageManager.getPackageInfo(packageName, 0)
            true
        } catch (_: PackageManager.NameNotFoundException) {
            false
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

    private fun buildNotification(contentText: String) =
        NotificationCompat.Builder(this, NOTIFICATION_CHANNEL_ID)
            .setSmallIcon(android.R.drawable.stat_sys_warning)
            .setContentTitle("NoVPN")
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
            "NoVPN runtime",
            NotificationManager.IMPORTANCE_LOW
        )
        manager.createNotificationChannel(channel)
    }

    companion object {
        private const val ACTION_START = "com.novpn.vpn.START"
        private const val ACTION_STOP = "com.novpn.vpn.STOP"
        private const val EXTRA_PROFILE_ASSET = "extra_profile_asset"
        private const val EXTRA_BYPASS_RU = "extra_bypass_ru"
        private const val EXTRA_EXCLUDED_PACKAGES = "extra_excluded_packages"
        private const val NOTIFICATION_CHANNEL_ID = "novpn_runtime"
        private const val NOTIFICATION_ID = 1001

        fun startIntent(
            context: Context,
            profileAsset: String,
            bypassRu: Boolean,
            excludedPackages: List<String>
        ): Intent {
            return Intent(context, NoVpnService::class.java).apply {
                action = ACTION_START
                putExtra(EXTRA_PROFILE_ASSET, profileAsset)
                putExtra(EXTRA_BYPASS_RU, bypassRu)
                putStringArrayListExtra(EXTRA_EXCLUDED_PACKAGES, ArrayList(excludedPackages))
            }
        }

        fun stopIntent(context: Context): Intent {
            return Intent(context, NoVpnService::class.java).apply {
                action = ACTION_STOP
            }
        }
    }
}
