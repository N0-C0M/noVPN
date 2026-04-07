package com.novpn.vpn

import android.content.Context
import android.os.Build
import java.io.File
import java.io.IOException

class EmbeddedRuntimeManager(private val context: Context) {
    private val runtimeRoot = File(context.filesDir, "runtime")
    private val binDir = File(runtimeRoot, "bin")
    private val logsDir = File(runtimeRoot, "logs")
    private var xrayProcess: Process? = null
    private var obfuscatorProcess: Process? = null

    fun start(xrayConfig: File, obfuscatorConfig: File) {
        runtimeRoot.mkdirs()
        binDir.mkdirs()
        logsDir.mkdirs()

        val xrayBinary = installAbiAwareBinary("xray")
        installAssetFile("bin/geoip.dat", "geoip.dat")
        installAssetFile("bin/geosite.dat", "geosite.dat")
        val obfuscatorBinary = installAbiAwareBinary("obfuscator")

        obfuscatorProcess = ProcessBuilder(
            obfuscatorBinary.absolutePath,
            "--config",
            obfuscatorConfig.absolutePath
        )
            .directory(binDir)
            .redirectErrorStream(true)
            .redirectOutput(File(logsDir, "obfuscator.log"))
            .start()

        val xrayBuilder = ProcessBuilder(
            xrayBinary.absolutePath,
            "run",
            "-config",
            xrayConfig.absolutePath
        )
            .directory(binDir)
            .redirectErrorStream(true)
            .redirectOutput(File(logsDir, "xray.log"))
        xrayBuilder.environment()["XRAY_LOCATION_ASSET"] = binDir.absolutePath
        xrayBuilder.environment()["XRAY_LOCATION_CONFIG"] =
            xrayConfig.parentFile?.absolutePath ?: runtimeRoot.absolutePath
        xrayProcess = xrayBuilder.start()
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

    private fun installAbiAwareBinary(binaryName: String): File {
        val assetPath = resolveBinaryAssetPath(binaryName)
        val targetFile = File(binDir, binaryName)
        return installAssetBinary(assetPath, targetFile)
    }

    private fun resolveBinaryAssetPath(binaryName: String): String {
        val candidates = buildAbiAssetCandidates(binaryName)
        for (candidate in candidates) {
            if (assetExists(candidate)) {
                return candidate
            }
        }

        throw IllegalStateException(
            "Embedded binary asset missing for $binaryName. " +
                "Expected one of: ${candidates.joinToString()}."
        )
    }

    private fun buildAbiAssetCandidates(binaryName: String): List<String> {
        val supported = Build.SUPPORTED_ABIS
            .flatMap { abi -> abiAliases(abi) }
            .distinct()

        val candidates = supported.map { abi -> "bin/$abi/$binaryName" }.toMutableList()
        candidates += "bin/$binaryName"
        return candidates
    }

    private fun abiAliases(abi: String): List<String> {
        val aliases = mutableListOf(abi)
        when (abi) {
            "arm64-v8a" -> aliases += listOf("arm64", "aarch64")
            "armeabi-v7a" -> aliases += listOf("armeabi", "armv7")
            "x86_64" -> aliases += listOf("amd64")
            "x86" -> aliases += listOf("i686", "i386")
        }
        return aliases
    }

    private fun assetExists(assetPath: String): Boolean {
        return try {
            context.assets.open(assetPath).close()
            true
        } catch (_: IOException) {
            false
        }
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
        try {
            context.assets.open(assetPath).use { input ->
                targetFile.outputStream().use { output ->
                    input.copyTo(output)
                }
            }
        } catch (exception: IOException) {
            throw IllegalStateException(
                "Required embedded asset missing: $assetPath.",
                exception
            )
        }
    }
}
