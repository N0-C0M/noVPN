package com.novpn.ui

import com.novpn.data.AvailableProfile
import com.novpn.data.AppRoutingMode
import com.novpn.data.PatternMaskingStrategy
import com.novpn.data.TrafficObfuscationStrategy

data class TunnelState(
    val bypassRu: Boolean = true,
    val inviteCode: String = "",
    val appRoutingMode: AppRoutingMode = AppRoutingMode.EXCLUDE_SELECTED,
    val selectedPackages: List<String> = emptyList(),
    val trafficStrategy: TrafficObfuscationStrategy = TrafficObfuscationStrategy.BALANCED,
    val patternStrategy: PatternMaskingStrategy = PatternMaskingStrategy.STEADY,
    val availableProfiles: List<AvailableProfile> = emptyList(),
    val selectedProfileId: String = "",
    val generatedConfigPath: String = "",
    val runtimeRunning: Boolean = false,
    val runtimeStatus: String = "",
    val runtimeDetail: String = "",
    val diagnosticsRunning: Boolean = false,
    val diagnosticsSummary: String = ""
)
