package com.novpn.vpn

data class VpnRuntimeRequest(
    val profileAsset: String,
    val bypassRu: Boolean,
    val excludedPackages: List<String>
)
