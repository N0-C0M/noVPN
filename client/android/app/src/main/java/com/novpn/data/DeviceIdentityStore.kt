package com.novpn.data

import android.content.Context
import android.os.Build
import android.provider.Settings
import java.util.Locale
import java.util.UUID

class DeviceIdentityStore(context: Context) {
    private val preferences = context.getSharedPreferences(PREFERENCE_FILE, Context.MODE_PRIVATE)
    private val appContext = context.applicationContext

    fun deviceId(): String {
        val stored = preferences.getString(KEY_DEVICE_ID, null).orEmpty().trim()
        if (stored.isNotBlank()) {
            return stored
        }

        val generated = buildDeviceId()
        preferences.edit().putString(KEY_DEVICE_ID, generated).apply()
        return generated
    }

    fun deviceName(): String {
        val manufacturer = Build.MANUFACTURER.orEmpty().trim()
        val model = Build.MODEL.orEmpty().trim()
        return listOf(manufacturer, model)
            .filter { it.isNotBlank() }
            .joinToString(" ")
            .ifBlank { "Android device" }
    }

    private fun buildDeviceId(): String {
        val androidId = Settings.Secure.getString(
            appContext.contentResolver,
            Settings.Secure.ANDROID_ID
        ).orEmpty().trim()

        val base = androidId.ifBlank {
            UUID.randomUUID().toString().replace("-", "")
        }

        return "android-" + base.lowercase(Locale.ROOT)
    }

    companion object {
        private const val PREFERENCE_FILE = "novpn_device_identity"
        private const val KEY_DEVICE_ID = "device_id"
    }
}
