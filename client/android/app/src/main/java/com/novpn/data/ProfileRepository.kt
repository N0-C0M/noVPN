package com.novpn.data

import android.content.Context
import android.net.Uri
import org.json.JSONObject
import java.io.File
import java.util.Locale
import java.util.UUID

class ProfileRepository(private val context: Context) {
    private val preferences by lazy { ClientPreferences(context) }
    private val importedProfilesDir by lazy {
        File(context.filesDir, "profiles").apply { mkdirs() }
    }
    private val bootstrapFallbackAddress by lazy { loadBootstrapServerAddress() }

    fun defaultProfileId(): String {
        return listProfileEntries().firstOrNull()?.profileId.orEmpty()
    }

    fun loadDefaultProfile(): ClientProfile {
        val profileId = defaultProfileId()
        require(profileId.isNotBlank()) { "Сначала активируйте код или импортируйте профиль сервера." }
        return loadProfile(profileId)
    }

    fun bootstrapServerAddress(): String {
        return bootstrapFallbackAddress
    }

    fun loadProfile(profileId: String): ClientProfile {
        require(profileId.isNotBlank()) { "Сначала активируйте код или импортируйте профиль сервера." }
        val entry = resolveProfileEntry(profileId)
        val payload = when (entry.source) {
            ProfileSource.ASSET -> context.assets.open(entry.fileName).bufferedReader().use { it.readText() }
            ProfileSource.IMPORTED -> File(importedProfilesDir, entry.fileName).readText()
        }
        return parseClientProfileJson(payload)
    }

    fun importProfile(uri: Uri): AvailableProfile {
        val payload = context.contentResolver.openInputStream(uri)?.bufferedReader()?.use { it.readText() }
            ?: throw IllegalArgumentException("Не удалось прочитать выбранный файл профиля.")
        return importProfilePayload(payload, uri.lastPathSegment.orEmpty())
    }

    fun importProfilePayloads(payloads: List<String>, nameHint: String = ""): List<AvailableProfile> {
        val normalizedPayloads = payloads
            .map { it.trim() }
            .filter { it.isNotBlank() }

        require(normalizedPayloads.isNotEmpty()) {
            "No profiles were provided for import."
        }

        return normalizedPayloads.mapIndexed { index, payload ->
            val hint = if (nameHint.isBlank()) {
                "imported-${index + 1}"
            } else {
                "$nameHint-${index + 1}"
            }
            importProfilePayload(payload, hint)
        }
    }

    fun importProfilePayload(payload: String, nameHint: String = ""): AvailableProfile {
        val profile = parseImportedPayload(payload)
        profile.requireRuntimeReady()

        val serializedProfile = serializeProfile(profile)
        val fileName = buildImportedFileName(nameHint, profile, serializedProfile)
        val outputFile = File(importedProfilesDir, fileName)
        outputFile.writeText(serializedProfile)
        return AvailableProfile(
            profileId = fileProfileId(fileName),
            name = profile.name,
            address = "${profile.server.address}:${profile.server.port}",
            serverName = profile.server.serverName,
            locationLabel = profile.server.locationLabel,
            isImported = true
        )
    }

    fun deleteProfile(profileId: String) {
        val entry = resolveProfileEntry(profileId)
        if (entry.source != ProfileSource.IMPORTED) {
            return
        }
        File(importedProfilesDir, entry.fileName).delete()
    }

    fun isImportedProfile(profileId: String): Boolean {
        return resolveProfileEntry(profileId).source == ProfileSource.IMPORTED
    }

    fun listProfiles(): List<AvailableProfile> {
        return listProfileEntries().map { entry ->
            val profile = loadProfile(entry.profileId)
            AvailableProfile(
                profileId = entry.profileId,
                name = profile.name,
                address = "${profile.server.address}:${profile.server.port}",
                serverName = profile.server.serverName,
                locationLabel = profile.server.locationLabel,
                isImported = entry.source == ProfileSource.IMPORTED
            )
        }
    }

    private fun parseClientProfileJson(payload: String): ClientProfile {
        val root = JSONObject(payload)
        val server = root.optJSONObject("server") ?: root
        val local = root.optJSONObject("local")
        val obfuscation = root.optJSONObject("obfuscation")
        val shortId = server.optString("short_id").ifBlank {
            server.optJSONArray("short_ids")
                ?.optString(0)
                .orEmpty()
        }
        val address = normalizeServerAddress(server.getString("address"))
        val locationLabel = server.optString("location_label")
            .ifBlank { ServerLocationCatalog.labelFor(address) }
        val seed = obfuscation?.optString("seed")
            ?.takeIf { it.isNotBlank() }
            ?: defaultSeed(shortId)
        val trafficStrategy = TrafficObfuscationStrategy.fromStorage(
            obfuscation?.optString("traffic_strategy")
        )
        val patternStrategy = PatternMaskingStrategy.fromStorage(
            obfuscation?.optString("pattern_strategy")
        )

        return ClientProfile(
            name = root.optString("name").ifBlank { DEFAULT_PROFILE_NAME },
            server = ServerProfile(
                address = address,
                port = server.getInt("port"),
                uuid = server.getString("uuid"),
                flow = server.optString("flow", DEFAULT_FLOW),
                serverName = server.getString("server_name"),
                fingerprint = server.optString("fingerprint", DEFAULT_FINGERPRINT),
                publicKey = server.getString("public_key"),
                shortId = shortId,
                locationLabel = locationLabel,
                spiderX = server.optString("spider_x", "/")
            ),
            local = LocalPorts(
                socksListen = local?.optString("socks_listen", DEFAULT_SOCKS_LISTEN) ?: DEFAULT_SOCKS_LISTEN,
                socksPort = local?.optInt("socks_port", DEFAULT_SOCKS_PORT) ?: DEFAULT_SOCKS_PORT,
                httpListen = local?.optString("http_listen", DEFAULT_HTTP_LISTEN) ?: DEFAULT_HTTP_LISTEN,
                httpPort = local?.optInt("http_port", DEFAULT_HTTP_PORT) ?: DEFAULT_HTTP_PORT
            ),
            obfuscation = ObfuscationProfile(
                seed = seed,
                trafficStrategy = trafficStrategy,
                patternStrategy = patternStrategy
            )
        )
    }

    private fun parseImportedPayload(payload: String): ClientProfile {
        val trimmed = payload.trim()
        return if (trimmed.startsWith("{")) {
            parseClientProfileJson(trimmed)
        } else {
            parseServerClientProfileYaml(trimmed)
        }
    }

    private fun parseServerClientProfileYaml(payload: String): ClientProfile {
        val scalars = mutableMapOf<String, String>()
        val shortIds = mutableListOf<String>()
        var activeList: String? = null

        payload.lineSequence().forEach { rawLine ->
            val line = rawLine.substringBefore(" #").trimEnd()
            if (line.isBlank()) {
                return@forEach
            }

            val trimmed = line.trim()
            if (trimmed.startsWith("- ")) {
                if (activeList == "short_ids") {
                    shortIds += trimYamlValue(trimmed.removePrefix("- ").trim())
                }
                return@forEach
            }

            if (trimmed.endsWith(":")) {
                activeList = trimmed.removeSuffix(":").trim()
                return@forEach
            }

            activeList = null
            val separatorIndex = trimmed.indexOf(':')
            if (separatorIndex <= 0) {
                return@forEach
            }

            val key = trimmed.substring(0, separatorIndex).trim()
            val value = trimYamlValue(trimmed.substring(separatorIndex + 1).trim())
            scalars[key] = value
        }

        val shortId = scalars["short_id"].orEmpty().ifBlank { shortIds.firstOrNull().orEmpty() }
        val address = normalizeServerAddress(scalars["address"].orEmpty())
        return ClientProfile(
            name = scalars["name"].orEmpty().ifBlank { DEFAULT_IMPORTED_NAME },
            server = ServerProfile(
                address = address,
                port = scalars["port"]?.toIntOrNull() ?: 0,
                uuid = scalars["uuid"].orEmpty(),
                flow = scalars["flow"].orEmpty().ifBlank { DEFAULT_FLOW },
                serverName = scalars["server_name"].orEmpty(),
                fingerprint = scalars["fingerprint"].orEmpty().ifBlank { DEFAULT_FINGERPRINT },
                publicKey = scalars["public_key"].orEmpty(),
                shortId = shortId,
                locationLabel = ServerLocationCatalog.labelFor(address),
                spiderX = scalars["spider_x"].orEmpty().ifBlank { "/" }
            ),
            local = LocalPorts(
                socksListen = DEFAULT_SOCKS_LISTEN,
                socksPort = DEFAULT_SOCKS_PORT,
                httpListen = DEFAULT_HTTP_LISTEN,
                httpPort = DEFAULT_HTTP_PORT
            ),
            obfuscation = ObfuscationProfile(
                seed = defaultSeed(shortId),
                trafficStrategy = TrafficObfuscationStrategy.BALANCED,
                patternStrategy = PatternMaskingStrategy.STEADY
            )
        )
    }

    private fun serializeProfile(profile: ClientProfile): String {
        val document = JSONObject()
            .put("name", profile.name)
            .put(
                "server",
                JSONObject()
                    .put("address", profile.server.address)
                    .put("port", profile.server.port)
                    .put("uuid", profile.server.uuid)
                    .put("flow", profile.server.flow)
                    .put("server_name", profile.server.serverName)
                    .put("fingerprint", profile.server.fingerprint)
                    .put("public_key", profile.server.publicKey)
                    .put("short_id", profile.server.shortId)
                    .put("location_label", profile.server.locationLabel)
                    .put("spider_x", profile.server.spiderX)
            )
            .put(
                "local",
                JSONObject()
                    .put("socks_listen", profile.local.socksListen)
                    .put("socks_port", profile.local.socksPort)
                    .put("http_listen", profile.local.httpListen)
                    .put("http_port", profile.local.httpPort)
            )
            .put(
                "obfuscation",
                JSONObject()
                    .put("seed", profile.obfuscation.seed)
                    .put("traffic_strategy", profile.obfuscation.trafficStrategy.storageValue)
                    .put("pattern_strategy", profile.obfuscation.patternStrategy.storageValue)
            )

        return document.toString(2) + "\n"
    }

    private fun listProfileEntries(): List<ProfileEntry> {
        return listImportedProfileEntries()
    }

    private fun listImportedProfileEntries(): List<ProfileEntry> {
        return importedProfilesDir.listFiles()
            .orEmpty()
            .filter { it.isFile && it.name.startsWith("profile.") && it.name.endsWith(".json") }
            .sortedBy { it.name.lowercase(Locale.ROOT) }
            .map { file ->
                ProfileEntry(
                    profileId = fileProfileId(file.name),
                    fileName = file.name,
                    source = ProfileSource.IMPORTED
                )
            }
    }

    private fun resolveProfileEntry(profileId: String): ProfileEntry {
        return when {
            profileId.startsWith(ASSET_PREFIX) -> {
                val fileName = profileId.removePrefix(ASSET_PREFIX)
                ProfileEntry(profileId = assetProfileId(fileName), fileName = fileName, source = ProfileSource.ASSET)
            }
            profileId.startsWith(FILE_PREFIX) -> {
                val fileName = profileId.removePrefix(FILE_PREFIX)
                ProfileEntry(profileId = fileProfileId(fileName), fileName = fileName, source = ProfileSource.IMPORTED)
            }
            assetProfileExists(profileId) -> {
                ProfileEntry(profileId = assetProfileId(profileId), fileName = profileId, source = ProfileSource.ASSET)
            }
            File(importedProfilesDir, profileId).exists() -> {
                ProfileEntry(profileId = fileProfileId(profileId), fileName = profileId, source = ProfileSource.IMPORTED)
            }
            else -> throw IllegalArgumentException("Профиль сервера не найден. Импортируйте или активируйте новый профиль.")
        }
    }

    private fun assetProfileExists(fileName: String): Boolean {
        return context.assets.list("")
            .orEmpty()
            .any { it == fileName }
    }

    private fun assetProfileEntryOrNull(fileName: String): ProfileEntry? {
        if (!assetProfileExists(fileName)) {
            return null
        }
        return ProfileEntry(
            profileId = assetProfileId(fileName),
            fileName = fileName,
            source = ProfileSource.ASSET
        )
    }

    private fun buildImportedFileName(
        nameHint: String,
        profile: ClientProfile,
        serializedProfile: String
    ): String {
        val candidates = listOf(
            nameHint,
            profile.name,
            profile.server.serverName
        )
        val slug = candidates
            .map(::slugify)
            .firstOrNull { it.isNotBlank() }
            ?: "imported-profile-" + UUID.randomUUID().toString().substring(0, 8)

        val baseName = "profile.$slug"
        var suffix = 0
        while (true) {
            val fileName = if (suffix == 0) "$baseName.json" else "$baseName-$suffix.json"
            val targetFile = File(importedProfilesDir, fileName)
            if (!targetFile.exists()) {
                return fileName
            }
            val existingPayload = runCatching { targetFile.readText() }.getOrNull()
            if (existingPayload == serializedProfile) {
                return fileName
            }
            suffix++
        }
    }

    private fun slugify(value: String): String {
        val normalized = value.lowercase(Locale.ROOT)
            .replace(Regex("[^a-z0-9]+"), "-")
            .trim('-')
        return normalized.take(48)
    }

    private fun trimYamlValue(value: String): String {
        return value.trim()
            .removeSurrounding("\"")
            .removeSurrounding("'")
    }

    private fun normalizeServerAddress(value: String): String {
        val address = value.trim()
        if (address.isBlank() || isNumericAddress(address)) {
            return address
        }

        if (preferences.forceServerIpMode()) {
            val fallback = bootstrapFallbackAddress
            if (isNumericAddress(fallback)) {
                return fallback
            }
        }

        val lowerAddress = address.lowercase(Locale.ROOT)
        if (lowerAddress == PENDING_ROOT_DOMAIN || lowerAddress.endsWith(".$PENDING_ROOT_DOMAIN")) {
            return bootstrapFallbackAddress.ifBlank { address }
        }

        return address
    }

    private fun isNumericAddress(value: String): Boolean {
        if (':' in value) {
            return true
        }
        return IPV4_REGEX.matches(value)
    }

    private fun loadBootstrapServerAddress(): String {
        return runCatching {
            val payload = AssetPayloadCodec.decodeAssetText(
                context = context,
                assetPath = BOOTSTRAP_ASSET,
                salt = BOOTSTRAP_SALT
            )
            val root = JSONObject(payload)
            root.optString("server_address").trim()
        }.getOrDefault("")
    }

    private fun defaultSeed(shortId: String): String {
        val seedBase = shortId.ifBlank {
            UUID.randomUUID().toString().replace("-", "")
        }
        return "novpn-seed-$seedBase"
    }

    private fun assetProfileId(fileName: String): String = "$ASSET_PREFIX$fileName"

    private fun fileProfileId(fileName: String): String = "$FILE_PREFIX$fileName"

    private data class ProfileEntry(
        val profileId: String,
        val fileName: String,
        val source: ProfileSource
    )

    private enum class ProfileSource {
        ASSET,
        IMPORTED
    }

    companion object {
        private const val BOOTSTRAP_ASSET = "bootstrap/b0.bin"
        private const val BOOTSTRAP_SALT = "bootstrap-server-address-v1"
        private const val DEFAULT_PROFILE_NAME = "Default Reality Profile"
        private const val DEFAULT_IMPORTED_NAME = "Imported Reality Profile"
        private const val DEFAULT_FLOW = "xtls-rprx-vision"
        private const val DEFAULT_FINGERPRINT = "chrome"
        private const val DEFAULT_SOCKS_LISTEN = "127.0.0.1"
        private const val DEFAULT_SOCKS_PORT = 10808
        private const val DEFAULT_HTTP_LISTEN = "127.0.0.1"
        private const val DEFAULT_HTTP_PORT = 10809
        private const val ASSET_PREFIX = "asset:"
        private const val FILE_PREFIX = "file:"
        private const val PENDING_ROOT_DOMAIN = "xower.eu.org"
        private val IPV4_REGEX = Regex("""^\d{1,3}(\.\d{1,3}){3}$""")
    }
}
