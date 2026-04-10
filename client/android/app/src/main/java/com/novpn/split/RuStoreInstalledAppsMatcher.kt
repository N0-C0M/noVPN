package com.novpn.split

import android.content.Context
import android.os.Build
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit
import java.net.HttpURLConnection
import java.net.URL
import java.util.Locale
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicInteger

data class RuStoreAppCandidate(
    val packageName: String,
    val label: String,
    val reasons: List<String>
)

class RuStoreInstalledAppsMatcher(private val context: Context) {
    private val packageManager = context.packageManager
    private val catalogChecker = RuStoreCatalogChecker()

    suspend fun match(
        entries: List<InstalledAppEntry>,
        onProgress: (completed: Int, total: Int, currentLabel: String) -> Unit = { _, _, _ -> }
    ): List<RuStoreAppCandidate> = coroutineScope {
        val eligibleEntries = entries
            .asSequence()
            .filterNot { it.packageName == context.packageName }
            .filterNot { isTelegramPackage(it.packageName) }
            .sortedBy { it.label.lowercase(Locale.ROOT) }
            .toList()
        val total = eligibleEntries.size
        val completed = AtomicInteger(0)
        val concurrency = Semaphore(MAX_PARALLEL_CHECKS)

        eligibleEntries.map { entry ->
            async(Dispatchers.IO) {
                concurrency.withPermit {
                    val candidate = matchEntry(entry)
                    onProgress(completed.incrementAndGet(), total, entry.label)
                    candidate
                }
            }
        }.awaitAll()
            .filterNotNull()
            .sortedBy { it.label.lowercase(Locale.ROOT) }
    }

    private fun matchEntry(entry: InstalledAppEntry): RuStoreAppCandidate? {
        val packageName = entry.packageName

        if (installerPackageName(packageName).lowercase(Locale.ROOT) in RUSTORE_INSTALLERS) {
            return RuStoreAppCandidate(
                packageName = packageName,
                label = entry.label,
                reasons = listOf("installed from RuStore")
            )
        }

        if (!catalogChecker.isPublished(packageName)) {
            return null
        }

        return RuStoreAppCandidate(
            packageName = packageName,
            label = entry.label,
            reasons = listOf("RuStore catalog page found")
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

    private fun isTelegramPackage(packageName: String): Boolean {
        return packageName.lowercase(Locale.ROOT).startsWith(TELEGRAM_PACKAGE_PREFIX)
    }

    companion object {
        private val RUSTORE_INSTALLERS = setOf(
            "ru.vk.store",
            "ru.vk.rustore"
        )
        private const val TELEGRAM_PACKAGE_PREFIX = "org.telegram."
        private const val MAX_PARALLEL_CHECKS = 6
    }
}

private class RuStoreCatalogChecker {
    fun isPublished(packageName: String): Boolean {
        presenceCache[packageName]?.let { return it }
        val published = fetchCatalogPresence(packageName)
        presenceCache[packageName] = published
        return published
    }

    private fun fetchCatalogPresence(packageName: String): Boolean {
        val connection = (URL("$CATALOG_BASE_URL/$packageName").openConnection() as HttpURLConnection).apply {
            requestMethod = "GET"
            instanceFollowRedirects = true
            connectTimeout = CONNECT_TIMEOUT_MS
            readTimeout = READ_TIMEOUT_MS
            setRequestProperty("User-Agent", USER_AGENT)
            setRequestProperty("Accept", "text/html,application/xhtml+xml")
        }

        val published = runCatching {
            connection.connect()
            val responseCode = connection.responseCode
            if (responseCode == HttpURLConnection.HTTP_NOT_FOUND || responseCode !in 200..299) {
                false
            } else {
                val bodySnippet = connection.inputStream.bufferedReader().use { reader ->
                    val buffer = CharArray(SNIPPET_LENGTH)
                    val readCount = reader.read(buffer)
                    if (readCount <= 0) {
                        ""
                    } else {
                        String(buffer, 0, readCount).lowercase(Locale.ROOT)
                    }
                }
                val expectedPath = "/catalog/app/${packageName.lowercase(Locale.ROOT)}"
                bodySnippet.contains(expectedPath) && MISSING_MARKERS.none(bodySnippet::contains)
            }
        }.getOrDefault(false)
        connection.disconnect()
        return published
    }

    companion object {
        private const val CATALOG_BASE_URL = "https://www.rustore.ru/catalog/app"
        private const val CONNECT_TIMEOUT_MS = 1800
        private const val READ_TIMEOUT_MS = 2200
        private const val SNIPPET_LENGTH = 16384
        private const val USER_AGENT = "Mozilla/5.0 (Android) NoVPN/1.0"
        private val MISSING_MARKERS = listOf(
            "страница не найдена",
            "404",
            "not found"
        )
        private val presenceCache = ConcurrentHashMap<String, Boolean>()
    }
}
