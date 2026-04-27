package com.novpn.vpn

import android.content.Context
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

class RuntimeLogStore(context: Context) {
    private val runtimeRoot = File(context.filesDir, "runtime")
    private val logsDir = File(runtimeRoot, "logs")
    private val appLogFile = File(logsDir, APP_LOG_FILE_NAME)

    init {
        runtimeRoot.mkdirs()
        logsDir.mkdirs()
    }

    fun logsDirectory(): File = logsDir

    fun appLogFile(): File = appLogFile

    fun append(source: String, message: String) {
        val line = "${timestamp()} [$source] ${message.trim()}\n"
        synchronized(writeLock) {
            logsDir.mkdirs()
            appLogFile.appendText(line)
        }
    }

    fun readTail(file: File, lineCount: Int = 20): String {
        if (!file.exists()) {
            return ""
        }
        return runCatching {
            file.readLines()
                .takeLast(lineCount)
                .joinToString(" | ")
                .trim()
        }.getOrDefault("")
    }

    fun readFull(file: File, maxChars: Int = 60_000): String {
        if (!file.exists()) {
            return ""
        }
        return runCatching {
            val text = file.readText()
            if (text.length <= maxChars) text else text.takeLast(maxChars)
        }.getOrDefault("")
    }

    companion object {
        const val APP_LOG_FILE_NAME = "app.log"

        private val writeLock = Any()

        private fun timestamp(): String {
            return SimpleDateFormat("yyyy-MM-dd HH:mm:ss.SSS", Locale.US).format(Date())
        }
    }
}
