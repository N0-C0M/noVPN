package com.novpn.vpn

import android.content.Context
import android.os.Build
import java.io.File

class EmbeddedRuntimeManager(private val context: Context) {
    private val runtimeRoot = File(context.filesDir, "runtime")
    private val binDir = File(runtimeRoot, "bin")
    private val logStore = RuntimeLogStore(context)
    private val logsDir = logStore.logsDirectory()
    private val xrayLogFile = File(logsDir, "xray.log")
    private val obfuscatorLogFile = File(logsDir, "obfuscator.log")
    private val appLogFile = logStore.appLogFile()
    private val prepareLock = Any()
    private var xrayProcess: Process? = null
    private var obfuscatorProcess: Process? = null

    @Volatile
    private var prepared = false

    fun prepare() {
        synchronized(prepareLock) {
            runtimeRoot.mkdirs()
            binDir.mkdirs()
            logsDir.mkdirs()
            appendAppLog("runtime", "Preparing embedded runtime in ${runtimeRoot.absolutePath}")

            resolveRuntimeExecutable("xray")
            installAssetFileIfNeeded("bin/geoip.dat", "geoip.dat")
            installAssetFileIfNeeded("bin/geosite.dat", "geosite.dat")
            resolveRuntimeExecutable("obfuscator")
            prepared = true
            appendAppLog("runtime", "Embedded runtime preparation complete")
        }
    }

    fun start(xrayConfig: File, obfuscatorConfig: File) {
        if (!prepared) {
            prepare()
        }

        val xrayBinary = resolveRuntimeExecutable("xray")
        val obfuscatorBinary = resolveRuntimeExecutable("obfuscator")
        appendAppLog(
            "runtime",
            "Starting sidecars: xray=${xrayBinary.absolutePath}, obfuscator=${obfuscatorBinary.absolutePath}, " +
                "xrayConfig=${xrayConfig.absolutePath}, obfuscatorConfig=${obfuscatorConfig.absolutePath}"
        )

        obfuscatorProcess = ProcessBuilder(
            obfuscatorBinary.absolutePath,
            "--config",
            obfuscatorConfig.absolutePath
        )
            .directory(binDir)
            .redirectErrorStream(true)
            .redirectOutput(obfuscatorLogFile)
            .start()
        appendAppLog("runtime", "Obfuscator process started with handle=${processHandle(obfuscatorProcess)}")
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
        appendAppLog("runtime", "Xray process started with handle=${processHandle(xrayProcess)}")
        verifyProcessStartup(xrayProcess, "xray", xrayLogFile)
    }

    fun stop() {
        appendAppLog("runtime", "Stopping embedded runtime processes")
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

    fun healthFailureDetail(): String? {
        buildProcessFailureDetail("xray", xrayProcess, xrayLogFile)?.let { return it }
        buildProcessFailureDetail("obfuscator", obfuscatorProcess, obfuscatorLogFile)?.let { return it }
        return null
    }

    fun logsDirectory(): File = logsDir

    fun appendAppLog(source: String, message: String) {
        logStore.append(source, message)
    }

    fun diagnosticsSummary(): String {
        val sections = buildList {
            processSummary("xray", xrayProcess)?.let(::add)
            processSummary("obfuscator", obfuscatorProcess)?.let(::add)
            logTailSummary("app", appLogFile)?.let(::add)
            logTailSummary("xray", xrayLogFile)?.let(::add)
            logTailSummary("obfuscator", obfuscatorLogFile)?.let(::add)
        }
        return sections.joinToString("\n")
    }

    private fun resolveRuntimeExecutable(binaryName: String): File {
        EmbeddedRuntimeAssets.resolveExecutablePathOrNull(context, binaryName)?.let { return it }

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            throw IllegalStateException(
                "Android blocked execution of $binaryName from the app files directory. " +
                    "No packaged runtime executable was found for this ABI."
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

    private fun installAssetFileIfNeeded(assetPath: String, targetName: String): File {
        val targetFile = File(binDir, targetName)
        if (!targetFile.exists() || targetFile.length() == 0L) {
            copyAssetToFile(assetPath, targetFile)
        }
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
            appendAppLog("runtime", "Process $label was not created")
            throw IllegalStateException("Process $label was not created.")
        }

        Thread.sleep(250)
        if (process.isAlive) {
            appendAppLog("runtime", "Process $label is alive after startup probe")
            return
        }

        val exitCode = runCatching { process.exitValue() }.getOrNull()
        val tail = readLogTail(logFile)
        appendAppLog("runtime", "Process $label exited immediately with code ${exitCode ?: -1}. Tail=$tail")
        val detail = if (tail.isBlank()) {
            "$label exited immediately with code ${exitCode ?: -1}."
        } else {
            "$label exited immediately with code ${exitCode ?: -1}. Log tail: $tail"
        }
        throw IllegalStateException(detail)
    }

    private fun processSummary(label: String, process: Process?): String? {
        process ?: return null
        return if (process.isAlive) {
            "Process $label is active."
        } else {
            "Process $label exited with code ${runCatching { process.exitValue() }.getOrElse { -1 }}."
        }
    }

    private fun logTailSummary(label: String, logFile: File): String? {
        val tail = readLogTail(logFile)
        if (tail.isBlank()) {
            return null
        }
        return "Log $label: $tail"
    }

    private fun buildProcessFailureDetail(label: String, process: Process?, logFile: File): String? {
        process ?: return null
        if (process.isAlive) {
            return null
        }
        val exitCode = runCatching { process.exitValue() }.getOrElse { -1 }
        val tail = readLogTail(logFile)
        return if (tail.isBlank()) {
            "$label exited with code $exitCode."
        } else {
            "$label exited with code $exitCode. Log tail: $tail"
        }
    }

    private fun readLogTail(logFile: File, lineCount: Int = 12): String {
        return logStore.readTail(logFile, lineCount)
    }

    private fun processHandle(process: Process?): String {
        return process?.javaClass?.simpleName + "@" + Integer.toHexString(System.identityHashCode(process))
    }
}
