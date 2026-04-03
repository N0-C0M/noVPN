package com.novpn.vpn

import android.content.Context
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

        val xrayBinary = installAssetBinary("bin/xray", "xray")
        installAssetFile("bin/geoip.dat", "geoip.dat")
        installAssetFile("bin/geosite.dat", "geosite.dat")
        val obfuscatorBinary = installAssetBinary("bin/obfuscator", "obfuscator")

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

    private fun installAssetBinary(assetPath: String, targetName: String): File {
        val targetFile = File(binDir, targetName)
        if (targetFile.exists() && targetFile.canExecute()) {
            return targetFile
        }

        try {
            context.assets.open(assetPath).use { input ->
                targetFile.outputStream().use { output ->
                    input.copyTo(output)
                }
            }
        } catch (exception: IOException) {
            throw IllegalStateException(
                "Embedded binary asset missing: $assetPath. Place binaries under app/src/main/assets/bin/.",
                exception
            )
        }

        targetFile.setReadable(true)
        targetFile.setExecutable(true)
        return targetFile
    }

    private fun installAssetFile(assetPath: String, targetName: String): File {
        val targetFile = File(binDir, targetName)
        if (targetFile.exists()) {
            return targetFile
        }

        try {
            context.assets.open(assetPath).use { input ->
                targetFile.outputStream().use { output ->
                    input.copyTo(output)
                }
            }
        } catch (exception: IOException) {
            throw IllegalStateException(
                "Required Xray asset missing: $assetPath. Place geoip.dat and geosite.dat under app/src/main/assets/bin/.",
                exception
            )
        }

        targetFile.setReadable(true)
        return targetFile
    }
}
