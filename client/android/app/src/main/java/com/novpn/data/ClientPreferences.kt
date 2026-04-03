package com.novpn.data

import android.content.Context

class ClientPreferences(context: Context) {
    private val preferences =
        context.getSharedPreferences(PREFERENCE_FILE, Context.MODE_PRIVATE)

    fun isBypassRuEnabled(): Boolean {
        return preferences.getBoolean(KEY_BYPASS_RU, true)
    }

    fun excludedPackages(): List<String> {
        return preferences.getStringSet(KEY_EXCLUDED_PACKAGES, emptySet())
            .orEmpty()
            .sorted()
    }

    fun selectedProfileAsset(defaultAsset: String): String {
        return preferences.getString(KEY_SELECTED_PROFILE, defaultAsset) ?: defaultAsset
    }

    fun saveBypassRu(enabled: Boolean) {
        preferences.edit().putBoolean(KEY_BYPASS_RU, enabled).apply()
    }

    fun saveExcludedPackages(packageNames: List<String>) {
        preferences.edit()
            .putStringSet(KEY_EXCLUDED_PACKAGES, packageNames.distinct().toSet())
            .apply()
    }

    fun saveSelectedProfileAsset(assetName: String) {
        preferences.edit().putString(KEY_SELECTED_PROFILE, assetName).apply()
    }

    companion object {
        private const val PREFERENCE_FILE = "novpn_client_preferences"
        private const val KEY_BYPASS_RU = "bypass_ru"
        private const val KEY_EXCLUDED_PACKAGES = "excluded_packages"
        private const val KEY_SELECTED_PROFILE = "selected_profile"
    }
}
