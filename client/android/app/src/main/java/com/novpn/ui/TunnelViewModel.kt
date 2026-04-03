package com.novpn.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import com.novpn.data.ProfileRepository
import com.novpn.obfs.ObfuscationSeedStore
import com.novpn.split.InstalledAppsScanner
import com.novpn.xray.AndroidXrayConfigWriter
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update

class TunnelViewModel(application: Application) : AndroidViewModel(application) {
    private val profileRepository = ProfileRepository(application)
    private val configWriter = AndroidXrayConfigWriter(application)
    private val appsScanner = InstalledAppsScanner(application)
    private val seedStore = ObfuscationSeedStore(application)

    private val _state = MutableStateFlow(TunnelState())
    val state: StateFlow<TunnelState> = _state

    init {
        _state.update { current ->
            current.copy(installedApps = appsScanner.loadLaunchableApps())
        }
    }

    fun setBypassRu(value: Boolean) {
        _state.update { it.copy(bypassRu = value) }
    }

    fun setExcludedPackages(value: List<String>) {
        _state.update { it.copy(excludedPackages = value.distinct()) }
    }

    fun generateConfig() {
        val profile = profileRepository.loadDefaultProfile()
        seedStore.loadOrSaveDefault(profile.obfuscation.seed)
        val outputFile = configWriter.write(profile, _state.value.bypassRu)
        _state.update { it.copy(generatedConfigPath = outputFile.absolutePath) }
    }
}
