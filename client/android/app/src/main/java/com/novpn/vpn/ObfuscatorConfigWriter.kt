package com.novpn.vpn

import android.content.Context
import com.novpn.data.ClientProfile
import org.json.JSONObject
import java.io.File

class ObfuscatorConfigWriter(private val context: Context) {

    fun write(profile: ClientProfile, xrayConfigPath: File): File {
        val outputDir = File(context.filesDir, "runtime").apply { mkdirs() }
        val outputFile = File(outputDir, "obfuscator.config.json")
        val document = JSONObject()
            .put("mode", "client")
            .put("seed", profile.obfuscation.seed)
            .put("traffic_strategy", profile.obfuscation.trafficStrategy.storageValue)
            .put("pattern_strategy", profile.obfuscation.patternStrategy.storageValue)
            .put(
                "remote",
                JSONObject()
                    .put("address", profile.server.address)
                    .put("port", profile.server.port)
            )
            .put(
                "integration",
                JSONObject()
                    .put("xrayConfigPath", xrayConfigPath.absolutePath)
                    .put("expectedCli", "--config <path>")
            )
            .put(
                "pattern_tuning",
                JSONObject()
                    .put("padding_profile", profile.obfuscation.patternStrategy.paddingProfile)
                    .put("jitter_window_ms", profile.obfuscation.patternStrategy.jitterWindowMs)
            )

        outputFile.writeText(document.toString(2))
        return outputFile
    }
}
