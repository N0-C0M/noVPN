package com.novpn.ui

import com.novpn.data.InstalledApp

data class TunnelState(
    val bypassRu: Boolean = true,
    val excludedPackages: List<String> = emptyList(),
    val installedApps: List<InstalledApp> = emptyList(),
    val generatedConfigPath: String = ""
)
