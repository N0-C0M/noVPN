package com.novpn.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URLEncoder
import java.net.URL
import java.nio.charset.StandardCharsets

data class GatewayPolicySnapshot(
    val blockedSitesCount: Int,
    val blockedAppsCount: Int,
    val mandatoryNotices: List<String>,
    val trafficUsedBytes: Long? = null,
    val trafficLimitBytes: Long? = null
)

class GatewayPolicyService {
    suspend fun fetch(
        serverAddress: String,
        deviceId: String,
        clientUuid: String
    ): GatewayPolicySnapshot = withContext(Dispatchers.IO) {
        val normalizedAddress = normalizeServerAddress(serverAddress)
        require(normalizedAddress.isNotBlank()) { "No server address available for policy sync." }

        val blocklist = readJson("http://$normalizedAddress/admin/client/policy")
        val notices = readJson("http://$normalizedAddress/admin/client/notices")
        val quota = readQuotaSnapshot(
            serverAddress = normalizedAddress,
            deviceId = deviceId,
            clientUuid = clientUuid
        )

        val blockedSites = blocklist.optJSONArray("blocked_sites")?.length() ?: 0
        val blockedApps = blocklist.optJSONArray("blocked_apps")?.length() ?: 0
        val mandatoryNotices = mutableListOf<String>()

        notices.optJSONArray("notices")?.let { array ->
            for (index in 0 until array.length()) {
                val item = array.optJSONObject(index) ?: continue
                val title = item.optString("title").trim()
                val message = item.optString("message").trim()
                if (title.isBlank() && message.isBlank()) {
                    continue
                }
                mandatoryNotices += if (title.isBlank()) {
                    message
                } else if (message.isBlank()) {
                    title
                } else {
                    "$title: $message"
                }
            }
        }

        GatewayPolicySnapshot(
            blockedSitesCount = blockedSites,
            blockedAppsCount = blockedApps,
            mandatoryNotices = mandatoryNotices,
            trafficUsedBytes = quota?.optLong("traffic_used_bytes", 0L)?.coerceAtLeast(0L),
            trafficLimitBytes = quota?.optLong("traffic_limit_bytes", 0L)?.coerceAtLeast(0L)
        )
    }

    private fun readJson(endpoint: String): JSONObject {
        val connection = (URL(endpoint).openConnection() as HttpURLConnection).apply {
            requestMethod = "GET"
            connectTimeout = 7000
            readTimeout = 7000
            setRequestProperty("Accept", "application/json")
        }

        return connection.useConnection { active ->
            val status = active.responseCode
            val payload = (active.errorStream ?: active.inputStream)?.bufferedReader()?.use { it.readText() }.orEmpty()
            if (status !in 200..299) {
                throw IllegalStateException(payload.ifBlank { "Policy endpoint returned HTTP $status" })
            }
            JSONObject(payload)
        }
    }

    private fun readQuotaSnapshot(serverAddress: String, deviceId: String, clientUuid: String): JSONObject? {
        val queryParts = buildList {
            if (deviceId.isNotBlank()) {
                add("device_id=${urlEncode(deviceId)}")
            }
            if (clientUuid.isNotBlank()) {
                add("client_uuid=${urlEncode(clientUuid)}")
            }
        }
        if (queryParts.isEmpty()) {
            return null
        }
        val endpoint = "http://$serverAddress/admin/client/quota?${queryParts.joinToString("&")}"
        return runCatching { readJson(endpoint) }.getOrNull()
    }

    private fun urlEncode(value: String): String {
        return URLEncoder.encode(value, StandardCharsets.UTF_8.name())
    }

    private fun normalizeServerAddress(serverAddress: String): String {
        return serverAddress.trim()
            .trim('/')
            .removePrefix("http://")
            .removePrefix("https://")
    }
}

private inline fun <T> HttpURLConnection.useConnection(block: (HttpURLConnection) -> T): T {
    return try {
        block(this)
    } finally {
        disconnect()
    }
}
