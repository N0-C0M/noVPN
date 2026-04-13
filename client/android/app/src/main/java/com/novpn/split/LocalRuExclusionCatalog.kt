package com.novpn.split

import android.content.Context
import com.novpn.data.AssetPayloadCodec
import java.util.Locale

data class LocalRuExclusionCatalog(
    val extraDomains: List<String>,
    val exactDomains: Set<String>,
    val exactPackages: Set<String>,
    val companyPrefixes: List<String>
)

object LocalRuExclusionCatalogLoader {
    private const val APP_PACKAGES_ASSET = "catalog/c0.bin"
    private const val SITE_LIST_ASSET = "catalog/c1.bin"
    private const val APP_PACKAGES_SALT = "catalog-app-packages-v1"
    private const val SITE_LIST_SALT = "catalog-site-list-v1"

    @Volatile
    private var cachedCatalog: LocalRuExclusionCatalog? = null

    fun load(context: Context): LocalRuExclusionCatalog {
        cachedCatalog?.let { return it }
        synchronized(this) {
            cachedCatalog?.let { return it }

            val packageText = AssetPayloadCodec.decodeAssetText(
                context = context,
                assetPath = APP_PACKAGES_ASSET,
                salt = APP_PACKAGES_SALT
            )
            val siteText = AssetPayloadCodec.decodeAssetText(
                context = context,
                assetPath = SITE_LIST_ASSET,
                salt = SITE_LIST_SALT
            )
            val siteDomains = parseSiteDomains(siteText)
            val siteDomainSet = siteDomains.toSet()
            val exactPackages = parsePackageIds("$packageText\n$siteText")
                .filterTo(linkedSetOf()) { token -> shouldKeepPackageToken(token, siteDomains) }

            return LocalRuExclusionCatalog(
                extraDomains = siteDomains,
                exactDomains = siteDomainSet,
                exactPackages = exactPackages,
                companyPrefixes = buildCompanyPrefixes(exactPackages)
            ).also { cachedCatalog = it }
        }
    }

    private fun parsePackageIds(text: String): Set<String> {
        return PACKAGE_REGEX.findAll(text)
            .map { it.value.lowercase(Locale.ROOT) }
            .mapNotNull(::normalizePackageToken)
            .toCollection(linkedSetOf())
    }

    private fun parseSiteDomains(text: String): List<String> {
        return DOMAIN_REGEX.findAll(text)
            .map { it.value.lowercase(Locale.ROOT) }
            .mapNotNull(::normalizeSiteToken)
            .filterNot(::isGoogleOrYoutubeDomain)
            .distinct()
            .sorted()
            .toList()
    }

    private fun buildCompanyPrefixes(packages: Set<String>): List<String> {
        val twoSegmentCounts = mutableMapOf<String, Int>()
        val threeSegmentCounts = mutableMapOf<String, Int>()

        packages.forEach { packageName ->
            val parts = packageName.split('.')
            if (parts.size >= 2) {
                val key = parts.take(2).joinToString(".")
                twoSegmentCounts[key] = (twoSegmentCounts[key] ?: 0) + 1
            }
            if (parts.size >= 3) {
                val key = parts.take(3).joinToString(".")
                threeSegmentCounts[key] = (threeSegmentCounts[key] ?: 0) + 1
            }
        }

        return buildSet {
            twoSegmentCounts.filterValues { it >= 2 }.keys.forEach(::add)
            threeSegmentCounts.filterValues { it >= 2 }.keys.forEach(::add)
        }
            .sortedByDescending { it.length }
    }

    private fun normalizePackageToken(value: String): String? {
        val normalized = value
            .trim()
            .trim('.', ',', ';', ':', '(', ')', '"', '\'')
            .lowercase(Locale.ROOT)
        if (normalized.count { it == '.' } < 1) {
            return null
        }
        if (!PACKAGE_NORMALIZED_REGEX.matches(normalized)) {
            return null
        }
        return normalized
    }

    private fun normalizeSiteToken(value: String): String? {
        val trimmed = value
            .trim()
            .trim('.', ',', ';', ':', '(', ')', '"', '\'')
            .lowercase(Locale.ROOT)
        if (trimmed.contains("/search?q=") && trimmed.contains("google.")) {
            return null
        }

        val noScheme = trimmed
            .removePrefix("https://")
            .removePrefix("http://")
        val host = noScheme
            .substringBefore('/')
            .substringBefore('?')
            .substringBefore('#')
            .removePrefix("www.")
            .trim('.')

        if (host.count { it == '.' } < 1) {
            return null
        }
        if (!HOST_REGEX.matches(host)) {
            return null
        }
        return host
    }

    private fun isGoogleOrYoutubeDomain(host: String): Boolean {
        return GOOGLE_AND_YOUTUBE_DOMAIN_SUFFIXES.any { suffix ->
            host == suffix || host.endsWith(".$suffix")
        }
    }

    private fun shouldKeepPackageToken(token: String, siteDomains: List<String>): Boolean {
        val parts = token.split('.')
        if (parts.size >= 3) {
            return true
        }
        if (parts.firstOrNull() in COMMON_PACKAGE_ROOTS) {
            return true
        }
        return token !in siteDomains && parts.lastOrNull() !in COMMON_SITE_TLDS
    }

    private val PACKAGE_REGEX = Regex("""(?<![A-Za-z0-9_])(?:[A-Za-z][A-Za-z0-9_]*\.){1,}[A-Za-z0-9_]+(?![A-Za-z0-9_])""")
    private val DOMAIN_REGEX = Regex("""(?<![A-Za-z0-9-])(?:https?://)?(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}(?:/[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]*)?""")
    private val PACKAGE_NORMALIZED_REGEX = Regex("""^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$""")
    private val HOST_REGEX = Regex("""^[a-z0-9-]+(\.[a-z0-9-]+)+$""")
    private val COMMON_PACKAGE_ROOTS = setOf("com", "ru", "org", "net", "io", "app", "mobi", "me")
    private val COMMON_SITE_TLDS = setOf(
        "ru", "com", "org", "net", "io", "cc", "tv", "me", "pro", "one", "travel",
        "life", "cloud", "tech", "games", "app", "ag", "to", "se"
    )
    private val GOOGLE_AND_YOUTUBE_DOMAIN_SUFFIXES = setOf(
        "google.com",
        "google.ru",
        "googleapis.com",
        "gstatic.com",
        "gvt1.com",
        "googlevideo.com",
        "youtube.com",
        "ytimg.com",
        "googleusercontent.com",
        "ggpht.com",
        "youtubei.googleapis.com"
    )
}
