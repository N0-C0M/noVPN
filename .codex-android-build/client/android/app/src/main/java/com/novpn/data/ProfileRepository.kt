package com.novpn.data

import android.content.Context
import android.net.Uri
import org.json.JSONObject
import java.io.File
import java.net.URI
import java.net.URLDecoder
import java.util.Locale
import java.util.UUID

class ProfileRepository(private val context: Context) {
    private val preferences by lazy { ClientPreferences(context) }
    private val importedProfilesDir by lazy {
        File(context.filesDir, "profiles").apply { mkdirs() }
    }
    private val bootstrapFallback by lazy { loadBootstrapConfig() }

    fun defaultProfileId(): String {
        return listProfileEntries().firstOrNull()?.profileId.orEmpty()
    }

    fun loadDefaultProfile(): ClientProfile {
        val profileId = defaultProfileId()
        require(profileId.isNotBlank()) { "Сначала активируйте код или импортируйте профиль сервера." }
        return loadProfile(profileId)
    }

    fun bootstrapServerAddress(): String {
        return bootstrapFallback.serverAddress
    }

    fun bootstrapApiBase(): String {
        return bootstrapFallback.apiBase
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

    fun importProfilePayloads(
        payloads: List<String>,
        nameHint: String = "",
        apiBaseOverride: String = ""
    ): List<AvailableProfile> {
        val normalizedPayloads = payloads
            .flatMap(::splitImportedPayloads)
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
            importProfilePayload(payload, hint, apiBaseOverride)
        }.distinctBy { it.profileId }
    }

    fun importProfilePayload(
        payload: String,
        nameHint: String = "",
        apiBaseOverride: String = ""
    ): AvailableProfile {
        val profile = parseImportedPayload(payload, apiBaseOverride)
        profile.requireRuntimeReady()

        val serializedProfile = serializeProfile(profile)
        val outputFile = listDistinctImportedProfiles()
            .firstOrNull { it.signature == profileSignature(profile) }
            ?.file
            ?: File(importedProfilesDir, buildImportedFileName(nameHint, profile, serializedProfile))
        outputFile.writeText(serializedProfile)
        return AvailableProfile(
            profileId = fileProfileId(outputFile.name),
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
        return listDistinctImportedProfiles().map { record ->
            val profile = record.profile
            AvailableProfile(
                profileId = record.entry.profileId,
                name = profile.name,
                address = "${profile.server.address}:${profile.server.port}",
                serverName = profile.server.serverName,
                locationLabel = profile.server.locationLabel,
                isImported = true
            )
        }
    }

    fun deleteQuotaSyncProfiles() {
        importedProfilesDir.listFiles()
            .orEmpty()
            .filter { it.isFile && it.name.startsWith(QUOTA_SYNC_FILE_PREFIX) && it.name.endsWith(".json") }
            .forEach { file ->
                file.delete()
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
                serverId = server.optString("server_id").trim(),
                address = address,
                port = server.getInt("port"),
                uuid = server.getString("uuid"),
                flow = server.optString("flow", DEFAULT_FLOW),
                serverName = server.getString("server_name"),
                fingerprint = server.optString("fingerprint", DEFAULT_FINGERPRINT),
                publicKey = server.getString("public_key"),
                shortId = shortId,
                locationLabel = locationLabel,
                spiderX = server.optString("spider_x", "/"),
                apiBase = server.optString("api_base").trim()
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

    private fun parseImportedPayload(payload: String, apiBaseOverride: String = ""): ClientProfile {
        val trimmed = payload.trim()
        return when {
            trimmed.startsWith("{") -> withApiBaseOverride(
                parseClientProfileJson(trimmed),
                apiBaseOverride
            )
            trimmed.startsWith("vless://", ignoreCase = true) -> parseVlessProfile(trimmed, apiBaseOverride)
            else -> withApiBaseOverride(
                parseServerClientProfileYaml(trimmed),
                apiBaseOverride
            )
        }
    }

    private fun parseVlessProfile(payload: String, apiBaseOverride: String): ClientProfile {
        val source = URI(payload.trim())
        val address = normalizeServerAddress(source.host.orEmpty())
        val port = source.port
        val uuid = source.userInfo?.trim().orEmpty()
        val fragment = decodeUriComponent(source.rawFragment).ifBlank { source.fragment.orEmpty().trim() }
        val queryValues = parseUriQuery(source.rawQuery)
        val serverName = firstNonBlank(
            queryValues["sni"],
            queryValues["host"],
            fragment,
            address
        )
        val shortId = firstNonBlank(
            queryValues["sid"],
            queryValues["short_id"]
        )
        val publicKey = firstNonBlank(
            queryValues["pbk"],
            queryValues["public_key"]
        )

        require(address.isNotBlank()) { "Imported VLESS link is missing server address." }
        require(port > 0) { "Imported VLESS link is missing server port." }
        require(uuid.isNotBlank()) { "Imported VLESS link is missing client UUID." }
        require(serverName.isNotBlank()) { "Imported VLESS link is missing SNI/server name." }
        require(publicKey.isNotBlank()) { "Imported VLESS link is missing public key." }
        require(shortId.isNotBlank()) { "Imported VLESS link is missing short ID." }

        return ClientProfile(
            name = fragment.ifBlank { "Imported Reality Profile" },
            server = ServerProfile(
                address = address,
                port = port,
                uuid = uuid,
                flow = queryValues["flow"].orEmpty().ifBlank { DEFAULT_FLOW },
                serverName = serverName,
                fingerprint = queryValues["fp"].orEmpty().ifBlank { DEFAULT_FINGERPRINT },
                publicKey = publicKey,
                shortId = shortId,
                locationLabel = ServerLocationCatalog.labelFor(address),
                spiderX = queryValues["spx"].orEmpty().ifBlank { "/" },
                apiBase = apiBaseOverride.trim()
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
                serverId = scalars["server_id"].orEmpty(),
                address = address,
                port = scalars["port"]?.toIntOrNull() ?: 0,
                uuid = scalars["uuid"].orEmpty(),
                flow = scalars["flow"].orEmpty().ifBlank { DEFAULT_FLOW },
                serverName = scalars["server_name"].orEmpty(),
                fingerprint = scalars["fingerprint"].orEmpty().ifBlank { DEFAULT_FINGERPRINT },
                publicKey = scalars["public_key"].orEmpty(),
                shortId = shortId,
                locationLabel = ServerLocationCatalog.labelFor(address),
                spiderX = scalars["spider_x"].orEmpty().ifBlank { "/" },
                apiBase = scalars["api_base"].orEmpty()
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
                    .put("server_id", profile.server.serverId)
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
                    .put("api_base", profile.server.apiBase)
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
        return listDistinctImportedProfiles().map { it.entry }
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

    private fun listDistinctImportedProfiles(): List<ImportedProfileRecord> {
        val rawRecords = importedProfilesDir.listFiles()
            .orEmpty()
            .filter { it.isFile && it.name.startsWith("profile.") && it.name.endsWith(".json") }
            .sortedBy { it.name.lowercase(Locale.ROOT) }
            .mapNotNull(::readImportedProfileRecord)
        if (rawRecords.size < 2) {
            return rawRecords
        }

        val selectedProfileId = preferences.selectedProfileId("")
        var selectedReplacementId: String? = null
        val distinct = rawRecords
            .groupBy { it.signature }
            .values
            .map { group ->
                val canonical = chooseCanonicalProfileRecord(group, selectedProfileId)
                group.filter { it.file.absolutePath != canonical.file.absolutePath }.forEach { duplicate ->
                    if (duplicate.entry.profileId == selectedProfileId) {
                        selectedReplacementId = canonical.entry.profileId
                    }
                    duplicate.file.delete()
                }
                canonical
            }
            .sortedBy { it.file.name.lowercase(Locale.ROOT) }

        if (!selectedReplacementId.isNullOrBlank()) {
            preferences.saveSelectedProfileId(selectedReplacementId.orEmpty())
        }

        return distinct
    }

    private fun readImportedProfileRecord(file: File): ImportedProfileRecord? {
        val payload = runCatching { file.readText() }.getOrNull() ?: return null
        val profile = runCatching { parseClientProfileJson(payload) }.getOrNull() ?: return null
        return ImportedProfileRecord(
            entry = ProfileEntry(
                profileId = fileProfileId(file.name),
                fileName = file.name,
                source = ProfileSource.IMPORTED
            ),
            file = file,
            profile = profile,
            signature = profileSignature(profile)
        )
    }

    private fun chooseCanonicalProfileRecord(
        group: List<ImportedProfileRecord>,
        selectedProfileId: String
    ): ImportedProfileRecord {
        return group.maxWithOrNull(
            compareBy<ImportedProfileRecord>(
                { profileRichnessScore(it.profile) },
                { if (it.entry.profileId == selectedProfileId) 1 else 0 },
                { it.file.lastModified() }
            )
        ) ?: group.first()
    }

    private fun profileRichnessScore(profile: ClientProfile): Int {
        var score = 0
        if (profile.server.serverId.isNotBlank()) {
            score += 50
        }
        if (profile.server.apiBase.isNotBlank()) {
            score += 20
        }
        if (profile.name != DEFAULT_IMPORTED_NAME && profile.name != DEFAULT_PROFILE_NAME) {
            score += 10
        }
        if (!profile.name.endsWith(".jar", ignoreCase = true) &&
            !profile.name.endsWith(".json", ignoreCase = true)
        ) {
            score += 5
        }
        if (profile.server.locationLabel.isNotBlank()) {
            score += 3
        }
        if (profile.server.serverName.isNotBlank()) {
            score += 2
        }
        return score
    }

    private fun profileSignature(profile: ClientProfile): ProfileSignature {
        return ProfileSignature(
            address = profile.server.address.trim().lowercase(Locale.ROOT),
            port = profile.server.port,
            uuid = profile.server.uuid.trim().lowercase(Locale.ROOT),
            serverName = profile.server.serverName.trim().lowercase(Locale.ROOT),
            publicKey = profile.server.publicKey.trim(),
            shortId = profile.server.shortId.trim().lowercase(Locale.ROOT)
        )
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

    private fun splitImportedPayloads(payload: String): List<String> {
        val nonBlankLines = payload.lineSequence()
            .map(String::trim)
            .filter { it.isNotBlank() }
            .toList()
        if (nonBlankLines.isEmpty()) {
            return emptyList()
        }
        if (nonBlankLines.all { it.startsWith("vless://", ignoreCase = true) }) {
            return nonBlankLines
        }
        return listOf(payload.trim())
    }

    private fun withApiBaseOverride(profile: ClientProfile, apiBaseOverride: String): ClientProfile {
        val normalizedApiBase = apiBaseOverride.trim()
        if (normalizedApiBase.isBlank() || profile.server.apiBase.isNotBlank()) {
            return profile
        }
        return profile.copy(
            server = profile.server.copy(
                apiBase = normalizedApiBase
            )
        )
    }

    private fun parseUriQuery(rawQuery: String?): Map<String, String> {
        if (rawQuery.isNullOrBlank()) {
            return emptyMap()
        }
        return rawQuery.split('&')
            .mapNotNull { entry ->
                if (entry.isBlank()) {
                    return@mapNotNull null
                }
                val separatorIndex = entry.indexOf('=')
                val rawKey = if (separatorIndex >= 0) {
                    entry.substring(0, separatorIndex)
                } else {
                    entry
                }
                val rawValue = if (separatorIndex >= 0) {
                    entry.substring(separatorIndex + 1)
                } else {
                    ""
                }
                val key = decodeUriComponent(rawKey).trim()
                if (key.isBlank()) {
                    return@mapNotNull null
                }
                key to decodeUriComponent(rawValue).trim()
            }
            .toMap()
    }

    private fun decodeUriComponent(value: String?): String {
        if (value.isNullOrBlank()) {
            return ""
        }
        return runCatching {
            URLDecoder.decode(value, Charsets.UTF_8.name())
        }.getOrDefault(value)
    }

    private fun firstNonBlank(vararg values: String?): String {
        for (value in values) {
            val normalized = value?.trim().orEmpty()
            if (normalized.isNotBlank()) {
                return normalized
            }
        }
        return ""
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
            val fallback = bootstrapFallback.serverAddress
            if (isNumericAddress(fallback)) {
                return fallback
            }
        }

        val lowerAddress = address.lowercase(Locale.ROOT)
        if (lowerAddress == PENDING_ROOT_DOMAIN || lowerAddress.endsWith(".$PENDING_ROOT_DOMAIN")) {
            return bootstrapFallback.serverAddress.ifBlank { address }
        }

        return address
    }

    private fun isNumericAddress(value: String): Boolean {
        if (':' in value) {
            return true
        }
        return IPV4_REGEX.matches(value)
    }

    private fun loadBootstrapConfig(): BootstrapConfig {
        return runCatching {
            val payload = AssetPayloadCodec.decodeAssetText(
                context = context,
                assetPath = BOOTSTRAP_ASSET,
                salt = BOOTSTRAP_SALT
            )
            val root = JSONObject(payload)
            BootstrapConfig(
                serverAddress = root.optString("server_address").trim(),
                apiBase = root.optString("api_base").trim()
            )
        }.getOrDefault(BootstrapConfig())
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

    private data class ImportedProfileRecord(
        val entry: ProfileEntry,
        val file: File,
        val profile: ClientProfile,
        val signature: ProfileSignature
    )

    private data class ProfileSignature(
        val address: String,
        val port: Int,
        val uuid: String,
        val serverName: String,
        val publicKey: String,
        val shortId: String
    )

    private data class BootstrapConfig(
        val serverAddress: String = "",
        val apiBase: String = ""
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
        private const val QUOTA_SYNC_FILE_PREFIX = "profile.quota-sync-"
        private const val PENDING_ROOT_DOMAIN = "xower.eu.org"
        private val IPV4_REGEX = Regex("""^\d{1,3}(\.\d{1,3}){3}$""")
    }
}
