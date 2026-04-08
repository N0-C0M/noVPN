package com.novpn.xray

import android.content.Context
import com.novpn.data.ClientProfile
import com.novpn.vpn.RuntimeLocalProxyConfig
import com.novpn.vpn.RuntimeLocalProxyFactory
import org.json.JSONArray
import org.json.JSONObject
import java.io.File

class AndroidXrayConfigWriter(private val context: Context) {

    fun write(
        profile: ClientProfile,
        bypassRu: Boolean,
        localProxy: RuntimeLocalProxyConfig = RuntimeLocalProxyFactory.create()
    ): File {
        val selectedFingerprint = when (profile.obfuscation.trafficStrategy) {
            com.novpn.data.TrafficObfuscationStrategy.BALANCED -> profile.server.fingerprint
            com.novpn.data.TrafficObfuscationStrategy.CDN_MIMIC -> "chrome"
            com.novpn.data.TrafficObfuscationStrategy.FRAGMENTED -> "safari"
        }
        val selectedSpiderX = when (profile.obfuscation.patternStrategy) {
            com.novpn.data.PatternMaskingStrategy.STEADY -> profile.server.spiderX
            com.novpn.data.PatternMaskingStrategy.PULSE -> profile.obfuscation.patternStrategy.spiderXPath
            com.novpn.data.PatternMaskingStrategy.RANDOMIZED -> {
                val suffix = profile.server.shortId.takeLast(4).ifBlank { "edge" }
                "${profile.obfuscation.patternStrategy.spiderXPath}/$suffix"
            }
        }
        val rules = JSONArray()
            .put(
                JSONObject()
                    .put("type", "field")
                    .put("ip", JSONArray().put("geoip:private"))
                    .put("outboundTag", "direct")
                    .put("ruleTag", "private-direct")
            )

        if (bypassRu) {
            rules.put(
                JSONObject()
                    .put("type", "field")
                    .put(
                        "domain",
                        JSONArray()
                            .put("domain:ru")
                            .put("domain:su")
                            .put("domain:xn--p1ai")
                    )
                    .put("outboundTag", "direct")
                    .put("ruleTag", "ru-domain-direct")
            )
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
                .put("network", "tcp")
                .put("outboundTag", "proxy")
                .put("ruleTag", "default-proxy")
        )

        val document = JSONObject()
            .put("log", JSONObject().put("loglevel", "warning"))
            .put("inbounds", JSONArray()
                .put(
                    JSONObject()
                        .put("tag", "socks-in")
                        .put("listen", localProxy.listenHost)
                        .put("port", localProxy.socksPort)
                        .put("protocol", "socks")
                        .put(
                            "settings",
                            JSONObject()
                                .put("auth", "password")
                                .put("udp", localProxy.udpEnabled)
                                .put(
                                    "accounts",
                                    JSONArray().put(
                                        JSONObject()
                                            .put("user", localProxy.username)
                                            .put("pass", localProxy.password)
                                    )
                                )
                        )
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
                                            .put("fingerprint", selectedFingerprint)
                                            .put("publicKey", profile.server.publicKey)
                                            .put("shortId", profile.server.shortId)
                                            .put("spiderX", selectedSpiderX)
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
}
