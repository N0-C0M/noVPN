package com.novpn.vpn

import android.content.Context
import java.io.File

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
}
