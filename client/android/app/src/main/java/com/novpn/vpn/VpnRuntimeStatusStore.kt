package com.novpn.vpn

import android.content.Context
import com.novpn.R
import org.json.JSONObject
import java.io.File

data class VpnRuntimeStatusSnapshot(
    val running: Boolean,
    val status: String,
    val detail: String
)

class VpnRuntimeStatusStore(context: Context) {
    private val appContext = context.applicationContext
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
    private val statusFile = File(appContext.filesDir, STATUS_FILE_NAME)

    init {
        clearLegacyProxyState()
    }

    fun load(): VpnRuntimeStatusSnapshot {
        val storedStatus = readFromFile() ?: readFromPrefs()
        if (shouldResetTransientState(storedStatus)) {
            val stoppedStatus = VpnRuntimeStatusSnapshot(
                running = false,
                status = appContext.getString(R.string.service_stopped),
                detail = ""
            )
            save(
                running = stoppedStatus.running,
                status = stoppedStatus.status,
                detail = stoppedStatus.detail
            )
            return stoppedStatus
        }
        return storedStatus.snapshot
    }

    private fun readFromPrefs(): StoredStatus {
        return StoredStatus(
            snapshot = VpnRuntimeStatusSnapshot(
                running = prefs.getBoolean(KEY_RUNNING, false),
                status = prefs.getString(KEY_STATUS, "").orEmpty(),
                detail = prefs.getString(KEY_DETAIL, "").orEmpty()
            ),
            updatedAtMs = 0L
        )
    }

    private fun shouldResetTransientState(storedStatus: StoredStatus): Boolean {
        val snapshot = storedStatus.snapshot
        if (snapshot.running || !isTransientStatus(snapshot.status)) {
            return false
        }

        val updatedAtMs = storedStatus.updatedAtMs
        if (updatedAtMs <= 0L) {
            return true
        }

        val ageMs = System.currentTimeMillis() - updatedAtMs
        return ageMs > TRANSIENT_STATUS_STALE_MS
    }

    private fun isTransientStatus(status: String): Boolean {
        if (status.isBlank()) {
            return false
        }
        return status == appContext.getString(R.string.runtime_starting) ||
            status == STATUS_STOPPING
    }

    fun markStarting(status: String, detail: String = "") {
        save(running = false, status = status, detail = detail)
    }

    fun markStopping(status: String = STATUS_STOPPING, detail: String = DETAIL_STOPPING) {
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
        writeToFile(running, status, detail)
        prefs.edit()
            .putBoolean(KEY_RUNNING, running)
            .putString(KEY_STATUS, status)
            .putString(KEY_DETAIL, detail)
            .remove(KEY_PROXY_HOST)
            .remove(KEY_PROXY_PORT)
            .remove(KEY_PROXY_USERNAME)
            .remove(KEY_PROXY_PASSWORD)
            .commit()
    }

    private fun readFromFile(): StoredStatus? {
        return runCatching {
            if (!statusFile.exists()) {
                return@runCatching null
            }
            val payload = statusFile.readText(Charsets.UTF_8).trim()
            if (payload.isBlank()) {
                return@runCatching null
            }
            val json = JSONObject(payload)
            StoredStatus(
                snapshot = VpnRuntimeStatusSnapshot(
                    running = json.optBoolean(JSON_KEY_RUNNING, false),
                    status = json.optString(JSON_KEY_STATUS, ""),
                    detail = json.optString(JSON_KEY_DETAIL, "")
                ),
                updatedAtMs = json.optLong(JSON_KEY_UPDATED_AT_MS, 0L)
            )
        }.getOrNull()
    }

    private fun writeToFile(running: Boolean, status: String, detail: String) {
        runCatching {
            statusFile.parentFile?.mkdirs()
            val payload = JSONObject()
                .put(JSON_KEY_RUNNING, running)
                .put(JSON_KEY_STATUS, status)
                .put(JSON_KEY_DETAIL, detail)
                .put(JSON_KEY_UPDATED_AT_MS, System.currentTimeMillis())
                .toString()

            val tempFile = File(statusFile.parentFile, "$STATUS_FILE_NAME.tmp")
            tempFile.writeText(payload, Charsets.UTF_8)
            if (!tempFile.renameTo(statusFile)) {
                statusFile.writeText(payload, Charsets.UTF_8)
                tempFile.delete()
            }
        }
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

    private data class StoredStatus(
        val snapshot: VpnRuntimeStatusSnapshot,
        val updatedAtMs: Long
    )

    companion object {
        private const val PREFS_NAME = "vpn_runtime_status"
        private const val STATUS_FILE_NAME = "vpn_runtime_status.json"
        private const val KEY_RUNNING = "running"
        private const val KEY_STATUS = "status"
        private const val KEY_DETAIL = "detail"
        private const val KEY_PROXY_HOST = "proxy_host"
        private const val KEY_PROXY_PORT = "proxy_port"
        private const val KEY_PROXY_USERNAME = "proxy_username"
        private const val KEY_PROXY_PASSWORD = "proxy_password"
        private const val JSON_KEY_RUNNING = "running"
        private const val JSON_KEY_STATUS = "status"
        private const val JSON_KEY_DETAIL = "detail"
        private const val JSON_KEY_UPDATED_AT_MS = "updated_at_ms"
        private const val TRANSIENT_STATUS_STALE_MS = 30_000L

        const val STATUS_STOPPING = "\u041e\u0442\u043a\u043b\u044e\u0447\u0430\u0435\u043c VPN"
        const val DETAIL_STOPPING =
            "\u0417\u0430\u0432\u0435\u0440\u0448\u0430\u0435\u043c \u0442\u0435\u043a\u0443\u0449\u0438\u0439 " +
                "\u0442\u0443\u043d\u043d\u0435\u043b\u044c \u043f\u0435\u0440\u0435\u0434 \u043f\u043e\u0432\u0442\u043e\u0440\u043d\u044b\u043c " +
                "\u0437\u0430\u043f\u0443\u0441\u043a\u043e\u043c."
    }
}
