package com.novpn.obfs

import android.content.Context

class ObfuscationSeedStore(context: Context) {
    private val preferences = context.getSharedPreferences("novpn_obfs", Context.MODE_PRIVATE)

    fun loadOrSaveDefault(defaultSeed: String): String {
        val existing = preferences.getString(KEY_SEED, null)
        if (!existing.isNullOrBlank() && !existing.contains("replace-with", ignoreCase = true)) {
            return existing
        }

        preferences.edit().putString(KEY_SEED, defaultSeed).apply()
        return defaultSeed
    }

    companion object {
        private const val KEY_SEED = "seed"
    }
}
