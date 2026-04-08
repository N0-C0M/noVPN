package com.novpn.ui

import com.novpn.data.AvailableProfile
import com.novpn.data.InstalledApp

data class TunnelState(
    val bypassRu: Boolean = true,
    val inviteCode: String = "",
    val excludedPackages: List<String> = emptyList(),
    val installedApps: List<InstalledApp> = emptyList(),
    val availableProfiles: List<AvailableProfile> = emptyList(),
    val selectedProfileId: String = "",
    val generatedConfigPath: String = "",
    val runtimeRunning: Boolean = false,
    val runtimeStatus: String = "",
    val runtimeDetail: String = "",
    val diagnosticsRunning: Boolean = false,
    val diagnosticsSummary: String = ""
)
