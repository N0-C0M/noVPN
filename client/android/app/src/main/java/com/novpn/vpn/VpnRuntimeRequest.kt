package com.novpn.vpn

data class VpnRuntimeRequest(
    val profileId: String,
    val bypassRu: Boolean,
    val excludedPackages: List<String>
)
