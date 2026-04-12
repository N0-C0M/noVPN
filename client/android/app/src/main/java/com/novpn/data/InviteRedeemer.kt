package com.novpn.data

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
    val activationMode: String = ""
)

class InviteRedeemer {
    suspend fun redeem(
        serverAddress: String,
        inviteCode: String,
        deviceId: String,
        deviceName: String
    ): CodeRedeemResult = withContext(Dispatchers.IO) {
        val normalizedAddress = serverAddress.trim().trimEnd('/')
        val normalizedCode = inviteCode.trim()
        require(normalizedAddress.isNotBlank()) { "No server address available for invite activation." }
        require(normalizedCode.isNotBlank()) { "Enter an invite code first." }

        val endpoint = URL("http://$normalizedAddress/admin/redeem/$normalizedCode")
        val connection = (endpoint.openConnection() as HttpURLConnection).apply {
            requestMethod = "POST"
            connectTimeout = 10_000
            readTimeout = 15_000
            doOutput = true
            setRequestProperty("Content-Type", "application/json; charset=utf-8")
            setRequestProperty("Accept", "application/json")
        }

        val payload = """
            {
              "device_id": ${quoteJson(deviceId)},
              "device_name": ${quoteJson(deviceName)}
            }
        """.trimIndent()

        connection.outputStream.use { output ->
            output.write(payload.toByteArray(StandardCharsets.UTF_8))
        }

        val status = connection.responseCode
        val body = readAll(connection)
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

        when (root.optString("kind")) {
            "invite" -> {
                val payloads = extractProfilePayloads(root)

                if (payloads.isEmpty()) {
                    throw IllegalStateException("Server did not return a client profile.")
                }

                val profileName = root.optJSONObject("client_profile")
                    ?.optString("name")
                    .orEmpty()
                    .ifBlank {
                        root.optJSONArray("client_profiles")
                            ?.optJSONObject(0)
                            ?.optString("name")
                            .orEmpty()
                    }

                CodeRedeemResult(
                    kind = CodeRedeemKind.INVITE,
                    profilePayload = payloads.first(),
                    profilePayloads = payloads,
                    profileName = profileName
                )
            }
            "promo" -> {
                val payloads = extractProfilePayloads(root)
                val profileName = root.optJSONObject("client_profile")
                    ?.optString("name")
                    .orEmpty()
                    .ifBlank {
                        root.optJSONArray("client_profiles")
                            ?.optJSONObject(0)
                            ?.optString("name")
                            .orEmpty()
                    }
                CodeRedeemResult(
                    kind = CodeRedeemKind.PROMO,
                    profilePayload = payloads.firstOrNull().orEmpty(),
                    profilePayloads = payloads,
                    profileName = profileName,
                    bonusBytes = root.optLong("bonus_bytes", 0L),
                    activationMode = root.optString("activation_mode").trim().lowercase()
                )
            }
            else -> throw IllegalStateException(
                body.ifBlank { "Code activation returned an empty response." }
            )
        }
    }

    suspend fun disconnect(
        serverAddress: String,
        deviceId: String,
        deviceName: String,
        clientUuid: String
    ) = withContext(Dispatchers.IO) {
        val normalizedAddress = serverAddress.trim().trimEnd('/')
        require(normalizedAddress.isNotBlank()) { "No server address available for disconnect." }
        require(deviceId.isNotBlank()) { "Device ID is missing." }
        require(clientUuid.isNotBlank()) { "Client UUID is missing." }

        val endpoint = URL("http://$normalizedAddress/admin/disconnect")
        val connection = (endpoint.openConnection() as HttpURLConnection).apply {
            requestMethod = "POST"
            connectTimeout = 10_000
            readTimeout = 15_000
            doOutput = true
            setRequestProperty("Content-Type", "application/json; charset=utf-8")
            setRequestProperty("Accept", "application/json")
        }

        val payload = """
            {
              "device_id": ${quoteJson(deviceId)},
              "device_name": ${quoteJson(deviceName)},
              "client_uuid": ${quoteJson(clientUuid)}
            }
        """.trimIndent()

        connection.outputStream.use { output ->
            output.write(payload.toByteArray(StandardCharsets.UTF_8))
        }

        val status = connection.responseCode
        val body = readAll(connection)
        if (status !in 200..299) {
            throw IllegalStateException(
                body.ifBlank { "Device disconnect failed with HTTP $status." }
            )
        }
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
        return payloads
    }
}
