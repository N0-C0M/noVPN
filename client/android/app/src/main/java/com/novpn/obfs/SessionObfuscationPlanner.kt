package com.novpn.obfs

import com.novpn.data.ClientProfile
import com.novpn.data.PatternMaskingStrategy
import com.novpn.data.TrafficObfuscationStrategy
import java.security.MessageDigest
import java.util.Locale

data class SessionObfuscationPlan(
    val rotationBucket: Long,
    val sessionNonce: String,
    val selectedFingerprint: String,
    val selectedSpiderX: String,
    val fingerprintPool: List<String>,
    val coverPathPool: List<String>,
    val paddingProfile: String,
    val jitterWindowMs: Int,
    val paddingMinBytes: Int,
    val paddingMaxBytes: Int,
    val burstIntervalMinMs: Int,
    val burstIntervalMaxMs: Int,
    val idleGapMinMs: Int,
    val idleGapMaxMs: Int
)

object SessionObfuscationPlanner {
    private const val ROTATION_INTERVAL_MS = 20 * 60 * 1000L

    fun build(
        profile: ClientProfile,
        deviceId: String,
        nowMs: Long = System.currentTimeMillis()
    ): SessionObfuscationPlan {
        val rotationBucket = nowMs / ROTATION_INTERVAL_MS
        val digest = sha256(
            buildString {
                append(profile.obfuscation.seed)
                append('|')
                append(deviceId.trim())
                append('|')
                append(profile.server.shortId)
                append('|')
                append(profile.server.address)
                append('|')
                append(profile.server.serverName)
                append('|')
                append(rotationBucket)
                append('|')
                append(profile.obfuscation.trafficStrategy.storageValue)
                append('|')
                append(profile.obfuscation.patternStrategy.storageValue)
            }
        )
        val digestHex = digest.joinToString("") { byte -> "%02x".format(byte.toInt() and 0xff) }

        val fingerprintPool = fingerprintPool(profile)
        val selectedFingerprint = fingerprintPool[(digest[0].toInt() and 0xff) % fingerprintPool.size]

        val coverPathPool = coverPathPool(profile, digestHex, selectedFingerprint)
        val selectedSpiderX = coverPathPool[(digest[1].toInt() and 0xff) % coverPathPool.size]

        val (paddingMinBytes, paddingMaxBytes) = paddingRange(profile.obfuscation.patternStrategy, digest)
        val (burstIntervalMinMs, burstIntervalMaxMs) = burstRange(profile.obfuscation.patternStrategy, digest)
        val (idleGapMinMs, idleGapMaxMs) = idleRange(profile.obfuscation.patternStrategy, digest)

        return SessionObfuscationPlan(
            rotationBucket = rotationBucket,
            sessionNonce = digestHex.take(16),
            selectedFingerprint = selectedFingerprint,
            selectedSpiderX = selectedSpiderX,
            fingerprintPool = fingerprintPool,
            coverPathPool = coverPathPool,
            paddingProfile = paddingProfile(profile.obfuscation.patternStrategy, digest),
            jitterWindowMs = profile.obfuscation.patternStrategy.jitterWindowMs + ((digest[2].toInt() and 0xff) % 120),
            paddingMinBytes = paddingMinBytes,
            paddingMaxBytes = paddingMaxBytes,
            burstIntervalMinMs = burstIntervalMinMs,
            burstIntervalMaxMs = burstIntervalMaxMs,
            idleGapMinMs = idleGapMinMs,
            idleGapMaxMs = idleGapMaxMs
        )
    }

    private fun fingerprintPool(profile: ClientProfile): List<String> {
        val base = profile.server.fingerprint.ifBlank { "chrome" }
        val variants = when (profile.obfuscation.trafficStrategy) {
            TrafficObfuscationStrategy.BALANCED -> listOf(base, "chrome", "firefox", "edge")
            TrafficObfuscationStrategy.CDN_MIMIC -> listOf("chrome", "edge", base)
            TrafficObfuscationStrategy.FRAGMENTED -> listOf("safari", "chrome", "firefox")
            TrafficObfuscationStrategy.MOBILE_MIX -> listOf("firefox", "safari", "chrome")
            TrafficObfuscationStrategy.TLS_BLEND -> listOf("edge", "chrome", "safari")
        }
        return variants.map { it.trim() }.filter { it.isNotBlank() }.distinct()
    }

    private fun coverPathPool(
        profile: ClientProfile,
        digestHex: String,
        selectedFingerprint: String
    ): List<String> {
        val shortSuffix = profile.server.shortId.takeLast(4).ifBlank { "edge" }
        val seedSuffix = digestHex.take(6)
        val fingerprintHint = selectedFingerprint.take(3).lowercase(Locale.ROOT)
        val pattern = profile.obfuscation.patternStrategy
        val traffic = profile.obfuscation.trafficStrategy
        val variants = mutableListOf(
            profile.server.spiderX.ifBlank { "/" },
            pattern.spiderXPath,
            traffic.spiderXPath,
            "${pattern.spiderXPath}/$shortSuffix",
            "${pattern.spiderXPath}?v=$seedSuffix"
        )
        when (pattern) {
            PatternMaskingStrategy.QUIET_SWEEP ->
                variants += "${pattern.spiderXPath}?fp=$fingerprintHint&r=$seedSuffix"
            PatternMaskingStrategy.RANDOMIZED ->
                variants += "${pattern.spiderXPath}/$seedSuffix"
            PatternMaskingStrategy.BURST_FADE ->
                variants += "${pattern.spiderXPath}?burst=$seedSuffix"
            PatternMaskingStrategy.PULSE ->
                variants += "${pattern.spiderXPath}?pulse=$seedSuffix"
            PatternMaskingStrategy.STEADY -> Unit
        }
        return variants
            .map { candidate ->
                val trimmed = candidate.trim().ifBlank { "/" }
                if (trimmed.startsWith("/")) trimmed else "/$trimmed"
            }
            .distinct()
    }

    private fun paddingRange(strategy: PatternMaskingStrategy, digest: ByteArray): Pair<Int, Int> {
        val base = when (strategy) {
            PatternMaskingStrategy.STEADY -> 96 to 224
            PatternMaskingStrategy.PULSE -> 160 to 448
            PatternMaskingStrategy.RANDOMIZED -> 192 to 896
            PatternMaskingStrategy.BURST_FADE -> 256 to 1280
            PatternMaskingStrategy.QUIET_SWEEP -> 72 to 192
        }
        val minBytes = base.first + ((digest[3].toInt() and 0xff) % 48)
        var maxBytes = base.second + ((digest[4].toInt() and 0xff) % 192)
        if (maxBytes <= minBytes) {
            maxBytes = minBytes + 128
        }
        return minBytes to maxBytes
    }

    private fun burstRange(strategy: PatternMaskingStrategy, digest: ByteArray): Pair<Int, Int> {
        val base = when (strategy) {
            PatternMaskingStrategy.STEADY -> 1800 to 3200
            PatternMaskingStrategy.PULSE -> 900 to 2200
            PatternMaskingStrategy.RANDOMIZED -> 700 to 2600
            PatternMaskingStrategy.BURST_FADE -> 500 to 1800
            PatternMaskingStrategy.QUIET_SWEEP -> 2400 to 4200
        }
        val minMs = base.first + ((digest[5].toInt() and 0xff) % 220)
        var maxMs = base.second + ((digest[6].toInt() and 0xff) % 480)
        if (maxMs <= minMs) {
            maxMs = minMs + 400
        }
        return minMs to maxMs
    }

    private fun idleRange(strategy: PatternMaskingStrategy, digest: ByteArray): Pair<Int, Int> {
        val base = when (strategy) {
            PatternMaskingStrategy.STEADY -> 2600 to 5200
            PatternMaskingStrategy.PULSE -> 1500 to 3600
            PatternMaskingStrategy.RANDOMIZED -> 900 to 5000
            PatternMaskingStrategy.BURST_FADE -> 700 to 3200
            PatternMaskingStrategy.QUIET_SWEEP -> 3200 to 6400
        }
        val minMs = base.first + ((digest[7].toInt() and 0xff) % 300)
        var maxMs = base.second + ((digest[8].toInt() and 0xff) % 720)
        if (maxMs <= minMs) {
            maxMs = minMs + 600
        }
        return minMs to maxMs
    }

    private fun paddingProfile(strategy: PatternMaskingStrategy, digest: ByteArray): String {
        val suffixes = listOf("light", "mixed", "dense")
        return strategy.paddingProfile + "-" + suffixes[(digest[9].toInt() and 0xff) % suffixes.size]
    }

    private fun sha256(input: String): ByteArray {
        val digest = MessageDigest.getInstance("SHA-256")
        return digest.digest(input.toByteArray(Charsets.UTF_8))
    }
}
