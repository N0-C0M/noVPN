package com.novpn.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import com.novpn.R
import com.novpn.data.ClientPreferences
import com.novpn.data.ProfileRepository
import com.novpn.obfs.ObfuscationSeedStore
import com.novpn.split.InstalledAppsScanner
import com.novpn.vpn.VpnRuntimeRequest
import com.novpn.xray.AndroidXrayConfigWriter
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update

class TunnelViewModel(application: Application) : AndroidViewModel(application) {
    private val appContext = application
    private val profileRepository = ProfileRepository(application)
    private val preferences = ClientPreferences(application)
    private val configWriter = AndroidXrayConfigWriter(application)
    private val appsScanner = InstalledAppsScanner(application)
    private val seedStore = ObfuscationSeedStore(application)

    private val _state = MutableStateFlow(TunnelState())
    val state: StateFlow<TunnelState> = _state

    init {
        refreshStateFromPreferences()
    }

    fun refreshStateFromPreferences() {
        val availableProfiles = profileRepository.listProfiles()
        val defaultAsset = profileRepository.defaultProfileAsset()
        val selectedAsset = preferences.selectedProfileAsset(defaultAsset)
        val normalizedAsset = availableProfiles
            .firstOrNull { it.assetName == selectedAsset }
            ?.assetName
            ?: defaultAsset

        if (normalizedAsset != selectedAsset) {
            preferences.saveSelectedProfileAsset(normalizedAsset)
        }

        _state.update {
            it.copy(
                bypassRu = preferences.isBypassRuEnabled(),
                excludedPackages = preferences.excludedPackages(),
                installedApps = appsScanner.loadLaunchableApps(),
                availableProfiles = availableProfiles,
                selectedProfileAsset = normalizedAsset,
                runtimeStatus = if (it.runtimeRunning) {
                    it.runtimeStatus
                } else {
                    appContext.getString(R.string.service_stopped)
                }
            )
        }
    }

    fun setBypassRu(value: Boolean) {
        preferences.saveBypassRu(value)
        _state.update { it.copy(bypassRu = value) }
    }

    fun setExcludedPackages(value: List<String>) {
        val normalized = value.distinct()
        preferences.saveExcludedPackages(normalized)
        _state.update { it.copy(excludedPackages = normalized) }
    }

    fun selectProfile(assetName: String) {
        preferences.saveSelectedProfileAsset(assetName)
        _state.update { it.copy(selectedProfileAsset = assetName) }
    }

    fun generateConfig() {
        val profile = profileRepository.loadProfile(currentProfileAsset())
        seedStore.loadOrSaveDefault(profile.obfuscation.seed)
        val outputFile = configWriter.write(profile, _state.value.bypassRu)
        _state.update { it.copy(generatedConfigPath = outputFile.absolutePath) }
    }

    fun buildRuntimeRequest(): VpnRuntimeRequest {
        return VpnRuntimeRequest(
            profileAsset = currentProfileAsset(),
            bypassRu = _state.value.bypassRu,
            excludedPackages = _state.value.excludedPackages
        )
    }

    fun markRuntimeStarted(configPath: String) {
        val profileName = selectedProfileName()
        _state.update {
            it.copy(
                runtimeRunning = true,
                runtimeStatus = appContext.getString(R.string.connected_to_profile, profileName),
                generatedConfigPath = configPath
            )
        }
    }

    fun markRuntimeStopped() {
        _state.update {
            it.copy(
                runtimeRunning = false,
                runtimeStatus = appContext.getString(R.string.service_stopped)
            )
        }
    }

    fun selectedProfileName(): String {
        return _state.value.availableProfiles
            .firstOrNull { it.assetName == currentProfileAsset() }
            ?.name
            ?: appContext.getString(R.string.default_server)
    }

    private fun currentProfileAsset(): String {
        val selected = _state.value.selectedProfileAsset
        return if (selected.isNotBlank()) {
            selected
        } else {
            profileRepository.defaultProfileAsset()
        }
    }
}
