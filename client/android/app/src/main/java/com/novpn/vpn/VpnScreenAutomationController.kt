package com.novpn.vpn

import android.content.Context
import android.net.VpnService
import androidx.core.content.ContextCompat
import com.novpn.R
import com.novpn.data.ClientPreferences
import com.novpn.data.ProfileRepository

class VpnScreenAutomationController(context: Context) {
    private val appContext = context.applicationContext
    private val preferences = ClientPreferences(appContext)
    private val runtimeStatusStore = VpnRuntimeStatusStore(appContext)
    private val profileRepository = ProfileRepository(appContext)

    fun handleScreenOff() {
        if (!preferences.isScreenOffVpnModeEnabled()) {
            return
        }

        val runtimeStatus = runtimeStatusStore.load()
        if (!runtimeStatus.running) {
            return
        }

        preferences.saveScreenOffVpnResumePending(true)
        runtimeStatusStore.markStopping(
            status = appContext.getString(R.string.runtime_stopping),
            detail = appContext.getString(R.string.runtime_screen_off_pausing_detail)
        )
        runCatching {
            appContext.startService(NoVpnService.stopIntent(appContext))
        }
    }

    fun resumeIfNeeded() {
        if (!preferences.isScreenOffVpnModeEnabled()) {
            preferences.saveScreenOffVpnResumePending(false)
            return
        }

        if (!preferences.isScreenOffVpnResumePending()) {
            return
        }

        if (runtimeStatusStore.load().running) {
            preferences.saveScreenOffVpnResumePending(false)
            return
        }

        if (VpnService.prepare(appContext) != null) {
            preferences.saveScreenOffVpnResumePending(false)
            runtimeStatusStore.markFailed(
                status = appContext.getString(R.string.status_permission_required),
                detail = appContext.getString(R.string.status_permission_denied_detail)
            )
            return
        }

        val profileId = resolveProfileId()
        if (profileId.isBlank()) {
            preferences.saveScreenOffVpnResumePending(false)
            runtimeStatusStore.markFailed(
                status = appContext.getString(R.string.runtime_start_failed),
                detail = appContext.getString(R.string.runtime_profile_incomplete)
            )
            return
        }

        preferences.saveScreenOffVpnResumePending(false)
        runtimeStatusStore.markStarting(
            status = appContext.getString(R.string.runtime_starting),
            detail = appContext.getString(R.string.runtime_screen_off_resuming_detail)
        )
        ContextCompat.startForegroundService(
            appContext,
            NoVpnService.startIntent(
                context = appContext,
                profileId = profileId,
                bypassRu = preferences.isBypassRuEnabled(),
                appRoutingMode = preferences.appRoutingMode(),
                selectedPackages = preferences.excludedPackages(),
                trafficStrategy = preferences.trafficObfuscationStrategy(),
                patternStrategy = preferences.patternMaskingStrategy()
            )
        )
    }

    private fun resolveProfileId(): String {
        val defaultProfileId = profileRepository.defaultProfileId()
        if (defaultProfileId.isBlank()) {
            return ""
        }
        return preferences.selectedProfileId(defaultProfileId)
            .ifBlank { defaultProfileId }
    }
}
