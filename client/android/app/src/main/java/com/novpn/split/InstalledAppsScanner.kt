package com.novpn.split

import android.content.Context
import android.graphics.drawable.Drawable
import com.novpn.data.InstalledApp

data class InstalledAppEntry(
    val packageName: String,
    val label: String,
    val icon: Drawable?
)

class InstalledAppsScanner(private val context: Context) {

    fun loadLaunchableApps(limit: Int = 80): List<InstalledApp> {
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

    fun loadLaunchableEntries(limit: Int = Int.MAX_VALUE): List<InstalledAppEntry> {
        return context.packageManager.getInstalledApplications(0)
            .asSequence()
            .filter { applicationInfo ->
                context.packageManager.getLaunchIntentForPackage(applicationInfo.packageName) != null
            }
            .map { applicationInfo ->
                InstalledAppEntry(
                    packageName = applicationInfo.packageName,
                    label = context.packageManager.getApplicationLabel(applicationInfo).toString(),
                    icon = runCatching {
                        context.packageManager.getApplicationIcon(applicationInfo)
                    }.getOrNull()
                )
            }
            .sortedBy { it.label.lowercase() }
            .take(limit)
            .toList()
    }
}
