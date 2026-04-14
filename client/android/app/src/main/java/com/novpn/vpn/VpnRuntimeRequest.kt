package com.novpn.vpn

import com.novpn.data.AppRoutingMode
import com.novpn.data.PatternMaskingStrategy
import com.novpn.data.TrafficObfuscationStrategy

data class VpnRuntimeRequest(
    val profileId: String,
    val bypassRu: Boolean,
    val appRoutingMode: AppRoutingMode,
    val selectedPackages: List<String>,
    val trafficStrategy: TrafficObfuscationStrategy,
    val patternStrategy: PatternMaskingStrategy,
    val autoToggleByScreenState: Boolean,
    val startOnlyForWhitelistApps: Boolean
)
