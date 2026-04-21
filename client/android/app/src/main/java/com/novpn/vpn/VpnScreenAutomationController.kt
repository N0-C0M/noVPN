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
        val shouldStopRuntime = synchronized(automationLock) {
            if (!preferences.isScreenOffVpnModeEnabled()) {
                return@synchronized false
            }

            if (preferences.isScreenOffVpnResumePending()) {
                return@synchronized false
            }

            val runtimeStatus = runtimeStatusStore.load()
            if (!runtimeStatus.running) {
                return@synchronized false
            }

            preferences.saveScreenOffVpnResumePending(true)
            runtimeStatusStore.markStopping(
                status = appContext.getString(R.string.runtime_stopping),
                detail = appContext.getString(R.string.runtime_screen_off_pausing_detail)
            )
            true
        }
        if (!shouldStopRuntime) {
            return
        }
        runCatching {
            appContext.startService(NoVpnService.stopIntent(appContext))
        }
    }

    fun resumeIfNeeded() {
        val startRequest = synchronized(automationLock) {
            if (!preferences.isScreenOffVpnModeEnabled()) {
                preferences.saveScreenOffVpnResumePending(false)
                return@synchronized null
            }

            if (!preferences.isScreenOffVpnResumePending()) {
                return@synchronized null
            }

            if (runtimeStatusStore.load().running) {
                preferences.saveScreenOffVpnResumePending(false)
                return@synchronized null
            }

            if (VpnService.prepare(appContext) != null) {
                preferences.saveScreenOffVpnResumePending(false)
                runtimeStatusStore.markFailed(
                    status = appContext.getString(R.string.status_permission_required),
                    detail = appContext.getString(R.string.status_permission_denied_detail)
                )
                return@synchronized null
            }

            val profileId = resolveProfileId()
            if (profileId.isBlank()) {
                preferences.saveScreenOffVpnResumePending(false)
                runtimeStatusStore.markFailed(
                    status = appContext.getString(R.string.runtime_start_failed),
                    detail = appContext.getString(R.string.runtime_profile_incomplete)
                )
                return@synchronized null
            }

            preferences.saveScreenOffVpnResumePending(false)
            runtimeStatusStore.markStarting(
                status = appContext.getString(R.string.runtime_starting),
                detail = appContext.getString(R.string.runtime_screen_off_resuming_detail)
            )
            ResumeRequest(
                profileId = profileId,
                bypassRu = preferences.isBypassRuEnabled(),
                appRoutingMode = preferences.appRoutingMode(),
                selectedPackages = preferences.excludedPackages(),
                trafficStrategy = preferences.trafficObfuscationStrategy(),
                patternStrategy = preferences.patternMaskingStrategy()
            )
        }
        if (startRequest == null) {
            return
        }

        ContextCompat.startForegroundService(
            appContext,
            NoVpnService.startIntent(
                context = appContext,
                profileId = startRequest.profileId,
                bypassRu = startRequest.bypassRu,
                appRoutingMode = startRequest.appRoutingMode,
                selectedPackages = startRequest.selectedPackages,
                trafficStrategy = startRequest.trafficStrategy,
                patternStrategy = startRequest.patternStrategy
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

    private data class ResumeRequest(
        val profileId: String,
        val bypassRu: Boolean,
        val appRoutingMode: com.novpn.data.AppRoutingMode,
        val selectedPackages: List<String>,
        val trafficStrategy: com.novpn.data.TrafficObfuscationStrategy,
        val patternStrategy: com.novpn.data.PatternMaskingStrategy
    )

    companion object {
        private val automationLock = Any()
    }
}
