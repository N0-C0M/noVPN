package com.novpn.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

data class GatewayPolicySnapshot(
    val blockedSitesCount: Int,
    val blockedAppsCount: Int,
    val mandatoryNotices: List<String>
)

class GatewayPolicyService {
    suspend fun fetch(serverAddress: String): GatewayPolicySnapshot = withContext(Dispatchers.IO) {
        val normalizedAddress = normalizeServerAddress(serverAddress)
        require(normalizedAddress.isNotBlank()) { "No server address available for policy sync." }

        val blocklist = readJson("http://$normalizedAddress/admin/client/policy")
        val notices = readJson("http://$normalizedAddress/admin/client/notices")

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
            mandatoryNotices = mandatoryNotices
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
            val payload = active.inputStream.bufferedReader().use { it.readText() }
            if (status !in 200..299) {
                throw IllegalStateException(payload.ifBlank { "Policy endpoint returned HTTP $status" })
            }
            JSONObject(payload)
        }
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
