package com.novpn.data

import android.content.Context
import java.io.File
import java.nio.charset.StandardCharsets
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

class ClientLogStore(context: Context) {
    private val logsDir = File(context.filesDir, "logs")
    private val logFile = File(logsDir, "client.log")
    private val timestampFormatter = SimpleDateFormat("yyyy-MM-dd HH:mm:ss.SSS", Locale.US)

    @Synchronized
    fun append(tag: String, message: String) {
        logsDir.mkdirs()
        val normalizedTag = tag.trim().ifBlank { "client" }
        val normalizedMessage = message
            .trim()
            .replace("\r\n", "\n")
            .replace('\r', '\n')
        if (normalizedMessage.isBlank()) {
            return
        }

        logFile.appendText(
            buildString {
                append('[')
                append(timestampFormatter.format(Date()))
                append("] [")
                append(normalizedTag)
                append("] ")
                append(normalizedMessage)
                append('\n')
            },
            StandardCharsets.UTF_8
        )
        trimToRecentLines()
    }

    @Synchronized
    fun appendError(tag: String, prefix: String, error: Throwable) {
        val message = error.message?.trim().orEmpty()
        append(
            tag = tag,
            message = if (message.isBlank()) {
                "$prefix (${error.javaClass.simpleName})"
            } else {
                "$prefix (${error.javaClass.simpleName}): $message"
            }
        )
    }

    @Synchronized
    fun readTail(maxLines: Int = DEFAULT_TAIL_LINES): String {
        if (!logFile.exists()) {
            return ""
        }
        return runCatching {
            logFile.readLines(StandardCharsets.UTF_8)
                .takeLast(maxLines.coerceAtLeast(1))
                .joinToString("\n")
                .trim()
        }.getOrDefault("")
    }

    @Synchronized
    fun clear() {
        if (!logFile.exists()) {
            return
        }
        logFile.writeText("", StandardCharsets.UTF_8)
    }

    @Synchronized
    private fun trimToRecentLines(maxLines: Int = MAX_LINES) {
        if (!logFile.exists()) {
            return
        }
        runCatching {
            val lines = logFile.readLines(StandardCharsets.UTF_8)
            if (lines.size <= maxLines) {
                return
            }
            logFile.writeText(
                lines.takeLast(maxLines).joinToString("\n", postfix = "\n"),
                StandardCharsets.UTF_8
            )
        }
    }

    companion object {
        private const val MAX_LINES = 600
        private const val DEFAULT_TAIL_LINES = 250
    }
}
