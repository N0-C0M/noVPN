package com.novpn.vpn

data class VpnRuntimeRequest(
    val bypassRu: Boolean,
    val excludedPackages: List<String>
)
