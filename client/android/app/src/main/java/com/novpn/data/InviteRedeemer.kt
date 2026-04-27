package com.novpn.data

import android.net.Uri
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import java.nio.charset.StandardCharsets

enum class CodeRedeemKind {
    INVITE,
    PROMO
}

data class CodeRedeemResult(
    val kind: CodeRedeemKind,
    val profilePayload: String = "",
    val profilePayloads: List<String> = emptyList(),
    val profileName: String = "",
    val bonusBytes: Long = 0L,
    val activationMode: String = "",
    val trafficUsedBytes: Long? = null,
    val trafficLimitBytes: Long? = null
)

class InviteRedeemer {
    suspend fun redeem(
        serverAddress: String,
        apiBase: String = "",
        inviteCode: String,
        deviceId: String,
        deviceName: String
    ): CodeRedeemResult = withContext(Dispatchers.IO) {
        val normalizedAddress = serverAddress.trim().trimEnd('/')
        val parsedInput = IncomingAccessLinkParser.parse(inviteCode.trim())
        require(parsedInput == null || parsedInput.kind == IncomingAccessKind.INVITE_CODE) {
            "This link should be imported directly instead of activated as an invite code."
        }
        val normalizedCode = parsedInput?.value?.trim().orEmpty().ifBlank { inviteCode.trim() }
        val normalizedApiBase = normalizeApiBase(
            serverAddress,
            parsedInput?.apiBaseOverride?.takeIf { it.isNotBlank() } ?: apiBase
        )
        require(normalizedAddress.isNotBlank() || normalizedApiBase.isNotBlank()) { "No server address available for invite activation." }
        require(normalizedCode.isNotBlank()) { "Enter an invite code first." }

        val payload = """
            {
              "device_id": ${quoteJson(deviceId)},
              "device_name": ${quoteJson(deviceName)}
            }
        """.trimIndent()

        val response = executePostWithRedirects(
            endpoint = URL("$normalizedApiBase/redeem/${Uri.encode(normalizedCode)}"),
            payload = payload
        )
        val status = response.statusCode
        val body = response.body
        if (status !in 200..299) {
            throw IllegalStateException(
                body.ifBlank { "Invite activation failed with HTTP $status." }
            )
        }

        val root = runCatching { JSONObject(body) }.getOrElse {
            throw IllegalStateException(
                body.ifBlank { "Code activation returned an empty response." }
            )
        }

        val trafficUsedBytes = extractTrafficValue(root, "traffic_used_bytes")
        val trafficLimitBytes = extractTrafficValue(root, "traffic_limit_bytes")

        when (root.optString("kind")) {
            "invite" -> {
                val payloads = extractProfilePayloads(root)

                if (payloads.isEmpty()) {
                    throw IllegalStateException("Server did not return a client profile.")
                }

                val profileName = extractProfileName(root)

                CodeRedeemResult(
                    kind = CodeRedeemKind.INVITE,
                    profilePayload = payloads.first(),
                    profilePayloads = payloads,
                    profileName = profileName,
                    trafficUsedBytes = trafficUsedBytes,
                    trafficLimitBytes = trafficLimitBytes
                )
            }
            "promo" -> {
                val payloads = extractProfilePayloads(root)
                val profileName = extractProfileName(root)
                CodeRedeemResult(
                    kind = CodeRedeemKind.PROMO,
                    profilePayload = payloads.firstOrNull().orEmpty(),
                    profilePayloads = payloads,
                    profileName = profileName,
                    bonusBytes = root.optLong("bonus_bytes", 0L),
                    activationMode = root.optString("activation_mode").trim().lowercase(),
                    trafficUsedBytes = trafficUsedBytes,
                    trafficLimitBytes = trafficLimitBytes
                )
            }
            else -> throw IllegalStateException(
                body.ifBlank { "Code activation returned an empty response." }
            )
        }
    }

    suspend fun disconnect(
        serverAddress: String,
        apiBase: String = "",
        deviceId: String,
        deviceName: String,
        clientUuid: String
    ) = withContext(Dispatchers.IO) {
        val normalizedAddress = serverAddress.trim().trimEnd('/')
        val normalizedApiBase = normalizeApiBase(serverAddress, apiBase)
        require(normalizedAddress.isNotBlank() || normalizedApiBase.isNotBlank()) { "No server address available for disconnect." }
        require(deviceId.isNotBlank()) { "Device ID is missing." }
        require(clientUuid.isNotBlank()) { "Client UUID is missing." }

        val payload = """
            {
              "device_id": ${quoteJson(deviceId)},
              "device_name": ${quoteJson(deviceName)},
              "client_uuid": ${quoteJson(clientUuid)}
            }
        """.trimIndent()

        val response = executePostWithRedirects(
            endpoint = URL("$normalizedApiBase/disconnect"),
            payload = payload
        )
        val status = response.statusCode
        val body = response.body
        if (status !in 200..299) {
            throw IllegalStateException(
                body.ifBlank { "Device disconnect failed with HTTP $status." }
            )
        }
    }

    private fun executePostWithRedirects(endpoint: URL, payload: String): HttpResponse {
        var currentUrl = endpoint
        repeat(MAX_REDIRECTS + 1) { attempt ->
            val connection = (currentUrl.openConnection() as HttpURLConnection).apply {
                instanceFollowRedirects = false
                requestMethod = "POST"
                connectTimeout = 10_000
                readTimeout = 15_000
                doOutput = true
                setRequestProperty("Content-Type", "application/json; charset=utf-8")
                setRequestProperty("Accept", "application/json")
            }

            connection.outputStream.use { output ->
                output.write(payload.toByteArray(StandardCharsets.UTF_8))
            }

            val status = connection.responseCode
            val body = readAll(connection)
            if (status !in REDIRECT_STATUS_CODES) {
                return HttpResponse(statusCode = status, body = body)
            }

            val location = connection.getHeaderField("Location").orEmpty().trim()
            if (location.isBlank() || attempt >= MAX_REDIRECTS) {
                return HttpResponse(statusCode = status, body = body)
            }
            currentUrl = URL(currentUrl, location)
        }
        return HttpResponse(statusCode = 500, body = "")
    }

    private fun readAll(connection: HttpURLConnection): String {
        val stream = connection.errorStream ?: connection.inputStream
        return stream?.use { input ->
            BufferedReader(InputStreamReader(input, StandardCharsets.UTF_8)).readText().trim()
        }.orEmpty()
    }

    private fun quoteJson(value: String): String {
        val escaped = buildString(value.length + 8) {
            value.forEach { ch ->
                when (ch) {
                    '\\' -> append("\\\\")
                    '"' -> append("\\\"")
                    '\n' -> append("\\n")
                    '\r' -> append("\\r")
                    '\t' -> append("\\t")
                    else -> append(ch)
                }
            }
        }
        return "\"$escaped\""
    }

    private fun extractProfilePayloads(root: JSONObject): List<String> {
        val payloads = mutableListOf<String>()
        root.optJSONArray("client_profiles_yaml")?.let { list ->
            for (index in 0 until list.length()) {
                list.optString(index)
                    .trim()
                    .takeIf { it.isNotBlank() }
                    ?.let(payloads::add)
            }
        }
        if (payloads.isEmpty()) {
            root.optString("client_profile_yaml")
                .trim()
                .takeIf { it.isNotBlank() }
                ?.let(payloads::add)
        }
        if (payloads.isEmpty()) {
            root.optJSONArray("client_profiles")?.let { list ->
                for (index in 0 until list.length()) {
                    val profileObject = list.optJSONObject(index) ?: continue
                    buildCanonicalProfilePayload(profileObject)?.let(payloads::add)
                }
            }
        }
        return payloads
    }

    private fun buildCanonicalProfilePayload(source: JSONObject): String? {
        val name = pickString(source, "name", "Name").ifBlank { "Imported Reality Profile" }
        val address = pickString(source, "address", "Address")
        val port = pickInt(source, "port", "Port")
        val uuid = pickString(source, "uuid", "UUID")
        val flow = pickString(source, "flow", "Flow").ifBlank { "xtls-rprx-vision" }
        val serverName = pickString(source, "server_name", "ServerName")
        val fingerprint = pickString(source, "fingerprint", "Fingerprint").ifBlank { "chrome" }
        val publicKey = pickString(source, "public_key", "PublicKey")
        val shortId = pickString(source, "short_id", "ShortID")
            .ifBlank { pickFirstString(source, "short_ids", "ShortIDs") }
        val spiderX = pickString(source, "spider_x", "SpiderX").ifBlank { "/" }

        if (address.isBlank() || port <= 0 || uuid.isBlank() || serverName.isBlank() || publicKey.isBlank() || shortId.isBlank()) {
            return null
        }

        val payload = JSONObject()
            .put("name", name)
            .put(
                "server",
                JSONObject()
                    .put("address", address)
                    .put("port", port)
                    .put("uuid", uuid)
                    .put("flow", flow)
                    .put("server_name", serverName)
                    .put("fingerprint", fingerprint)
                    .put("public_key", publicKey)
                    .put("short_id", shortId)
                    .put("spider_x", spiderX)
            )
        return payload.toString()
    }

    private fun pickString(source: JSONObject, vararg keys: String): String {
        for (key in keys) {
            if (source.has(key)) {
                val value = source.optString(key).trim()
                if (value.isNotBlank()) {
                    return value
                }
            }
        }
        return ""
    }

    private fun pickInt(source: JSONObject, vararg keys: String): Int {
        for (key in keys) {
            if (source.has(key)) {
                val value = source.optInt(key, 0)
                if (value > 0) {
                    return value
                }
            }
        }
        return 0
    }

    private fun pickFirstString(source: JSONObject, vararg keys: String): String {
        for (key in keys) {
            val array = source.optJSONArray(key) ?: continue
            for (index in 0 until array.length()) {
                val value = array.optString(index).trim()
                if (value.isNotBlank()) {
                    return value
                }
            }
        }
        return ""
    }

    private fun extractTrafficValue(root: JSONObject, key: String): Long? {
        if (root.has(key)) {
            return root.optLong(key, 0L).coerceAtLeast(0L)
        }
        val client = root.optJSONObject("client") ?: return null
        if (!client.has(key)) {
            return null
        }
        return client.optLong(key, 0L).coerceAtLeast(0L)
    }

    private fun extractProfileName(root: JSONObject): String {
        root.optJSONObject("client_profile")?.let { profile ->
            pickString(profile, "name", "Name").takeIf { it.isNotBlank() }?.let { return it }
        }
        root.optJSONArray("client_profiles")?.let { profiles ->
            for (index in 0 until profiles.length()) {
                val profile = profiles.optJSONObject(index) ?: continue
                pickString(profile, "name", "Name").takeIf { it.isNotBlank() }?.let { return it }
            }
        }
        return ""
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
        val normalizedAddress = serverAddress.trim()
            .trim('/')
            .removePrefix("http://")
            .removePrefix("https://")
        if (normalizedAddress.isBlank()) {
            return ""
        }
        return "http://$normalizedAddress/admin"
    }

    private data class HttpResponse(
        val statusCode: Int,
        val body: String
    )

    companion object {
        private const val MAX_REDIRECTS = 4
        private val REDIRECT_STATUS_CODES = setOf(301, 302, 303, 307, 308)
    }
}
