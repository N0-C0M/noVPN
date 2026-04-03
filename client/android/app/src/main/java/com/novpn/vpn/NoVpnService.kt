package com.novpn.vpn

import android.content.pm.PackageManager
import android.net.VpnService
import android.os.ParcelFileDescriptor

class NoVpnService : VpnService() {

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
}
