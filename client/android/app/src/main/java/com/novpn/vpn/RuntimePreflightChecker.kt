package com.novpn.vpn

import android.content.Context
import com.novpn.R
import com.novpn.data.ProfileRepository
import com.novpn.data.requireRuntimeReady

data class RuntimePreflightReport(
    val isReady: Boolean,
    val headline: String,
    val details: List<String>
) {
    fun requireReady() {
        if (!isReady) {
            throw IllegalStateException(details.joinToString(" "))
        }
    }
}

class RuntimePreflightChecker(private val context: Context) {
    private val profileRepository = ProfileRepository(context)
    private val tun2ProxyReady by lazy {
        runCatching {
            Class.forName("com.novpn.vpn.Tun2ProxyBridge")
            true
        }.getOrDefault(false)
    }

    fun evaluate(profileId: String): RuntimePreflightReport {
        val details = mutableListOf<String>()
        var ready = true

        runCatching {
            profileRepository.loadProfile(profileId).requireRuntimeReady()
        }.onSuccess {
            details += "Profile validated"
        }.onFailure { error ->
            ready = false
            details += error.message ?: "Profile validation failed"
        }

        val xrayAsset = EmbeddedRuntimeAssets.resolveBinaryAssetPathOrNull(context, "xray")
        if (xrayAsset == null) {
            ready = false
            details += "Missing embedded Xray binary for this device ABI"
        } else {
            details += "Xray asset ready: ${EmbeddedRuntimeAssets.assetLabel(xrayAsset)}"
        }

        val obfuscatorAsset = EmbeddedRuntimeAssets.resolveBinaryAssetPathOrNull(context, "obfuscator")
        if (obfuscatorAsset == null) {
            ready = false
            details += "Missing embedded obfuscator binary for this device ABI"
        } else {
            details += "Obfuscator asset ready: ${EmbeddedRuntimeAssets.assetLabel(obfuscatorAsset)}"
        }

        val geoDataReady = EmbeddedRuntimeAssets.assetExists(context, "bin/geoip.dat") &&
            EmbeddedRuntimeAssets.assetExists(context, "bin/geosite.dat")
        if (geoDataReady) {
            details += "GeoIP and GeoSite assets present"
        } else {
            ready = false
            details += "Missing geoip.dat or geosite.dat in embedded assets"
        }

        if (tun2ProxyReady) {
            details += context.getString(R.string.preflight_tun2proxy_ready)
        } else {
            ready = false
            details += "Tun2proxy native bridge is unavailable for this build."
        }

        details += context.getString(R.string.preflight_local_proxy_hardened)
        details += context.getString(R.string.preflight_no_api_surface)
        details += context.getString(R.string.preflight_tunnel_live)

        return RuntimePreflightReport(
            isReady = ready,
            headline = context.getString(
                if (ready) R.string.preflight_ready else R.string.preflight_attention
            ),
            details = details.distinct()
        )
    }
}
