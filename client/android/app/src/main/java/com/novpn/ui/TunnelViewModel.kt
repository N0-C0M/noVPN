package com.novpn.ui

import android.app.Application
import android.net.Uri
import androidx.lifecycle.AndroidViewModel
import com.novpn.data.DeviceIdentityStore
import com.novpn.data.InviteRedeemer
import com.novpn.R
import com.novpn.data.AppRoutingMode
import com.novpn.data.ClientPreferences
import com.novpn.data.PatternMaskingStrategy
import com.novpn.data.ProfileRepository
import com.novpn.data.NetworkDiagnosticsRunner
import com.novpn.data.requireRuntimeReady
import com.novpn.data.withRuntimeStrategies
import com.novpn.data.withObfuscationSeed
import com.novpn.obfs.ObfuscationSeedStore
import com.novpn.split.InstalledAppsScanner
import com.novpn.data.TrafficObfuscationStrategy
import com.novpn.vpn.RuntimePreflightChecker
import com.novpn.vpn.RuntimePreflightReport
import com.novpn.vpn.RuntimeLocalProxySession
import com.novpn.vpn.VpnRuntimeStatusStore
import com.novpn.vpn.VpnRuntimeRequest
import com.novpn.xray.AndroidXrayConfigWriter
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.withContext

class TunnelViewModel(application: Application) : AndroidViewModel(application) {
    private val appContext = application
    private val profileRepository = ProfileRepository(application)
    private val preferences = ClientPreferences(application)
    private val configWriter = AndroidXrayConfigWriter(application)
    private val appsScanner = InstalledAppsScanner(application)
    private val seedStore = ObfuscationSeedStore(application)
    private val preflightChecker = RuntimePreflightChecker(application)
    private val deviceIdentityStore = DeviceIdentityStore(application)
    private val inviteRedeemer = InviteRedeemer()
    private val runtimeStatusStore = VpnRuntimeStatusStore(application)
    private val diagnosticsRunner = NetworkDiagnosticsRunner()

    private val _state = MutableStateFlow(TunnelState())
    val state: StateFlow<TunnelState> = _state

    init {
        refreshStateFromPreferences()
    }

    fun refreshStateFromPreferences() {
        val availableProfiles = profileRepository.listProfiles()
        val defaultProfileId = profileRepository.defaultProfileId()
        val selectedProfileId = if (defaultProfileId.isBlank()) {
            ""
        } else {
            preferences.selectedProfileId(defaultProfileId)
        }
        val normalizedProfileId = availableProfiles
            .firstOrNull { it.profileId == selectedProfileId }
            ?.profileId
            ?: defaultProfileId

        if (normalizedProfileId != selectedProfileId) {
            preferences.saveSelectedProfileId(normalizedProfileId)
        }

        val runtimeStatus = runtimeStatusStore.load()

        _state.update {
            it.copy(
                bypassRu = preferences.isBypassRuEnabled(),
                inviteCode = preferences.inviteCode(),
                appRoutingMode = preferences.appRoutingMode(),
                selectedPackages = preferences.excludedPackages(),
                trafficStrategy = preferences.trafficObfuscationStrategy(),
                patternStrategy = preferences.patternMaskingStrategy(),
                installedApps = appsScanner.loadLaunchableApps(),
                availableProfiles = availableProfiles,
                selectedProfileId = normalizedProfileId,
                runtimeRunning = runtimeStatus.running,
                runtimeStatus = runtimeStatus.status.ifBlank {
                    appContext.getString(R.string.service_stopped)
                },
                runtimeDetail = runtimeStatus.detail
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
        _state.update { it.copy(selectedPackages = normalized) }
    }

    fun setAppRoutingMode(value: AppRoutingMode) {
        preferences.saveAppRoutingMode(value)
        _state.update { it.copy(appRoutingMode = value) }
    }

    fun setTrafficStrategy(value: TrafficObfuscationStrategy) {
        preferences.saveTrafficObfuscationStrategy(value)
        _state.update { it.copy(trafficStrategy = value) }
    }

    fun setPatternStrategy(value: PatternMaskingStrategy) {
        preferences.savePatternMaskingStrategy(value)
        _state.update { it.copy(patternStrategy = value) }
    }

    fun setInviteCode(value: String) {
        val normalized = value.trim()
        preferences.saveInviteCode(normalized)
        _state.update { it.copy(inviteCode = normalized) }
    }

    fun selectProfile(profileId: String) {
        preferences.saveSelectedProfileId(profileId)
        refreshStateFromPreferences()
    }

    fun importProfile(uri: Uri) {
        val profile = profileRepository.importProfile(uri)
        preferences.saveSelectedProfileId(profile.profileId)
        refreshStateFromPreferences()
    }

    suspend fun activateInviteCode(): String {
        val inviteCode = _state.value.inviteCode.trim()
        require(inviteCode.isNotBlank()) {
            appContext.getString(R.string.invite_code_missing)
        }

        val payload = inviteRedeemer.redeem(
            serverAddress = profileRepository.bootstrapServerAddress(),
            inviteCode = inviteCode,
            deviceId = deviceIdentityStore.deviceId(),
            deviceName = deviceIdentityStore.deviceName()
        )

        val importedProfile = profileRepository.importProfilePayload(
            payload = payload,
            nameHint = "invite-$inviteCode"
        )
        preferences.saveSelectedProfileId(importedProfile.profileId)
        preferences.saveInviteCode(inviteCode)
        refreshStateFromPreferences()
        return importedProfile.name
    }

    fun generateConfig() {
        runtimePreflight(requireCurrentProfileId()).requireReady()
        val profile = profileRepository.loadProfile(requireCurrentProfileId())
        profile.requireRuntimeReady()
        val effectiveProfile = profile.withObfuscationSeed(
            seedStore.loadOrSaveDefault(profile.obfuscation.seed)
        ).withRuntimeStrategies(_state.value.trafficStrategy, _state.value.patternStrategy)
        val outputFile = configWriter.write(effectiveProfile, _state.value.bypassRu)
        _state.update { it.copy(generatedConfigPath = outputFile.absolutePath) }
    }

    fun buildRuntimeRequest(): VpnRuntimeRequest {
        return VpnRuntimeRequest(
            profileId = requireCurrentProfileId(),
            bypassRu = _state.value.bypassRu,
            appRoutingMode = _state.value.appRoutingMode,
            selectedPackages = _state.value.selectedPackages,
            trafficStrategy = _state.value.trafficStrategy,
            patternStrategy = _state.value.patternStrategy
        )
    }

    fun markRuntimeStarted(configPath: String) {
        _state.update {
            it.copy(
                generatedConfigPath = configPath
            )
        }
    }

    fun markRuntimeStopped() {
        _state.update {
            it.copy(
                runtimeRunning = false,
                runtimeStatus = appContext.getString(R.string.service_stopped),
                runtimeDetail = ""
            )
        }
    }

    fun markDiagnosticsStarted() {
        _state.update {
            it.copy(
                diagnosticsRunning = true,
                diagnosticsSummary = appContext.getString(R.string.diagnostics_running)
            )
        }
    }

    fun markDiagnosticsFailed(message: String) {
        _state.update {
            it.copy(
                diagnosticsRunning = false,
                diagnosticsSummary = message
            )
        }
    }

    suspend fun runNetworkDiagnostics(): String {
        val localProxy = requireNotNull(RuntimeLocalProxySession.current()) {
            appContext.getString(R.string.diagnostics_runtime_unavailable)
        }
        val profile = profileRepository.loadProfile(requireCurrentProfileId())
        val result = withContext(Dispatchers.IO) {
            diagnosticsRunner.run(profile, localProxy)
        }
        _state.update {
            it.copy(
                diagnosticsRunning = false,
                diagnosticsSummary = result.summary
            )
        }
        return result.summary
    }

    fun selectedProfileName(): String {
        return _state.value.availableProfiles
            .firstOrNull { it.profileId == currentProfileId() }
            ?.name
            ?: appContext.getString(R.string.default_server)
    }

    fun runtimePreflight(): RuntimePreflightReport {
        return runtimePreflight(requireCurrentProfileId())
    }

    private fun currentProfileId(): String {
        val selected = _state.value.selectedProfileId
        return if (selected.isNotBlank()) {
            selected
        } else {
            profileRepository.defaultProfileId()
        }
    }

    private fun requireCurrentProfileId(): String {
        val profileId = currentProfileId()
        require(profileId.isNotBlank()) {
            appContext.getString(R.string.runtime_profile_incomplete)
        }
        return profileId
    }

    private fun runtimePreflight(profileId: String): RuntimePreflightReport {
        return preflightChecker.evaluate(profileId)
    }
}
