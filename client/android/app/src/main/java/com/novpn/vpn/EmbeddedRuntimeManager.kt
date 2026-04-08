package com.novpn.vpn

import android.content.Context
import android.os.Build
import java.io.File

class EmbeddedRuntimeManager(private val context: Context) {
    private val runtimeRoot = File(context.filesDir, "runtime")
    private val binDir = File(runtimeRoot, "bin")
    private val logsDir = File(runtimeRoot, "logs")
    private val xrayLogFile = File(logsDir, "xray.log")
    private val obfuscatorLogFile = File(logsDir, "obfuscator.log")
    private var xrayProcess: Process? = null
    private var obfuscatorProcess: Process? = null

    fun start(xrayConfig: File, obfuscatorConfig: File) {
        runtimeRoot.mkdirs()
        binDir.mkdirs()
        logsDir.mkdirs()

        val xrayBinary = resolveRuntimeExecutable("xray")
        installAssetFile("bin/geoip.dat", "geoip.dat")
        installAssetFile("bin/geosite.dat", "geosite.dat")
        val obfuscatorBinary = resolveRuntimeExecutable("obfuscator")

        obfuscatorProcess = ProcessBuilder(
            obfuscatorBinary.absolutePath,
            "--config",
            obfuscatorConfig.absolutePath
        )
            .directory(binDir)
            .redirectErrorStream(true)
            .redirectOutput(obfuscatorLogFile)
            .start()
        verifyProcessStartup(obfuscatorProcess, "obfuscator", obfuscatorLogFile)

        val xrayBuilder = ProcessBuilder(
            xrayBinary.absolutePath,
            "run",
            "-config",
            xrayConfig.absolutePath
        )
            .directory(binDir)
            .redirectErrorStream(true)
            .redirectOutput(xrayLogFile)
        xrayBuilder.environment()["XRAY_LOCATION_ASSET"] = binDir.absolutePath
        xrayBuilder.environment()["XRAY_LOCATION_CONFIG"] =
            xrayConfig.parentFile?.absolutePath ?: runtimeRoot.absolutePath
        xrayProcess = xrayBuilder.start()
        verifyProcessStartup(xrayProcess, "xray", xrayLogFile)
    }

    fun stop() {
        xrayProcess?.destroy()
        obfuscatorProcess?.destroy()
        xrayProcess = null
        obfuscatorProcess = null
    }

    fun isRunning(): Boolean {
        return listOf(xrayProcess, obfuscatorProcess).any { process ->
            process != null && process.isAlive
        }
    }

    fun logsDirectory(): File = logsDir

    fun diagnosticsSummary(): String {
        val sections = buildList {
            processSummary("xray", xrayProcess)?.let(::add)
            processSummary("obfuscator", obfuscatorProcess)?.let(::add)
            logTailSummary("xray", xrayLogFile)?.let(::add)
            logTailSummary("obfuscator", obfuscatorLogFile)?.let(::add)
        }
        return sections.joinToString("\n")
    }

    private fun resolveRuntimeExecutable(binaryName: String): File {
        EmbeddedRuntimeAssets.resolveExecutablePathOrNull(context, binaryName)?.let { return it }

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            throw IllegalStateException(
                "Android заблокировал запуск $binaryName из файлов приложения. " +
                    "Для этой ABI не найден упакованный runtime-исполняемый файл."
            )
        }

        return installAbiAwareBinary(binaryName)
    }

    private fun installAbiAwareBinary(binaryName: String): File {
        val assetPath = EmbeddedRuntimeAssets.resolveBinaryAssetPath(context, binaryName)
        val targetFile = File(binDir, binaryName)
        return installAssetBinary(assetPath, targetFile)
    }

    private fun installAssetBinary(assetPath: String, targetFile: File): File {
        copyAssetToFile(assetPath, targetFile)
        targetFile.setReadable(true)
        targetFile.setExecutable(true)
        return targetFile
    }

    private fun installAssetFile(assetPath: String, targetName: String): File {
        val targetFile = File(binDir, targetName)
        copyAssetToFile(assetPath, targetFile)
        targetFile.setReadable(true)
        return targetFile
    }

    private fun copyAssetToFile(assetPath: String, targetFile: File) {
        context.assets.open(assetPath).use { input ->
            targetFile.outputStream().use { output ->
                input.copyTo(output)
            }
        }
    }

    private fun verifyProcessStartup(process: Process?, label: String, logFile: File) {
        if (process == null) {
            throw IllegalStateException("Процесс $label не был создан.")
        }

        Thread.sleep(250)
        if (process.isAlive) {
            return
        }

        val exitCode = runCatching { process.exitValue() }.getOrNull()
        val tail = readLogTail(logFile)
        val detail = if (tail.isBlank()) {
            "$label завершился сразу с кодом ${exitCode ?: -1}."
        } else {
            "$label завершился сразу с кодом ${exitCode ?: -1}. Хвост лога: $tail"
        }
        throw IllegalStateException(detail)
    }

    private fun processSummary(label: String, process: Process?): String? {
        process ?: return null
        return if (process.isAlive) {
            "Процесс $label активен."
        } else {
            "Процесс $label завершился с кодом ${runCatching { process.exitValue() }.getOrElse { -1 }}."
        }
    }

    private fun logTailSummary(label: String, logFile: File): String? {
        val tail = readLogTail(logFile)
        if (tail.isBlank()) {
            return null
        }
        return "Лог $label: $tail"
    }

    private fun readLogTail(logFile: File, lineCount: Int = 12): String {
        if (!logFile.exists()) {
            return ""
        }
        return runCatching {
            logFile.readLines()
                .takeLast(lineCount)
                .joinToString(" | ")
                .trim()
        }.getOrDefault("")
    }
}
