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
    val locationLabel: String,
    val isImported: Boolean
)

data class ServerProfile(
    val serverId: String = "",
    val address: String,
    val port: Int,
    val uuid: String,
    val flow: String,
    val serverName: String,
    val fingerprint: String,
    val publicKey: String,
    val shortId: String,
    val locationLabel: String = "",
    val spiderX: String = "/",
    val apiBase: String = ""
)

data class LocalPorts(
    val socksListen: String,
    val socksPort: Int,
    val httpListen: String,
    val httpPort: Int
)

data class ObfuscationProfile(
    val seed: String,
    val trafficStrategy: TrafficObfuscationStrategy = TrafficObfuscationStrategy.BALANCED,
    val patternStrategy: PatternMaskingStrategy = PatternMaskingStrategy.STEADY
)

data class InstalledApp(
    val packageName: String,
    val label: String
)

fun ClientProfile.withObfuscationSeed(seed: String): ClientProfile {
    return copy(obfuscation = obfuscation.copy(seed = seed))
}

fun ClientProfile.withRuntimeStrategies(
    trafficStrategy: TrafficObfuscationStrategy,
    patternStrategy: PatternMaskingStrategy
): ClientProfile {
    return copy(
        obfuscation = obfuscation.copy(
            trafficStrategy = trafficStrategy,
            patternStrategy = patternStrategy
        )
    )
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
            "Импортируйте реальный серверный профиль перед подключением. Не заполнены поля: ${invalidFields.joinToString()}."
        )
    }
}
