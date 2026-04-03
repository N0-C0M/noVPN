package com.novpn.data

import android.content.Context
import org.json.JSONObject

class ProfileRepository(private val context: Context) {

    fun defaultProfileAsset(): String {
        return listProfileAssets().firstOrNull() ?: DEFAULT_ASSET
    }

    fun loadDefaultProfile(): ClientProfile {
        return loadProfile(defaultProfileAsset())
    }

    fun loadProfile(assetName: String): ClientProfile {
        val payload = context.assets.open(assetName).bufferedReader().use { it.readText() }
        val root = JSONObject(payload)
        val server = root.getJSONObject("server")
        val local = root.getJSONObject("local")
        val obfuscation = root.getJSONObject("obfuscation")

        return ClientProfile(
            name = root.getString("name"),
            server = ServerProfile(
                address = server.getString("address"),
                port = server.getInt("port"),
                uuid = server.getString("uuid"),
                flow = server.getString("flow"),
                serverName = server.getString("server_name"),
                fingerprint = server.getString("fingerprint"),
                publicKey = server.getString("public_key"),
                shortId = server.getString("short_id"),
                spiderX = server.optString("spider_x", "/")
            ),
            local = LocalPorts(
                socksListen = local.getString("socks_listen"),
                socksPort = local.getInt("socks_port"),
                httpListen = local.getString("http_listen"),
                httpPort = local.getInt("http_port")
            ),
            obfuscation = ObfuscationProfile(
                seed = obfuscation.getString("seed")
            )
        )
    }

    fun listProfiles(): List<AvailableProfile> {
        return listProfileAssets().map { assetName ->
            val profile = loadProfile(assetName)
            AvailableProfile(
                assetName = assetName,
                name = profile.name,
                address = "${profile.server.address}:${profile.server.port}",
                serverName = profile.server.serverName
            )
        }
    }

    private fun listProfileAssets(): List<String> {
        return context.assets.list("")
            .orEmpty()
            .filter { it.startsWith("profile.") && it.endsWith(".json") }
            .sorted()
    }

    companion object {
        private const val DEFAULT_ASSET = "profile.default.json"
    }
}
