package com.novpn.data

data class ClientProfile(
    val name: String,
    val server: ServerProfile,
    val local: LocalPorts,
    val obfuscation: ObfuscationProfile
)

data class AvailableProfile(
    val profileId: String,
    val name: String,
    val address: String,
    val serverName: String,
    val isImported: Boolean
)

data class ServerProfile(
    val address: String,
    val port: Int,
    val uuid: String,
    val flow: String,
    val serverName: String,
    val fingerprint: String,
    val publicKey: String,
    val shortId: String,
    val spiderX: String = "/"
)

data class LocalPorts(
    val socksListen: String,
    val socksPort: Int,
    val httpListen: String,
    val httpPort: Int
)

data class ObfuscationProfile(
    val seed: String
)

data class InstalledApp(
    val packageName: String,
    val label: String
)

fun ClientProfile.withObfuscationSeed(seed: String): ClientProfile {
    return copy(obfuscation = ObfuscationProfile(seed = seed))
}

fun ClientProfile.requireRuntimeReady() {
    val invalidFields = buildList {
        if (server.address.isBlank()) add("address")
        if (server.uuid.isBlank() || server.uuid == "00000000-0000-4000-8000-000000000000") add("uuid")
        if (server.serverName.isBlank()) add("server_name")
        if (server.publicKey.isBlank() || server.publicKey.startsWith("REPLACE_WITH_")) add("public_key")
        if (server.shortId.isBlank() || server.shortId.startsWith("REPLACE_WITH_")) add("short_id")
    }

    if (invalidFields.isNotEmpty()) {
        throw IllegalStateException(
            "Import a real server client-profile before connecting. Missing: ${invalidFields.joinToString()}."
        )
    }
}
