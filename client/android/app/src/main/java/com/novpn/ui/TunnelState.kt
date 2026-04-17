package com.novpn.ui

import com.novpn.data.AvailableProfile
import com.novpn.data.AppRoutingMode
import com.novpn.data.PatternMaskingStrategy
import com.novpn.data.TrafficObfuscationStrategy

data class TunnelState(
    val bypassRu: Boolean = true,
    val inviteCode: String = "",
    val defaultWhitelistEnabled: Boolean = true,
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
    val trafficUsedBytes: Long = 0L,
    val trafficLimitBytes: Long = 0L,
    val blockedSitesCount: Int = 0,
    val blockedAppsCount: Int = 0,
    val mandatoryNotices: List<String> = emptyList(),
    val diagnosticsRunning: Boolean = false,
    val diagnosticsSummary: String = "",
    val connectionLogs: String = ""
)
