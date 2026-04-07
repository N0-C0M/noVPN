package com.novpn.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import java.nio.charset.StandardCharsets

class InviteRedeemer {
    suspend fun redeem(
        serverAddress: String,
        inviteCode: String,
        deviceId: String,
        deviceName: String
    ): String = withContext(Dispatchers.IO) {
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
            setRequestProperty("Accept", "application/x-yaml")
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

        body.ifBlank {
            throw IllegalStateException("Invite activation returned an empty profile payload.")
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
}
