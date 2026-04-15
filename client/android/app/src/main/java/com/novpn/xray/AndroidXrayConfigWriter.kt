package com.novpn.xray

import android.content.Context
import com.novpn.data.ClientProfile
import com.novpn.data.DeviceIdentityStore
import com.novpn.obfs.SessionObfuscationPlan
import com.novpn.obfs.SessionObfuscationPlanner
import com.novpn.split.LocalRuExclusionCatalogLoader
import com.novpn.vpn.RuntimeLocalProxyConfig
import com.novpn.vpn.RuntimeLocalProxyFactory
import org.json.JSONArray
import org.json.JSONObject
import java.io.File

class AndroidXrayConfigWriter(private val context: Context) {

    fun write(
        profile: ClientProfile,
        bypassRu: Boolean,
        localProxy: RuntimeLocalProxyConfig = RuntimeLocalProxyFactory.create(),
        sessionPlan: SessionObfuscationPlan? = null
    ): File {
        val effectivePlan = sessionPlan ?: SessionObfuscationPlanner.build(
            profile = profile,
            deviceId = DeviceIdentityStore(context).deviceId()
        )
        val localCatalog = LocalRuExclusionCatalogLoader.load(context)
        val rules = JSONArray()
            .put(
                JSONObject()
                    .put("type", "field")
                    .put("ip", JSONArray().put("geoip:private"))
                    .put("outboundTag", "direct")
                    .put("ruleTag", "private-direct")
            )
            .put(
                JSONObject()
                    .put("type", "field")
                    .put(
                        "domain",
                        JSONArray().apply {
                            // Prefer geosite to keep Google service coverage broad (Play Services, APIs, media, etc.).
                            put(GOOGLE_GEOSITE_SELECTOR)
                            GOOGLE_PROXY_DOMAIN_SUFFIXES.forEach { suffix ->
                                put("domain:$suffix")
                            }
                        }
                    )
                    .put("outboundTag", "proxy")
                    .put("ruleTag", "google-proxy")
            )

        if (bypassRu) {
            rules.put(
                JSONObject()
                    .put("type", "field")
                    .put("domain", buildDirectDomainSuffixes("ru", "xn--p1ai", "su"))
                    .put("outboundTag", "direct")
                    .put("ruleTag", "ru-domain-direct")
            )
            if (localCatalog.extraDomains.isNotEmpty()) {
                val extraDomains = JSONArray()
                localCatalog.extraDomains.forEach { domain ->
                    extraDomains.put("domain:$domain")
                }
                rules.put(
                    JSONObject()
                        .put("type", "field")
                        .put("domain", extraDomains)
                        .put("outboundTag", "direct")
                        .put("ruleTag", "catalog-domain-direct")
                )
            }
            rules.put(
                JSONObject()
                    .put("type", "field")
                    .put("ip", JSONArray().put("geoip:ru"))
                    .put("outboundTag", "direct")
                    .put("ruleTag", "ru-ip-direct")
            )
        }

        rules.put(
            JSONObject()
                .put("type", "field")
                .put("network", "tcp,udp")
                .put("outboundTag", "proxy")
                .put("ruleTag", "default-proxy")
        )

        val inboundSettings = JSONObject()
            .put("auth", if (localProxy.username.isBlank() || localProxy.password.isBlank()) "noauth" else "password")
            .put("udp", localProxy.udpEnabled)
        if (localProxy.username.isNotBlank() && localProxy.password.isNotBlank()) {
            inboundSettings.put(
                "accounts",
                JSONArray().put(
                    JSONObject()
                        .put("user", localProxy.username)
                        .put("pass", localProxy.password)
                )
            )
        }

        val document = JSONObject()
            .put("log", JSONObject().put("loglevel", "warning"))
            .put("inbounds", JSONArray()
                .put(
                    JSONObject()
                        .put("tag", "socks-in")
                        .put("listen", localProxy.listenHost)
                        .put("port", localProxy.socksPort)
                        .put("protocol", "socks")
                        .put("settings", inboundSettings)
                        .put(
                            "sniffing",
                            JSONObject()
                                .put("enabled", true)
                                .put("destOverride", JSONArray().put("http").put("tls").put("quic"))
                                .put("routeOnly", true)
                        )
                )
            )
            .put("outbounds", JSONArray()
                .put(
                    JSONObject()
                        .put("tag", "proxy")
                        .put("protocol", "vless")
                        .put(
                            "settings",
                            JSONObject().put(
                                "vnext",
                                JSONArray().put(
                                    JSONObject()
                                        .put("address", profile.server.address)
                                        .put("port", profile.server.port)
                                        .put(
                                            "users",
                                            JSONArray().put(
                                                JSONObject()
                                                    .put("id", profile.server.uuid)
                                                    .put("encryption", "none")
                                                    .put("flow", profile.server.flow)
                                            )
                                        )
                                )
                            )
                        )
                        .put(
                            "streamSettings",
                            JSONObject()
                                .put("network", "tcp")
                                .put("security", "reality")
                                .put(
                                        "realitySettings",
                                        JSONObject()
                                            .put("serverName", profile.server.serverName)
                                            .put("fingerprint", effectivePlan.selectedFingerprint)
                                            .put("publicKey", profile.server.publicKey)
                                            .put("shortId", profile.server.shortId)
                                            .put("spiderX", effectivePlan.selectedSpiderX)
                                    )
                        )
                )
                .put(JSONObject().put("tag", "direct").put("protocol", "freedom"))
                .put(JSONObject().put("tag", "block").put("protocol", "blackhole"))
                .put(JSONObject().put("tag", "dns-out").put("protocol", "dns"))
            )
            .put(
                "routing",
                JSONObject()
                    .put("domainStrategy", "IPIfNonMatch")
                    .put("rules", rules)
            )

        val configDir = File(context.filesDir, "xray").apply { mkdirs() }
        val outputFile = File(configDir, "config.json")
        outputFile.writeText(document.toString(2))
        return outputFile
    }

    companion object {
        private const val GOOGLE_GEOSITE_SELECTOR = "geosite:google"

        private val GOOGLE_PROXY_DOMAIN_SUFFIXES = listOf(
            "1e100.net",
            "app-measurement.com",
            "doubleclick.net",
            "firebaseio.com",
            "google.com",
            "google.ru",
            "googleadservices.com",
            "googleapis.com",
            "googlesyndication.com",
            "googletagmanager.com",
            "googletagservices.com",
            "gstatic.com",
            "gvt1.com",
            "gvt2.com",
            "googlevideo.com",
            "youtube.com",
            "youtu.be",
            "ytimg.com",
            "youtubei.googleapis.com",
            "googleusercontent.com",
            "ggpht.com"
        )

        private fun buildDirectDomainSuffixes(vararg suffixes: String): JSONArray {
            return JSONArray().apply {
                suffixes.forEach { suffix ->
                    val normalized = suffix.trim().lowercase()
                    if (normalized.isBlank() || !DIRECT_DOMAIN_SUFFIX_REGEX.matches(normalized)) {
                        return@forEach
                    }
                    put("domain:$normalized")
                    put("regexp:(^|\\\\.).*\\\\.$normalized$")
                }
            }
        }

        private val DIRECT_DOMAIN_SUFFIX_REGEX = Regex("^[a-z0-9-]+$")
    }
}
