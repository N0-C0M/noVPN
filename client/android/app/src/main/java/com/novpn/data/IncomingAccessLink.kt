package com.novpn.data

import android.net.Uri
import java.util.Locale

data class IncomingAccessLink(
    val kind: IncomingAccessKind,
    val value: String,
    val apiBaseOverride: String = ""
)

enum class IncomingAccessKind {
    INVITE_CODE,
    SUBSCRIPTION_URL,
    VLESS_URL
}

object IncomingAccessLinkParser {
    fun parse(uri: Uri?): IncomingAccessLink? {
        return parseInternal(uri?.normalizeScheme(), depth = 0)
    }

    private fun parseInternal(uri: Uri?, depth: Int): IncomingAccessLink? {
        if (uri == null || depth > MAX_NESTED_DEPTH) {
            return null
        }
        val rawValue = uri.toString().trim()
        if (rawValue.isBlank()) {
            return null
        }

        val scheme = uri.scheme?.trim()?.lowercase(Locale.US).orEmpty()
        return when (scheme) {
            "vless" -> IncomingAccessLink(
                kind = IncomingAccessKind.VLESS_URL,
                value = rawValue
            )
            "http", "https" -> {
                when {
                    isRedeemPath(uri) -> extractInviteCode(uri)?.let { code ->
                        IncomingAccessLink(
                            kind = IncomingAccessKind.INVITE_CODE,
                            value = code
                        )
                    }
                    isSubscriptionPath(uri) -> IncomingAccessLink(
                        kind = IncomingAccessKind.SUBSCRIPTION_URL,
                        value = rawValue,
                        apiBaseOverride = deriveApiBase(uri)
                    )
                    else -> {
                        parseNestedLink(uri, depth + 1)
                            ?: extractInviteCode(uri)?.let { code ->
                                IncomingAccessLink(
                                    kind = IncomingAccessKind.INVITE_CODE,
                                    value = code
                                )
                            }
                    }
                }
            }
            "happ", "novpn" -> {
                parseNestedLink(uri, depth + 1)
                    ?: extractInviteCode(uri)?.let { code ->
                        IncomingAccessLink(
                            kind = IncomingAccessKind.INVITE_CODE,
                            value = code
                        )
                    }
            }
            else -> null
        }
    }

    private fun parseNestedLink(uri: Uri, depth: Int): IncomingAccessLink? {
        for (queryKey in NESTED_LINK_QUERY_KEYS) {
            val nested = uri.getQueryParameter(queryKey)?.trim().orEmpty()
            if (nested.isBlank()) {
                continue
            }
            parseInternal(Uri.parse(nested), depth)?.let { parsed ->
                if (parsed.apiBaseOverride.isNotBlank()) {
                    return parsed
                }
                val override = uri.getQueryParameter("api_base")?.trim().orEmpty()
                if (override.isBlank()) {
                    return parsed
                }
                return parsed.copy(apiBaseOverride = override)
            }
        }
        return null
    }

    private fun extractInviteCode(uri: Uri): String? {
        for (queryKey in INVITE_CODE_QUERY_KEYS) {
            uri.getQueryParameter(queryKey)
                ?.trim()
                ?.takeIf { it.isNotBlank() }
                ?.let { return it }
        }

        val segments = uri.pathSegments.orEmpty()
        val redeemIndex = segments.indexOf("redeem")
        if (redeemIndex >= 0 && redeemIndex + 1 < segments.size) {
            return segments[redeemIndex + 1].trim().takeIf { it.isNotBlank() }
        }

        if (uri.scheme.equals("happ", ignoreCase = true) || uri.scheme.equals("novpn", ignoreCase = true)) {
            return segments.lastOrNull()?.trim()?.takeIf { it.isNotBlank() }
        }
        return null
    }

    private fun isRedeemPath(uri: Uri): Boolean {
        val path = uri.path.orEmpty()
        if (path.contains("/redeem/")) {
            return true
        }
        return uri.getQueryParameter("code")?.isNotBlank() == true ||
            uri.getQueryParameter("invite")?.isNotBlank() == true ||
            uri.getQueryParameter("invite_code")?.isNotBlank() == true
    }

    private fun isSubscriptionPath(uri: Uri): Boolean {
        val path = uri.path.orEmpty()
        return path.startsWith("/s/") ||
            path == "/admin/client/subscription" ||
            path.startsWith("/admin/client/subscription/")
    }

    private fun deriveApiBase(uri: Uri): String {
        val scheme = uri.scheme?.trim().orEmpty()
        val authority = uri.encodedAuthority?.trim().orEmpty()
        if (scheme.isBlank() || authority.isBlank()) {
            return ""
        }
        return "$scheme://$authority/admin"
    }

    private const val MAX_NESTED_DEPTH = 2

    private val NESTED_LINK_QUERY_KEYS = listOf(
        "url",
        "link",
        "subscription",
        "subscription_url",
        "href"
    )

    private val INVITE_CODE_QUERY_KEYS = listOf(
        "code",
        "invite",
        "invite_code"
    )
}
