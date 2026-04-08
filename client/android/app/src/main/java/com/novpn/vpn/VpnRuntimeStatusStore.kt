package com.novpn.vpn

import android.content.Context

data class VpnRuntimeStatusSnapshot(
    val running: Boolean,
    val status: String,
    val detail: String,
    val localProxy: RuntimeLocalProxyConfig?
)

class VpnRuntimeStatusStore(context: Context) {
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    fun load(): VpnRuntimeStatusSnapshot {
        return VpnRuntimeStatusSnapshot(
            running = prefs.getBoolean(KEY_RUNNING, false),
            status = prefs.getString(KEY_STATUS, "").orEmpty(),
            detail = prefs.getString(KEY_DETAIL, "").orEmpty(),
            localProxy = loadLocalProxy()
        )
    }

    fun markStarting(status: String, detail: String = "") {
        save(running = false, status = status, detail = detail, localProxy = null)
    }

    fun markRunning(status: String, detail: String = "", localProxy: RuntimeLocalProxyConfig? = null) {
        save(running = true, status = status, detail = detail, localProxy = localProxy)
    }

    fun markFailed(status: String, detail: String = "") {
        save(running = false, status = status, detail = detail, localProxy = null)
    }

    fun markStopped(status: String, detail: String = "") {
        save(running = false, status = status, detail = detail, localProxy = null)
    }

    private fun save(
        running: Boolean,
        status: String,
        detail: String,
        localProxy: RuntimeLocalProxyConfig?
    ) {
        prefs.edit()
            .putBoolean(KEY_RUNNING, running)
            .putString(KEY_STATUS, status)
            .putString(KEY_DETAIL, detail)
            .putString(KEY_PROXY_HOST, localProxy?.listenHost)
            .putInt(KEY_PROXY_PORT, localProxy?.socksPort ?: 0)
            .putString(KEY_PROXY_USERNAME, localProxy?.username)
            .putString(KEY_PROXY_PASSWORD, localProxy?.password)
            .apply()
    }

    private fun loadLocalProxy(): RuntimeLocalProxyConfig? {
        val host = prefs.getString(KEY_PROXY_HOST, "").orEmpty()
        val port = prefs.getInt(KEY_PROXY_PORT, 0)
        val username = prefs.getString(KEY_PROXY_USERNAME, "").orEmpty()
        val password = prefs.getString(KEY_PROXY_PASSWORD, "").orEmpty()
        if (host.isBlank() || port <= 0 || username.isBlank() || password.isBlank()) {
            return null
        }

        return RuntimeLocalProxyConfig(
            listenHost = host,
            socksPort = port,
            username = username,
            password = password,
            udpEnabled = false
        )
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
