package com.novpn.vpn

import android.content.Context
import com.novpn.data.ClientProfile
import com.novpn.data.DeviceIdentityStore
import com.novpn.obfs.SessionObfuscationPlan
import com.novpn.obfs.SessionObfuscationPlanner
import org.json.JSONArray
import org.json.JSONObject
import java.io.File

class ObfuscatorConfigWriter(private val context: Context) {

    fun write(
        profile: ClientProfile,
        xrayConfigPath: File,
        listenProxy: RuntimeLocalProxyConfig,
        upstreamProxy: RuntimeLocalProxyConfig,
        sessionPlan: SessionObfuscationPlan? = null
    ): File {
        val effectivePlan = sessionPlan ?: SessionObfuscationPlanner.build(
            profile = profile,
            deviceId = DeviceIdentityStore(context).deviceId()
        )
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
                "listen",
                JSONObject()
                    .put("address", listenProxy.listenHost)
                    .put("port", listenProxy.socksPort)
                    .put("username", listenProxy.username)
                    .put("password", listenProxy.password)
            )
            .put(
                "upstream",
                JSONObject()
                    .put("address", upstreamProxy.listenHost)
                    .put("port", upstreamProxy.socksPort)
                    .put("username", upstreamProxy.username)
                    .put("password", upstreamProxy.password)
            )
            .put(
                "integration",
                JSONObject()
                    .put("xrayConfigPath", xrayConfigPath.absolutePath)
                    .put("expectedCli", "--config <path>")
            )
            .put(
                "session",
                JSONObject()
                    .put("nonce", effectivePlan.sessionNonce)
                    .put("rotation_bucket", effectivePlan.rotationBucket)
                    .put("selected_fingerprint", effectivePlan.selectedFingerprint)
                    .put("selected_spider_x", effectivePlan.selectedSpiderX)
                    .put("fingerprint_pool", JSONArray(effectivePlan.fingerprintPool))
                    .put("cover_path_pool", JSONArray(effectivePlan.coverPathPool))
            )
            .put(
                "pattern_tuning",
                JSONObject()
                    .put("padding_profile", effectivePlan.paddingProfile)
                    .put("jitter_window_ms", effectivePlan.jitterWindowMs)
                    .put("padding_min_bytes", effectivePlan.paddingMinBytes)
                    .put("padding_max_bytes", effectivePlan.paddingMaxBytes)
                    .put("burst_interval_min_ms", effectivePlan.burstIntervalMinMs)
                    .put("burst_interval_max_ms", effectivePlan.burstIntervalMaxMs)
                    .put("idle_gap_min_ms", effectivePlan.idleGapMinMs)
                    .put("idle_gap_max_ms", effectivePlan.idleGapMaxMs)
            )

        outputFile.writeText(document.toString(2))
        return outputFile
    }
}
