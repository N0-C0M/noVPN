package com.novpn.vpn

import android.content.Context

data class VpnRuntimeStatusSnapshot(
    val running: Boolean,
    val status: String,
    val detail: String
)

class VpnRuntimeStatusStore(context: Context) {
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

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

    fun markStopped(status: String, detail: String = "") {
        save(running = false, status = status, detail = detail)
    }

    private fun save(running: Boolean, status: String, detail: String) {
        prefs.edit()
            .putBoolean(KEY_RUNNING, running)
            .putString(KEY_STATUS, status)
            .putString(KEY_DETAIL, detail)
            .apply()
    }

    companion object {
        private const val PREFS_NAME = "vpn_runtime_status"
        private const val KEY_RUNNING = "running"
        private const val KEY_STATUS = "status"
        private const val KEY_DETAIL = "detail"
    }
}
