package com.novpn.data

enum class AppRoutingMode(val storageValue: String) {
    EXCLUDE_SELECTED("exclude_selected"),
    ONLY_SELECTED("only_selected");

    companion object {
        fun fromStorage(value: String?): AppRoutingMode {
            return entries.firstOrNull { it.storageValue == value } ?: EXCLUDE_SELECTED
        }
    }
}

enum class TrafficObfuscationStrategy(
    val storageValue: String,
    val fingerprint: String,
    val spiderXPath: String
) {
    BALANCED("balanced", "chrome", "/"),
    CDN_MIMIC("cdn_mimic", "chrome", "/cdn-cgi/trace"),
    FRAGMENTED("fragmented", "safari", "/assets");

    companion object {
        fun fromStorage(value: String?): TrafficObfuscationStrategy {
            return entries.firstOrNull { it.storageValue == value } ?: BALANCED
        }
    }
}

enum class PatternMaskingStrategy(
    val storageValue: String,
    val spiderXPath: String,
    val jitterWindowMs: Int,
    val paddingProfile: String
) {
    STEADY("steady", "/robots.txt", 60, "steady"),
    PULSE("pulse", "/cdn-cgi/trace", 180, "pulse"),
    RANDOMIZED("randomized", "/assets/cache", 320, "randomized");

    companion object {
        fun fromStorage(value: String?): PatternMaskingStrategy {
            return entries.firstOrNull { it.storageValue == value } ?: STEADY
        }
    }
}

object ServerLocationCatalog {
    private val labelsByAddress = mapOf(
        "2.26.85.47" to "Финляндия"
    )

    fun labelFor(address: String): String {
        return labelsByAddress[address.trim()].orEmpty()
    }
}
