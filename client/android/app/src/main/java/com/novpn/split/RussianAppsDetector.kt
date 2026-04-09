package com.novpn.split

import android.content.Context
import android.content.pm.ApplicationInfo
import android.os.Build

data class RussianAppCandidate(
    val packageName: String,
    val label: String,
    val confidence: Confidence,
    val reasons: List<String>
) {
    enum class Confidence {
        HIGH,
        MEDIUM
    }
}

data class RussianAppsScanResult(
    val candidates: List<RussianAppCandidate>,
    val addedPackages: List<String>,
    val selectedPackages: List<String>
)

class RussianAppsDetector(private val context: Context) {
    private val packageManager = context.packageManager

    fun detectLikelyRussianApps(limit: Int = Int.MAX_VALUE): List<RussianAppCandidate> {
        return packageManager.getInstalledApplications(0)
            .asSequence()
            .filter(::isUserLaunchableApp)
            .mapNotNull(::scoreCandidate)
            .sortedWith(
                compareByDescending<RussianAppCandidate> {
                    it.confidence == RussianAppCandidate.Confidence.HIGH
                }.thenBy { it.label.lowercase() }
            )
            .take(limit)
            .toList()
    }

    private fun isUserLaunchableApp(applicationInfo: ApplicationInfo): Boolean {
        val hasLauncher = packageManager.getLaunchIntentForPackage(applicationInfo.packageName) != null
        if (!hasLauncher || applicationInfo.packageName == context.packageName) {
            return false
        }
        val flags = applicationInfo.flags
        val isSystem = flags and ApplicationInfo.FLAG_SYSTEM != 0 ||
            flags and ApplicationInfo.FLAG_UPDATED_SYSTEM_APP != 0
        return !isSystem
    }

    private fun scoreCandidate(applicationInfo: ApplicationInfo): RussianAppCandidate? {
        val packageName = applicationInfo.packageName
        val label = packageManager.getApplicationLabel(applicationInfo).toString()
        val normalizedPackage = packageName.lowercase()
        val normalizedLabel = label.lowercase()
        val normalizedInstaller = installerPackageName(packageName).lowercase()
        val reasons = linkedSetOf<String>()
        var score = 0
        var strongSignal = false

        if (normalizedInstaller in RUSTORE_INSTALLERS) {
            score += 5
            strongSignal = true
            reasons += "installed from RuStore"
        }

        if (normalizedPackage.startsWith("ru.") || normalizedPackage.contains(".ru.")) {
            score += 3
            strongSignal = true
            reasons += "package name uses a .ru domain"
        }

        if (STRONG_PACKAGE_PREFIXES.any { normalizedPackage.startsWith(it) }) {
            score += 5
            strongSignal = true
            reasons += "package matches a known Russian service"
        }

        val brandHits = BRAND_TOKENS.filter { token ->
            normalizedPackage.contains(token) || normalizedLabel.contains(token)
        }
        if (brandHits.isNotEmpty()) {
            score += 4
            strongSignal = true
            reasons += "name matches a known Russian brand"
        }

        if (containsCyrillic(label)) {
            score += 1
            reasons += "app name contains Cyrillic"
        }

        if (normalizedLabel.contains(".ru") || normalizedLabel.contains("рос") || normalizedLabel.contains("russia")) {
            score += 2
            reasons += "label hints at a Russian publisher"
        }

        val confidence = when {
            strongSignal && score >= 8 -> RussianAppCandidate.Confidence.HIGH
            strongSignal && score >= 5 -> RussianAppCandidate.Confidence.MEDIUM
            else -> null
        } ?: return null

        return RussianAppCandidate(
            packageName = packageName,
            label = label,
            confidence = confidence,
            reasons = reasons.toList()
        )
    }

    private fun installerPackageName(packageName: String): String {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            runCatching {
                packageManager.getInstallSourceInfo(packageName).installingPackageName.orEmpty()
            }.getOrDefault("")
        } else {
            @Suppress("DEPRECATION")
            runCatching {
                packageManager.getInstallerPackageName(packageName).orEmpty()
            }.getOrDefault("")
        }
    }

    private fun containsCyrillic(value: String): Boolean {
        return value.any { symbol -> symbol in '\u0400'..'\u04FF' }
    }

    companion object {
        private val RUSTORE_INSTALLERS = setOf(
            "ru.vk.store",
            "ru.vk.rustore"
        )

        private val STRONG_PACKAGE_PREFIXES = listOf(
            "com.yandex.",
            "ru.yandex.",
            "com.vkontakte.",
            "ru.vk.",
            "ru.mail.",
            "com.mail.",
            "ru.sber.",
            "ru.gosuslugi.",
            "com.tbank.",
            "ru.tbank.",
            "com.tinkoff.",
            "ru.tinkoff.",
            "ru.alfabank.",
            "ru.avito.",
            "com.avito.",
            "ru.ozon.",
            "com.ozon.",
            "ru.wildberries.",
            "com.wildberries.",
            "ru.rutube.",
            "ru.hh.",
            "ru.megafon.",
            "ru.mts.",
            "ru.beeline.",
            "ru.rostelecom.",
            "ru.gazprombank."
        )

        private val BRAND_TOKENS = listOf(
            "яндекс",
            "yandex",
            "вконтакте",
            "vkontakte",
            "vk видео",
            "vk music",
            "vk id",
            "mail.ru",
            "mailru",
            "сбер",
            "sber",
            "тинькофф",
            "tinkoff",
            "т-банк",
            "tbank",
            "альфа",
            "alfabank",
            "газпром",
            "gazprom",
            "госуслуги",
            "gosuslugi",
            "ozon",
            "озон",
            "wildberries",
            "вайлдберриз",
            "авито",
            "avito",
            "rutube",
            "рутуб",
            "megafon",
            "мегафон",
            "мтс",
            "beeline",
            "билайн",
            "ростелеком",
            "rostelecom",
            "2gis",
            "2гис",
            "однокласс",
            "mirpay",
            "мир pay",
            "мирпэй",
            "rambler",
            "hh.ru"
        )
    }
}
