package com.novpn.data

data class ClientProfile(
    val name: String,
    val server: ServerProfile,
    val local: LocalPorts,
    val obfuscation: ObfuscationProfile
)

data class AvailableProfile(
    val assetName: String,
    val name: String,
    val address: String,
    val serverName: String
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
