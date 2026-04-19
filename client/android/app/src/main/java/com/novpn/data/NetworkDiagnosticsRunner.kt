package com.novpn.data

import com.novpn.vpn.RuntimeLocalProxyConfig
import java.io.BufferedInputStream
import java.io.BufferedOutputStream
import java.io.ByteArrayOutputStream
import java.net.Inet4Address
import java.net.Inet6Address
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.URI
import java.net.Socket
import java.net.SocketTimeoutException
import java.nio.charset.StandardCharsets
import java.util.Locale
import kotlin.math.roundToInt

data class NetworkDiagnosticsResult(
    val latencyMs: Int,
    val jitterMs: Int,
    val downloadMbps: Double,
    val uploadMbps: Double,
    val summary: String
)

class NetworkDiagnosticsRunner {
    fun verifyTunnel(
        profile: ClientProfile,
        proxy: RuntimeLocalProxyConfig,
        apiBaseFallback: String = "",
        startupProbe: Boolean = false
    ) {
        val target = resolveTarget(profile, apiBaseFallback)
        if (target.supportsHttpDiagnostics) {
            if (startupProbe) {
                verifyStartupControlPlane(target, proxy)
            } else {
                runStage("Control-plane probe") {
                    executeRequest(
                        proxy = proxy,
                        host = target.host,
                        port = target.port,
                        method = "HEAD",
                        path = target.diagPath("/ping?startup=1&ts=${System.currentTimeMillis()}"),
                        body = ByteArray(0)
                    )
                }
            }
            return
        }

        runStage("Proxy handshake") {
            openSocksSocket(proxy, target.host, target.port).use { }
        }
    }

    fun run(profile: ClientProfile, proxy: RuntimeLocalProxyConfig, apiBaseFallback: String = ""): NetworkDiagnosticsResult {
        val target = resolveTarget(profile, apiBaseFallback)

        val latencySamples = (1..3).map { index ->
            val startedAt = System.nanoTime()
            if (target.supportsHttpDiagnostics) {
                runStage("Latency probe #$index") {
                    executeRequest(
                        proxy = proxy,
                        host = target.host,
                        port = target.port,
                        method = "HEAD",
                        path = target.diagPath("/ping?seq=$index&ts=${System.currentTimeMillis()}"),
                        body = ByteArray(0)
                    )
                }
            } else {
                runStage("Latency probe #$index") {
                    openSocksSocket(proxy, target.host, target.port).use { }
                }
            }
            elapsedMillis(startedAt)
        }

        val latencyAverage = latencySamples.average().roundToInt()
        val jitter = (latencySamples.maxOrNull() ?: latencyAverage) - (latencySamples.minOrNull() ?: latencyAverage)

        val downloadMbps: Double
        val uploadMbps: Double
        if (target.supportsHttpDiagnostics) {
            val downloadBytes = DOWNLOAD_BYTES
            val downloadStartedAt = System.nanoTime()
            val downloaded = runStage("Download test") {
                executeRequest(
                    proxy = proxy,
                    host = target.host,
                    port = target.port,
                    method = "GET",
                    path = target.diagPath("/download?bytes=$downloadBytes"),
                    body = ByteArray(0)
                ).bodyBytes
            }
            val downloadDurationSeconds = elapsedSeconds(downloadStartedAt)
            downloadMbps = if (downloadDurationSeconds <= 0.0) {
                0.0
            } else {
                (downloaded * 8.0) / downloadDurationSeconds / 1_000_000.0
            }

            val uploadPayload = ByteArray(UPLOAD_BYTES) { index -> (index % 251).toByte() }
            val uploadStartedAt = System.nanoTime()
            runStage("Upload test") {
                executeRequest(
                    proxy = proxy,
                    host = target.host,
                    port = target.port,
                    method = "POST",
                    path = target.diagPath("/upload"),
                    body = uploadPayload
                )
            }
            val uploadDurationSeconds = elapsedSeconds(uploadStartedAt)
            uploadMbps = if (uploadDurationSeconds <= 0.0) {
                0.0
            } else {
                (uploadPayload.size * 8.0) / uploadDurationSeconds / 1_000_000.0
            }
        } else {
            downloadMbps = 0.0
            uploadMbps = 0.0
        }

        val summary = buildString {
            append("Latency ")
            append(latencyAverage)
            append(" ms")
            append(" | Jitter ")
            append(jitter)
            append(" ms")
            if (target.supportsHttpDiagnostics) {
                append("\nDownload ")
                append(formatMbps(downloadMbps))
                append(" | Upload ")
                append(formatMbps(uploadMbps))
            } else {
                append("\nControl plane diagnostics unavailable for this profile")
            }
        }

        return NetworkDiagnosticsResult(
            latencyMs = latencyAverage,
            jitterMs = jitter,
            downloadMbps = downloadMbps,
            uploadMbps = uploadMbps,
            summary = summary
        )
    }

    private fun resolveTarget(profile: ClientProfile, apiBaseFallback: String): DiagnosticsTarget {
        val apiBase = profile.server.apiBase
            .ifBlank { apiBaseFallback }
            .trim()
        if (apiBase.isNotBlank()) {
            val normalized = if (apiBase.startsWith("http://") || apiBase.startsWith("https://")) {
                apiBase
            } else {
                "http://$apiBase"
            }
            runCatching { URI(normalized) }.getOrNull()?.let { uri ->
                val host = uri.host?.trim().orEmpty()
                if (host.isNotBlank()) {
                    val scheme = uri.scheme?.lowercase(Locale.US).orEmpty()
                    val port = when {
                        uri.port > 0 -> uri.port
                        scheme == "https" -> 443
                        else -> 80
                    }
                    val path = uri.path.orEmpty().trimEnd('/')
                    return DiagnosticsTarget(
                        host = host,
                        port = port,
                        basePath = path,
                        supportsHttpDiagnostics = scheme != "https"
                    )
                }
            }
        }

        return DiagnosticsTarget(
            host = profile.server.address,
            port = profile.server.port,
            basePath = "",
            supportsHttpDiagnostics = false
        )
    }

    private fun executeRequest(
        proxy: RuntimeLocalProxyConfig,
        host: String,
        port: Int,
        method: String,
        path: String,
        body: ByteArray
    ): HttpProbeResponse {
        val socket = openSocksSocket(proxy, host, port)
        socket.soTimeout = READ_TIMEOUT_MS
        socket.tcpNoDelay = true

        socket.use { activeSocket ->
            val output = BufferedOutputStream(activeSocket.getOutputStream())
            val bodyLength = body.size
            val request = buildString {
                append(method)
                append(' ')
                append(path)
                append(" HTTP/1.1\r\n")
                append("Host: ")
                append(host)
                append("\r\n")
                append("Connection: close\r\n")
                append("Accept: application/json, application/octet-stream\r\n")
                if (bodyLength > 0) {
                    append("Content-Type: application/octet-stream\r\n")
                    append("Content-Length: ")
                    append(bodyLength)
                    append("\r\n")
                }
                append("\r\n")
            }.toByteArray(StandardCharsets.US_ASCII)

            output.write(request)
            if (bodyLength > 0) {
                output.write(body)
            }
            output.flush()

            val input = BufferedInputStream(activeSocket.getInputStream())
            val statusLine = readAsciiLine(input)
            if (statusLine.isBlank()) {
                throw IllegalStateException(
                    "Сервер диагностики не вернул строку HTTP-статуса. Похоже, туннель не довёл запрос до /admin/diag/ping."
                )
            }

            val statusCode = statusLine.split(' ')
                .drop(1)
                .firstOrNull()
                ?.toIntOrNull()
                ?: throw IllegalStateException("Сервер диагностики вернул некорректный HTTP-ответ: $statusLine")

            var contentLength = -1L
            while (true) {
                val headerLine = readAsciiLine(input)
                if (headerLine.isEmpty()) {
                    break
                }
                val separator = headerLine.indexOf(':')
                if (separator <= 0) {
                    continue
                }
                val headerName = headerLine.substring(0, separator).trim().lowercase(Locale.US)
                val headerValue = headerLine.substring(separator + 1).trim()
                if (headerName == "content-length") {
                    contentLength = headerValue.toLongOrNull() ?: -1L
                }
            }

            val bodyBytes = if (contentLength >= 0) {
                discardExactly(input, contentLength)
            } else {
                discardToEnd(input)
            }

            if (statusCode !in 200..299) {
                throw IllegalStateException("Сервер диагностики вернул HTTP $statusCode.")
            }

            return HttpProbeResponse(statusCode = statusCode, bodyBytes = bodyBytes)
        }
    }

    private fun openSocksSocket(proxy: RuntimeLocalProxyConfig, host: String, port: Int): Socket {
        val socket = Socket()
        socket.connect(InetSocketAddress(proxy.listenHost, proxy.socksPort), CONNECT_TIMEOUT_MS)
        socket.soTimeout = HANDSHAKE_TIMEOUT_MS

        val input = BufferedInputStream(socket.getInputStream())
        val output = BufferedOutputStream(socket.getOutputStream())

        val usesPasswordAuth = proxy.username.isNotBlank() && proxy.password.isNotBlank()
        if (usesPasswordAuth) {
            output.write(byteArrayOf(0x05, 0x02, 0x00, 0x02))
        } else {
            output.write(byteArrayOf(0x05, 0x01, 0x00))
        }
        output.flush()
        val selectedMethod = readExactly(input, 2)
        if (selectedMethod[0].toInt() != 0x05) {
            throw IllegalStateException("Local SOCKS bridge returned an invalid handshake response.")
        }
        when (selectedMethod[1].toInt() and 0xff) {
            0x00 -> Unit
            0x02 -> {
                val usernameBytes = proxy.username.toByteArray(StandardCharsets.UTF_8)
                val passwordBytes = proxy.password.toByteArray(StandardCharsets.UTF_8)
                val authRequest = ByteArrayOutputStream().apply {
                    write(0x01)
                    write(usernameBytes.size)
                    write(usernameBytes)
                    write(passwordBytes.size)
                    write(passwordBytes)
                }.toByteArray()
                output.write(authRequest)
                output.flush()
                expectBytes(input, byteArrayOf(0x01, 0x00))
            }
            else -> throw IllegalStateException(
                "Local SOCKS bridge rejected authentication with method ${(selectedMethod[1].toInt() and 0xff)}."
            )
        }

        val addressBytes = buildDestinationAddress(host)
        val connectRequest = ByteArrayOutputStream().apply {
            write(0x05)
            write(0x01)
            write(0x00)
            write(addressBytes)
            write((port shr 8) and 0xff)
            write(port and 0xff)
        }.toByteArray()
        output.write(connectRequest)
        output.flush()

        val header = readExactly(input, 4)
        if (header[0].toInt() != 0x05 || header[1].toInt() != 0x00) {
            throw IllegalStateException("Локальный SOCKS отклонил CONNECT с кодом ${header[1].toInt() and 0xff}.")
        }

        val atyp = header[3].toInt() and 0xff
        when (atyp) {
            0x01 -> readExactly(input, 4)
            0x03 -> {
                val length = readExactly(input, 1)[0].toInt() and 0xff
                readExactly(input, length)
            }
            0x04 -> readExactly(input, 16)
        }
        readExactly(input, 2)
        return socket
    }

    private fun <T> runStage(name: String, block: () -> T): T {
        return try {
            block()
        } catch (error: SocketTimeoutException) {
            throw IllegalStateException(
                "$name: время ожидания истекло. Туннель отвечает слишком медленно для текущего теста.",
                error
            )
        }
    }

    private fun verifyStartupControlPlane(target: DiagnosticsTarget, proxy: RuntimeLocalProxyConfig) {
        var lastProbeError: Exception? = null
        repeat(STARTUP_CONTROL_PLANE_ATTEMPTS) { attempt ->
            try {
                runStage("Control-plane probe #${attempt + 1}") {
                    executeRequest(
                        proxy = proxy,
                        host = target.host,
                        port = target.port,
                        method = "HEAD",
                        path = target.diagPath("/ping?startup=1&attempt=${attempt + 1}&ts=${System.currentTimeMillis()}"),
                        body = ByteArray(0)
                    )
                }
                return
            } catch (error: Exception) {
                if (error is InterruptedException) {
                    throw error
                }
                lastProbeError = error
                if (attempt < STARTUP_CONTROL_PLANE_ATTEMPTS - 1) {
                    Thread.sleep(STARTUP_CONTROL_PLANE_BACKOFF_MS)
                }
            }
        }

        try {
            runStage("Proxy handshake fallback") {
                openSocksSocket(proxy, target.host, target.port).use { }
            }
        } catch (fallbackError: Exception) {
            if (fallbackError is InterruptedException) {
                throw fallbackError
            }
            lastProbeError?.let(fallbackError::addSuppressed)
            throw fallbackError
        }
    }

    private fun buildDestinationAddress(host: String): ByteArray {
        val address = runCatching { InetAddress.getByName(host) }.getOrNull()
        return when (address) {
            is Inet4Address -> byteArrayOf(0x01) + address.address
            is Inet6Address -> byteArrayOf(0x04) + address.address
            else -> {
                val hostBytes = host.toByteArray(StandardCharsets.UTF_8)
                byteArrayOf(0x03, hostBytes.size.toByte()) + hostBytes
            }
        }
    }

    private fun readAsciiLine(input: BufferedInputStream): String {
        val output = ByteArrayOutputStream()
        while (true) {
            val next = input.read()
            if (next == -1) {
                break
            }
            if (next == '\n'.code) {
                break
            }
            if (next != '\r'.code) {
                output.write(next)
            }
        }
        return output.toString(StandardCharsets.US_ASCII.name())
    }

    private fun readExactly(input: BufferedInputStream, count: Int): ByteArray {
        val buffer = ByteArray(count)
        var offset = 0
        while (offset < count) {
            val read = input.read(buffer, offset, count - offset)
            if (read < 0) {
                throw IllegalStateException("Диагностический поток завершился раньше ожидаемого.")
            }
            offset += read
        }
        return buffer
    }

    private fun discardExactly(input: BufferedInputStream, count: Long): Long {
        val buffer = ByteArray(32 * 1024)
        var remaining = count
        var total = 0L
        while (remaining > 0) {
            val chunk = minOf(buffer.size.toLong(), remaining).toInt()
            val read = input.read(buffer, 0, chunk)
            if (read < 0) {
                break
            }
            total += read
            remaining -= read
        }
        return total
    }

    private fun discardToEnd(input: BufferedInputStream): Long {
        val buffer = ByteArray(32 * 1024)
        var total = 0L
        while (true) {
            val read = input.read(buffer)
            if (read < 0) {
                return total
            }
            total += read
        }
    }

    private fun expectBytes(input: BufferedInputStream, expected: ByteArray) {
        val actual = readExactly(input, expected.size)
        if (!actual.contentEquals(expected)) {
            throw IllegalStateException("Локальный SOCKS вернул неожиданный ответ на handshake.")
        }
    }

    private fun elapsedMillis(startedAt: Long): Int {
        return ((System.nanoTime() - startedAt) / 1_000_000L).toInt()
    }

    private fun elapsedSeconds(startedAt: Long): Double {
        return (System.nanoTime() - startedAt) / 1_000_000_000.0
    }

    private fun formatMbps(value: Double): String {
        return String.format(Locale.US, "%.2f Mbps", value)
    }

    private data class HttpProbeResponse(
        val statusCode: Int,
        val bodyBytes: Long
    )

    private data class DiagnosticsTarget(
        val host: String,
        val port: Int,
        val basePath: String,
        val supportsHttpDiagnostics: Boolean
    ) {
        fun diagPath(suffix: String): String {
            return buildString {
                append(basePath)
                append("/diag")
                append(suffix)
            }
        }
    }

    companion object {
        private const val DOWNLOAD_BYTES = 1024 * 1024L
        private const val UPLOAD_BYTES = 256 * 1024
        private const val CONNECT_TIMEOUT_MS = 10_000
        private const val HANDSHAKE_TIMEOUT_MS = 15_000
        private const val READ_TIMEOUT_MS = 60_000
        private const val STARTUP_CONTROL_PLANE_ATTEMPTS = 4
        private const val STARTUP_CONTROL_PLANE_BACKOFF_MS = 750L
    }
}
