package com.novpn.split

import android.content.Context
import com.novpn.data.InstalledApp

class InstalledAppsScanner(private val context: Context) {

    fun loadLaunchableApps(limit: Int = 50): List<InstalledApp> {
        return context.packageManager.getInstalledApplications(0)
            .asSequence()
            .filter { applicationInfo ->
                context.packageManager.getLaunchIntentForPackage(applicationInfo.packageName) != null
            }
            .map { applicationInfo ->
                InstalledApp(
                    packageName = applicationInfo.packageName,
                    label = context.packageManager.getApplicationLabel(applicationInfo).toString()
                )
            }
            .sortedBy { it.label.lowercase() }
            .take(limit)
            .toList()
    }
}
