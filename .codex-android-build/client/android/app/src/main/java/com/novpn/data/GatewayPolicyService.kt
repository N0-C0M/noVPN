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
    val profilePayloads: List<String> = emptyList(),
    val trafficUsedBytes: Long? = null,
    val trafficLimitBytes: Long? = null
)

class AccessRevokedException(message: String) : IllegalStateException(message)

class GatewayPolicyService {
    suspend fun fetch(
        serverAddress: String,
        apiBase: String = "",
        deviceId: String,
        clientUuid: String,
        deviceName: String = "",
        deviceOS: String = "",
        deviceOSVersion: String = ""
    ): GatewayPolicySnapshot = withContext(Dispatchers.IO) {
        val normalizedAddress = normalizeServerAddress(serverAddress)
        val normalizedApiBase = normalizeApiBase(serverAddress, apiBase)
        require(normalizedAddress.isNotBlank() || normalizedApiBase.isNotBlank()) { "No server address available for policy sync." }

        val blocklist = readJson("$normalizedApiBase/client/policy")
        val notices = readJson("$normalizedApiBase/client/notices")
        val quota = readQuotaSnapshot(
            serverAddress = normalizedAddress,
            apiBase = normalizedApiBase,
            deviceId = deviceId,
            clientUuid = clientUuid,
            deviceName = deviceName,
            deviceOS = deviceOS,
            deviceOSVersion = deviceOSVersion
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
            profilePayloads = extractProfilePayloads(quota),
            trafficUsedBytes = quota?.optLong("traffic_used_bytes", 0L)?.coerceAtLeast(0L),
            trafficLimitBytes = quota?.optLong("traffic_limit_bytes", 0L)?.coerceAtLeast(0L)
        )
    }

    private fun readJson(endpoint: String, headers: Map<String, String> = emptyMap()): JSONObject {
        val connection = (URL(endpoint).openConnection() as HttpURLConnection).apply {
            requestMethod = "GET"
            connectTimeout = 7000
            readTimeout = 7000
            setRequestProperty("Accept", "application/json")
            headers.forEach { (key, value) ->
                if (value.isNotBlank()) {
                    setRequestProperty(key, value)
                }
            }
        }

        return connection.useConnection { active ->
            val status = active.responseCode
            val payload = (active.errorStream ?: active.inputStream)?.bufferedReader()?.use { it.readText() }.orEmpty()
            if (status == HttpURLConnection.HTTP_FORBIDDEN) {
                throw AccessRevokedException(payload.ifBlank { "Device access was revoked by the server." })
            }
            if (status !in 200..299) {
                throw IllegalStateException(payload.ifBlank { "Policy endpoint returned HTTP $status" })
            }
            JSONObject(payload)
        }
    }

    private fun readQuotaSnapshot(
        serverAddress: String,
        apiBase: String,
        deviceId: String,
        clientUuid: String,
        deviceName: String,
        deviceOS: String,
        deviceOSVersion: String
    ): JSONObject? {
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
        val endpoint = "$apiBase/client/quota?${queryParts.joinToString("&")}"
        val headers = buildMap {
            if (deviceId.isNotBlank()) {
                put("X-Device-ID", deviceId)
            }
            if (deviceName.isNotBlank()) {
                put("X-Device-Name", deviceName)
            }
            if (deviceOS.isNotBlank()) {
                put("X-Device-OS", deviceOS)
            }
            if (deviceOSVersion.isNotBlank()) {
                put("X-Device-OS-Version", deviceOSVersion)
                put("X-Ver-OS", deviceOSVersion)
            }
        }
        return readJson(endpoint, headers)
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

    private fun normalizeApiBase(serverAddress: String, apiBase: String): String {
        val normalized = apiBase.trim().trimEnd('/')
        if (normalized.isNotBlank()) {
            return if (normalized.startsWith("http://") || normalized.startsWith("https://")) {
                normalized
            } else {
                "http://$normalized"
            }
        }
        val normalizedAddress = normalizeServerAddress(serverAddress)
        if (normalizedAddress.isBlank()) {
            return ""
        }
        return "http://$normalizedAddress/admin"
    }

    private fun extractProfilePayloads(quota: JSONObject?): List<String> {
        if (quota == null) {
            return emptyList()
        }
        val payloads = mutableListOf<String>()
        quota.optJSONArray("client_profiles_yaml")?.let { list ->
            for (index in 0 until list.length()) {
                list.optString(index)
                    .trim()
                    .takeIf { it.isNotBlank() }
                    ?.let(payloads::add)
            }
        }
        if (payloads.isNotEmpty()) {
            return payloads
        }
        quota.optJSONArray("client_profiles")?.let { list ->
            for (index in 0 until list.length()) {
                val profileObject = list.optJSONObject(index) ?: continue
                profileObject.toString().trim()
                    .takeIf { it.isNotBlank() }
                    ?.let(payloads::add)
            }
        }
        return payloads
    }
}

private inline fun <T> HttpURLConnection.useConnection(block: (HttpURLConnection) -> T): T {
    return try {
        block(this)
    } finally {
        disconnect()
    }
}
