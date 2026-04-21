package com.novpn.vpn

import android.content.Context

data class VpnRuntimeStatusSnapshot(
    val running: Boolean,
    val status: String,
    val detail: String
)

class VpnRuntimeStatusStore(context: Context) {
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    init {
        clearLegacyProxyState()
    }

    fun load(): VpnRuntimeStatusSnapshot {
        return VpnRuntimeStatusSnapshot(
            running = prefs.getBoolean(KEY_RUNNING, false),
            status = prefs.getString(KEY_STATUS, "").orEmpty(),
            detail = prefs.getString(KEY_DETAIL, "").orEmpty()
        )
    }

    fun markStarting(status: String, detail: String = "") {
        save(running = false, status = status, detail = detail)
    }

    fun markRunning(status: String, detail: String = "") {
        save(running = true, status = status, detail = detail)
    }

    fun markFailed(status: String, detail: String = "") {
        save(running = false, status = status, detail = detail)
    }

    fun markStopping(status: String, detail: String = "") {
        save(running = false, status = status, detail = detail)
    }

    fun markStopped(status: String, detail: String = "") {
        save(running = false, status = status, detail = detail)
    }

    private fun save(running: Boolean, status: String, detail: String) {
        prefs.edit()
            .putBoolean(KEY_RUNNING, running)
            .putString(KEY_STATUS, status)
            .putString(KEY_DETAIL, detail)
            .remove(KEY_PROXY_HOST)
            .remove(KEY_PROXY_PORT)
            .remove(KEY_PROXY_USERNAME)
            .remove(KEY_PROXY_PASSWORD)
            .apply()
    }

    private fun clearLegacyProxyState() {
        if (!prefs.contains(KEY_PROXY_HOST) &&
            !prefs.contains(KEY_PROXY_PORT) &&
            !prefs.contains(KEY_PROXY_USERNAME) &&
            !prefs.contains(KEY_PROXY_PASSWORD)
        ) {
            return
        }

        prefs.edit()
            .remove(KEY_PROXY_HOST)
            .remove(KEY_PROXY_PORT)
            .remove(KEY_PROXY_USERNAME)
            .remove(KEY_PROXY_PASSWORD)
            .apply()
    }

    companion object {
        private const val PREFS_NAME = "vpn_runtime_status"
        private const val KEY_RUNNING = "running"
        private const val KEY_STATUS = "status"
        private const val KEY_DETAIL = "detail"
        private const val KEY_PROXY_HOST = "proxy_host"
        private const val KEY_PROXY_PORT = "proxy_port"
        private const val KEY_PROXY_USERNAME = "proxy_username"
        private const val KEY_PROXY_PASSWORD = "proxy_password"
    }
}
