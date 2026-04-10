package com.novpn.split

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import com.novpn.data.AppRoutingMode
import com.novpn.data.ClientPreferences

class RuAppInstallReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        if (intent?.action != Intent.ACTION_PACKAGE_ADDED) {
            return
        }
        if (intent.getBooleanExtra(Intent.EXTRA_REPLACING, false)) {
            return
        }

        val packageName = intent.data?.schemeSpecificPart.orEmpty().trim()
        if (packageName.isBlank()) {
            return
        }

        val matcher = LocalRuAppExclusionMatcher(context)
        if (matcher.matchPackageName(packageName).isEmpty()) {
            return
        }

        val preferences = ClientPreferences(context)
        val updatedExcluded = (preferences.excludedPackages() + packageName)
            .distinct()
            .sorted()
        preferences.saveAppRoutingMode(AppRoutingMode.EXCLUDE_SELECTED)
        preferences.saveExcludedPackages(updatedExcluded)
        preferences.saveKnownInstalledPackages(preferences.knownInstalledPackages() + packageName)
    }
}
