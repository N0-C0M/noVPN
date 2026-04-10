package com.novpn.split

import android.content.Context
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.util.Locale

class LocalRuAppExclusionMatcher(context: Context) {
    private val catalog = LocalRuExclusionCatalogLoader.load(context)

    suspend fun match(
        entries: List<InstalledAppEntry>,
        onProgress: suspend (completed: Int, total: Int, currentLabel: String) -> Unit = { _, _, _ -> }
    ): List<LocalRuAppCandidate> = withContext(Dispatchers.Default) {
        val total = entries.size
        val matches = mutableListOf<LocalRuAppCandidate>()
        entries.sortedBy { it.label.lowercase(Locale.ROOT) }.forEachIndexed { index, entry ->
            matchEntry(entry)?.let(matches::add)
            onProgress(index + 1, total, entry.label)
        }
        matches
    }

    private fun matchEntry(entry: InstalledAppEntry): LocalRuAppCandidate? {
        val reasons = matchPackageName(entry.packageName)
        if (reasons.isEmpty()) {
            return null
        }

        return LocalRuAppCandidate(
            packageName = entry.packageName,
            label = entry.label,
            reasons = reasons
        )
    }

    fun matchPackageName(packageName: String): List<String> {
        val normalized = packageName.trim().lowercase(Locale.ROOT)
        val reasons = linkedSetOf<String>()

        if (normalized in catalog.exactPackages) {
            reasons += "Matched package from ru app package list"
        }
        if (catalog.companyPrefixes.any { normalized == it || normalized.startsWith("$it.") }) {
            reasons += "Matched known vendor package prefix"
        }
        if (normalized.startsWith("ru.") || normalized.contains(".ru.") || normalized.endsWith(".ru")) {
            reasons += "Matched ru package pattern"
        }

        return reasons.toList()
    }
}
