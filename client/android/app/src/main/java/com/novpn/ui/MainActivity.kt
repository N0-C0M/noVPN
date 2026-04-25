package com.novpn.ui

import android.app.Activity
import android.animation.ValueAnimator
import android.content.Intent
import android.content.res.ColorStateList
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.net.VpnService
import android.os.Bundle
import android.text.InputType
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import com.novpn.R
import com.novpn.data.AvailableProfile
import com.novpn.data.CodeRedeemKind
import com.novpn.data.ClientPreferences
import com.novpn.vpn.NoVpnService
import com.novpn.vpn.VpnRuntimeStatusStore
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {
    private val viewModel by viewModels<TunnelViewModel>()
    private val preferences by lazy { ClientPreferences(this) }

    private lateinit var headerLabel: TextView
    private lateinit var headerServer: TextView
    private lateinit var statusTitle: TextView
    private lateinit var statusDetail: TextView
    private lateinit var powerButton: Button
    private lateinit var inviteCodeInput: EditText
    private lateinit var activateCodeButton: Button
    private lateinit var disconnectCodeButton: Button
    private lateinit var diagnosticsButton: Button
    private lateinit var diagnosticsDetail: TextView
    private lateinit var serverStrip: LinearLayout
    private lateinit var startupOverlay: View
    private lateinit var startupCard: LinearLayout
    private lateinit var startupTitleLabel: TextView
    private lateinit var startupDetailLabel: TextView
    private lateinit var startupProgressBar: ProgressBar
    private lateinit var startupProgressPercent: TextView
    private var powerPulseAnimator: ValueAnimator? = null
    private var lastPowerVisualState = PowerVisualState.IDLE
    private var startupDelayElapsed = false
    private var startupOverlayDismissed = false
    private var firstRenderCompleted = false
    private var startupWarmupReady = false
    private var runtimeStatusSyncJob: Job? = null

    private val settingsLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) {
        viewModel.refreshStateFromPreferences()
        renderState(viewModel.state.value)
    }

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            startVpnRuntime()
        } else {
            statusTitle.text = getString(R.string.status_permission_required)
            statusDetail.text = getString(R.string.status_permission_denied_detail)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(buildContentView())
        viewModel.refreshStateFromPreferences()
        renderState(viewModel.state.value)
        lifecycleScope.launch {
            viewModel.refreshGatewayPolicy()
            renderState(viewModel.state.value)
        }
        lifecycleScope.launch {
            delay(650)
            startupDelayElapsed = true
            maybeDismissStartupOverlay()
        }
        lifecycleScope.launch {
            viewModel.runStartupWarmup(::updateStartupProgress)
            startupWarmupReady = true
            maybeDismissStartupOverlay()
        }
    }

    override fun onResume() {
        super.onResume()
        viewModel.refreshStateFromPreferences()
        renderState(viewModel.state.value)
        lifecycleScope.launch {
            viewModel.refreshGatewayPolicy()
            renderState(viewModel.state.value)
        }
    }

    override fun onStart() {
        super.onStart()
        startRuntimeStatusSync()
    }

    override fun onStop() {
        stopRuntimeStatusSync()
        super.onStop()
    }

    override fun onDestroy() {
        stopRuntimeStatusSync()
        super.onDestroy()
    }

    private fun buildContentView(): View {
        val root = FrameLayout(this)

        val scroll = ScrollView(this).apply {
            background = GradientDrawable(
                GradientDrawable.Orientation.TOP_BOTTOM,
                intArrayOf(Color.parseColor("#02040A"), Color.parseColor("#060B12"))
            )
        }

        val content = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(20), dp(22), dp(20), dp(28))
        }
        scroll.addView(
            content,
            LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            )
        )

        content.addView(buildHeader())
        content.addView(buildHeroSection())
        content.addView(buildInviteSection())
        content.addView(buildServerSection())
        root.addView(
            scroll,
            FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.MATCH_PARENT
            )
        )

        startupOverlay = buildStartupOverlay()
        root.addView(
            startupOverlay,
            FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.MATCH_PARENT
            )
        )
        return root
    }

    private fun buildStartupOverlay(): View {
        return FrameLayout(this).apply {
            alpha = 1f
            background = GradientDrawable(
                GradientDrawable.Orientation.TOP_BOTTOM,
                intArrayOf(Color.parseColor("#04070D"), Color.parseColor("#09131E"))
            )

            startupCard = LinearLayout(this@MainActivity).apply {
                orientation = LinearLayout.VERTICAL
                gravity = Gravity.CENTER
                background = roundedDrawable("#0A1018", "#203040", 36f, 2)
                setPadding(dp(24), dp(28), dp(24), dp(28))
                elevation = dp(10).toFloat()
                startupTitleLabel = label(getString(R.string.startup_loading_title), 24f, "#F3F6FB", true).apply {
                    gravity = Gravity.CENTER
                }
                addView(startupTitleLabel)

                startupDetailLabel = label(getString(R.string.startup_loading_detail), 13f, "#7F90A5", false).apply {
                    gravity = Gravity.CENTER
                    setPadding(0, dp(10), 0, 0)
                }
                addView(startupDetailLabel)

                val progressRow = LinearLayout(this@MainActivity).apply {
                    orientation = LinearLayout.HORIZONTAL
                    gravity = Gravity.CENTER_VERTICAL
                    setPadding(0, dp(18), 0, 0)
                }

                startupProgressBar = ProgressBar(
                    this@MainActivity,
                    null,
                    android.R.attr.progressBarStyleHorizontal
                ).apply {
                    max = 100
                    progress = 6
                    progressTintList = ColorStateList.valueOf(Color.parseColor("#5FD4A6"))
                    progressBackgroundTintList = ColorStateList.valueOf(Color.parseColor("#213244"))
                    layoutParams = LinearLayout.LayoutParams(
                        0,
                        dp(8),
                        1f
                    ).apply {
                        marginEnd = dp(12)
                    }
                }
                progressRow.addView(startupProgressBar)

                startupProgressPercent = label("6%", 12f, "#A8BAD0", true)
                progressRow.addView(startupProgressPercent)
                addView(progressRow)
            }

            addView(
                startupCard,
                FrameLayout.LayoutParams(
                    FrameLayout.LayoutParams.MATCH_PARENT,
                    FrameLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    gravity = Gravity.CENTER
                    marginStart = dp(22)
                    marginEnd = dp(22)
                }
            )
        }
    }

    private fun buildHeader(): View {
        val row = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
        }

        val titleBlock = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
        }

        headerLabel = label(getString(R.string.header_profile_label), 12f, "#6F8096", false).apply {
            letterSpacing = 0.08f
        }
        headerServer = label(getString(R.string.header_no_server), 22f, "#F3F6FB", true).apply {
            setPadding(0, dp(6), 0, 0)
        }

        titleBlock.addView(headerLabel)
        titleBlock.addView(headerServer)

        val actionRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.END
        }

        diagnosticsButton = Button(this).apply {
            text = getString(R.string.run_diagnostics)
            isAllCaps = false
            setTextColor(Color.parseColor("#F3F6FB"))
            textSize = 12f
            typeface = Typeface.DEFAULT_BOLD
            background = roundedDrawable("#0E1520", "#243244", 22f, 2)
            setPadding(dp(18), dp(12), dp(18), dp(12))
            setOnClickListener { runDiagnostics() }
        }

        val settingsButton = Button(this).apply {
            text = getString(R.string.settings)
            isAllCaps = false
            setTextColor(Color.parseColor("#F3F6FB"))
            textSize = 12f
            typeface = Typeface.DEFAULT_BOLD
            background = roundedDrawable("#0E1520", "#243244", 22f, 2)
            setPadding(dp(18), dp(12), dp(18), dp(12))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.WRAP_CONTENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                marginStart = dp(10)
            }
            setOnClickListener {
                openSettings()
            }
        }

        actionRow.addView(diagnosticsButton)
        actionRow.addView(settingsButton)

        row.addView(titleBlock)
        row.addView(actionRow)
        return row
    }

    private fun buildHeroSection(): View {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            background = roundedDrawable("#0A1018", "#182432", 38f, 2)
            setPadding(dp(18), dp(28), dp(18), dp(30))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(24)
            }

            powerButton = Button(this@MainActivity).apply {
                isAllCaps = false
                textSize = 20f
                typeface = Typeface.DEFAULT_BOLD
                setTextColor(Color.parseColor("#F3F6FB"))
                gravity = Gravity.CENTER
                background = powerDrawable(false)
                layoutParams = LinearLayout.LayoutParams(dp(214), dp(214)).apply {
                    topMargin = dp(8)
                    bottomMargin = dp(24)
                }
                setOnClickListener { toggleRuntime() }
            }
            addView(powerButton)

            statusTitle = label(getString(R.string.status_ready), 25f, "#F3F6FB", true).apply {
                gravity = Gravity.CENTER
            }
            addView(statusTitle)

            statusDetail = label("", 13f, "#8B9BAE", false).apply {
                gravity = Gravity.CENTER
                setPadding(dp(10), dp(12), dp(10), 0)
            }
            addView(statusDetail)

        }
    }

    private fun buildServerSection(): View {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#0A1018", "#182432", 38f, 2)
            setPadding(dp(18), dp(18), dp(18), dp(18))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(18)
            }

            addView(label(getString(R.string.server_section_title), 16f, "#F3F6FB", true))
            addView(
                label(getString(R.string.server_section_hint), 12f, "#7B8DA3", false).apply {
                    setPadding(0, dp(6), 0, dp(14))
                }
            )

            serverStrip = LinearLayout(this@MainActivity).apply {
                orientation = LinearLayout.VERTICAL
            }
            addView(serverStrip)
        }
    }

    private fun buildDiagnosticsSection(): View {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#0A1018", "#182432", 38f, 2)
            setPadding(dp(18), dp(18), dp(18), dp(18))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(18)
            }

            addView(label(getString(R.string.diagnostics_title), 16f, "#F3F6FB", true))
            addView(
                label(getString(R.string.diagnostics_hint), 12f, "#7B8DA3", false).apply {
                    setPadding(0, dp(6), 0, dp(14))
                }
            )

            diagnosticsButton = Button(this@MainActivity).apply {
                text = getString(R.string.run_diagnostics)
                isAllCaps = false
                setTextColor(Color.parseColor("#F3F6FB"))
                textSize = 13f
                typeface = Typeface.DEFAULT_BOLD
                background = roundedDrawable("#0E1520", "#243244", 22f, 2)
                setPadding(dp(18), dp(12), dp(18), dp(12))
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                )
                setOnClickListener { runDiagnostics() }
            }
            addView(diagnosticsButton)

            diagnosticsDetail = label(getString(R.string.diagnostics_idle), 12f, "#7B8DA3", false).apply {
                setPadding(0, dp(12), 0, 0)
            }
            addView(diagnosticsDetail)
        }
    }

    private fun buildInviteSection(): View {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#0A1018", "#182432", 38f, 2)
            setPadding(dp(18), dp(18), dp(18), dp(18))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(18)
            }

            addView(label(getString(R.string.invite_code_title), 16f, "#F3F6FB", true))
            addView(
                label(getString(R.string.invite_code_hint), 12f, "#7B8DA3", false).apply {
                    setPadding(0, dp(6), 0, dp(14))
                }
            )

            inviteCodeInput = EditText(this@MainActivity).apply {
                hint = getString(R.string.invite_code_placeholder)
                setHintTextColor(Color.parseColor("#607287"))
                setTextColor(Color.parseColor("#F3F6FB"))
                textSize = 15f
                inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
                background = roundedDrawable("#0E1520", "#243244", 20f, 2)
                setPadding(dp(16), dp(14), dp(16), dp(14))
            }
            addView(
                inviteCodeInput,
                LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                )
            )

            val buttonRow = LinearLayout(this@MainActivity).apply {
                orientation = LinearLayout.HORIZONTAL
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    topMargin = dp(12)
                }
            }

            activateCodeButton = Button(this@MainActivity).apply {
                text = getString(R.string.activate_invite_code)
                isAllCaps = false
                setTextColor(Color.parseColor("#F3F6FB"))
                textSize = 12.5f
                typeface = Typeface.DEFAULT_BOLD
                background = roundedDrawable("#0E1520", "#243244", 22f, 2)
                setPadding(dp(14), dp(12), dp(14), dp(12))
                layoutParams = LinearLayout.LayoutParams(
                    0,
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    1f
                ).apply {
                    marginEnd = dp(6)
                }
                setOnClickListener { activateInviteCode() }
            }
            buttonRow.addView(activateCodeButton)

            disconnectCodeButton = Button(this@MainActivity).apply {
                text = "Disconnect device"
                isAllCaps = false
                setTextColor(Color.parseColor("#F3F6FB"))
                textSize = 12.5f
                typeface = Typeface.DEFAULT_BOLD
                background = roundedDrawable("#0E1520", "#243244", 22f, 2)
                setPadding(dp(14), dp(12), dp(14), dp(12))
                layoutParams = LinearLayout.LayoutParams(
                    0,
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    1f
                ).apply {
                    marginStart = dp(6)
                }
                setOnClickListener { disconnectDeviceFromCode() }
            }
            buttonRow.addView(disconnectCodeButton)

            addView(buttonRow)
        }
    }

    private fun renderState(state: TunnelState) {
        val selected = state.availableProfiles.firstOrNull { it.profileId == state.selectedProfileId }
            ?: state.availableProfiles.firstOrNull()

        headerServer.text = formatHeaderServerName(selected?.name)
        val locationLine = getString(
            R.string.server_location_format,
            selected?.locationLabel?.ifBlank { getString(R.string.server_location_unknown) }
                ?: getString(R.string.server_location_unknown)
        )
        val trafficRemainingBytes = state.trafficLimitBytes.takeIf { it > 0L }?.let { limit ->
            val used = state.trafficUsedBytes.coerceAtLeast(0L)
            (limit - used).coerceAtLeast(0L)
        }
        val trafficRemainingLine = if (state.trafficLimitBytes > 0L) {
            "Traffic left: ${formatBytes(trafficRemainingBytes ?: 0L)}"
        } else {
            "Traffic left: ∞"
        }

        val isStarting = state.runtimeStatus == getString(R.string.runtime_starting)
        val isStopping = state.runtimeStatus == VpnRuntimeStatusStore.STATUS_STOPPING
        val showRuntimeDetail = !state.runtimeRunning &&
            state.runtimeDetail.isNotBlank() &&
            state.runtimeStatus.isNotBlank() &&
            state.runtimeStatus != getString(R.string.service_stopped)
        val statusTitleText: String

        if (state.runtimeRunning) {
            powerButton.text = getString(R.string.disconnect)
            powerButton.background = powerDrawable(true)
            statusTitleText = if (trafficRemainingBytes != null && trafficRemainingBytes <= 0L) {
                "Traffic limit reached"
            } else {
                getString(R.string.status_connected)
            }
        } else if (isStopping) {
            powerButton.text = STOPPING_BUTTON_TEXT
            powerButton.background = powerDrawable(true)
            statusTitleText = state.runtimeStatus
        } else if (isStarting) {
            powerButton.text = STARTING_BUTTON_TEXT
            powerButton.background = powerDrawable(false)
            statusTitleText = state.runtimeStatus
        } else {
            powerButton.text = getString(R.string.connect)
            powerButton.background = powerDrawable(false)
            statusTitleText = if (state.runtimeStatus.isBlank() || state.runtimeStatus == getString(R.string.service_stopped)) {
                getString(R.string.status_ready)
            } else {
                state.runtimeStatus
            }
        }

        val statusDetailText = buildString {
            appendLine(locationLine)
            append(trafficRemainingLine)
            if (showRuntimeDetail) {
                appendLine()
                appendLine()
                append(state.runtimeDetail.trim())
            }
        }
        powerButton.isEnabled = !(isStarting || isStopping)
        updateTextWithFade(statusTitle, statusTitleText)
        updateTextWithFade(statusDetail, statusDetailText)
        animatePowerState(state)

        if (::inviteCodeInput.isInitialized) {
            val currentCode = inviteCodeInput.text?.toString().orEmpty()
            if (currentCode != state.inviteCode) {
                inviteCodeInput.setText(state.inviteCode)
                inviteCodeInput.setSelection(inviteCodeInput.text?.length ?: 0)
            }
        }

        if (::diagnosticsButton.isInitialized) {
            diagnosticsButton.isEnabled = !state.diagnosticsRunning
            diagnosticsButton.text = if (state.diagnosticsRunning) {
                getString(R.string.diagnostics_running_button)
            } else {
                getString(R.string.run_diagnostics)
            }
        }
        if (::disconnectCodeButton.isInitialized) {
            disconnectCodeButton.isEnabled = selected?.isImported == true
        }

        rebuildServerCards(state.availableProfiles, state.selectedProfileId)
        firstRenderCompleted = true
        maybeDismissStartupOverlay()
    }

    private fun rebuildServerCards(profiles: List<AvailableProfile>, selectedProfileId: String) {
        serverStrip.removeAllViews()

        if (profiles.isEmpty()) {
            serverStrip.addView(
                label(getString(R.string.no_profiles_found), 12f, "#7B8DA3", false)
            )
            return
        }

        profiles.forEachIndexed { index, profile ->
            val selected = profile.profileId == selectedProfileId
            val card = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                background = roundedDrawable(
                    if (selected) "#122232" else "#0D141D",
                    if (selected) "#3E678C" else "#1A2634",
                    24f,
                    2
                )
                setPadding(dp(14), dp(14), dp(14), dp(14))
                isClickable = true
                isFocusable = true
                setOnClickListener {
                    viewModel.selectProfile(profile.profileId)
                    renderState(viewModel.state.value)
                    lifecycleScope.launch {
                        viewModel.refreshGatewayPolicy()
                        renderState(viewModel.state.value)
                    }
                }
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    bottomMargin = if (index == profiles.lastIndex) 0 else dp(10)
                }
            }

            val textBlock = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                layoutParams = LinearLayout.LayoutParams(
                    0,
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    1f
                )
            }

            textBlock.addView(label(profile.name, 14f, "#F3F6FB", true))
            textBlock.addView(
                label(
                    buildString {
                        append(profile.locationLabel.ifBlank { getString(R.string.server_location_unknown) })
                        append("  |  ")
                        append(
                            getString(
                                if (profile.isImported) R.string.profile_source_imported else R.string.profile_source_bundled
                            )
                        )
                    },
                    11.5f,
                    if (selected) "#D8E5F3" else "#93A3B7",
                    false
                ).apply {
                    setPadding(0, dp(6), 0, 0)
                }
            )
            card.addView(textBlock)

            card.addView(
                label(
                    if (selected) "Selected" else "Switch",
                    11.5f,
                    if (selected) "#A7E8C7" else "#9FB3C8",
                    true
                ).apply {
                    background = roundedDrawable(
                        if (selected) "#123326" else "#101A26",
                        if (selected) "#3A8C68" else "#284059",
                        18f,
                        1
                    )
                    setPadding(dp(12), dp(7), dp(12), dp(7))
                }
            )

            serverStrip.addView(card)
        }
    }

    private fun maybeDismissStartupOverlay() {
        if (!firstRenderCompleted || !startupDelayElapsed || !startupWarmupReady || startupOverlayDismissed) {
            return
        }
        startupOverlayDismissed = true
        startupCard.animate()
            .alpha(0f)
            .translationY(-dp(10f))
            .scaleX(0.98f)
            .scaleY(0.98f)
            .setDuration(220)
            .start()
        startupOverlay.animate()
            .alpha(0f)
            .setDuration(260)
            .withEndAction {
                startupOverlay.visibility = View.GONE
            }
            .start()
    }

    private fun updateStartupProgress(update: StartupWarmupUpdate) {
        if (!::startupProgressBar.isInitialized) {
            return
        }
        startupTitleLabel.text = update.title
        startupDetailLabel.text = buildString {
            append(update.detail)
            if (preferences.isInitialRuAppAuditPending()) {
                append("\n\n")
                append(getString(R.string.startup_ru_audit_mandatory))
            }
        }
        startupProgressBar.progress = update.progressPercent.coerceIn(0, 100)
        startupProgressPercent.text = "${update.progressPercent.coerceIn(0, 100)}%"
    }

    private fun startRuntimeStatusSync() {
        stopRuntimeStatusSync()
        runtimeStatusSyncJob = lifecycleScope.launch {
            var previousSnapshot = RuntimeStatusSnapshot.from(viewModel.state.value)
            while (isActive) {
                delay(RUNTIME_STATUS_SYNC_INTERVAL_MS)
                viewModel.refreshStateFromPreferences()
                val currentState = viewModel.state.value
                val currentSnapshot = RuntimeStatusSnapshot.from(currentState)
                if (currentSnapshot != previousSnapshot) {
                    renderState(currentState)
                    previousSnapshot = currentSnapshot
                }
            }
        }
    }

    private fun stopRuntimeStatusSync() {
        runtimeStatusSyncJob?.cancel()
        runtimeStatusSyncJob = null
    }

    private fun toggleRuntime() {
        if (viewModel.state.value.runtimeStatus == getString(R.string.runtime_starting) ||
            viewModel.state.value.runtimeStatus == VpnRuntimeStatusStore.STATUS_STOPPING
        ) {
            return
        }
        if (viewModel.state.value.runtimeRunning) {
            animateDisconnectTransition()
            viewModel.markRuntimeStopping()
            renderState(viewModel.state.value)
            startService(NoVpnService.stopIntent(this))
            return
        }

        beginVpnStartFlow()
    }

    private fun beginVpnStartFlow() {
        if (viewModel.state.value.runtimeStatus == VpnRuntimeStatusStore.STATUS_STOPPING) {
            Toast.makeText(this, VpnRuntimeStatusStore.DETAIL_STOPPING, Toast.LENGTH_SHORT).show()
            return
        }
        val prepareIntent = VpnService.prepare(this)
        if (prepareIntent != null) {
            vpnPermissionLauncher.launch(prepareIntent)
        } else {
            startVpnRuntime()
        }
    }

    private fun openSettings() {
        val state = viewModel.state.value
        val selected = state.availableProfiles.firstOrNull { it.profileId == state.selectedProfileId }
            ?: state.availableProfiles.firstOrNull()
        val appsLine = if (state.defaultWhitelistEnabled) {
            "Default whitelist: ${state.selectedPackages.size} apps"
        } else {
            when (state.appRoutingMode) {
                com.novpn.data.AppRoutingMode.EXCLUDE_SELECTED ->
                    getString(R.string.apps_selection_summary_exclude, state.selectedPackages.size)
                com.novpn.data.AppRoutingMode.ONLY_SELECTED ->
                    getString(R.string.apps_selection_summary_include, state.selectedPackages.size)
            }
        }
        val strategyLine = getString(
            R.string.strategy_summary_format,
            trafficStrategyLabel(state.trafficStrategy),
            patternStrategyLabel(state.patternStrategy)
        )
        val noticesLine = if (state.mandatoryNotices.isEmpty()) {
            ""
        } else {
            state.mandatoryNotices.joinToString(" | ")
        }
        settingsLauncher.launch(
            SettingsActivity.createIntent(
                context = this,
                serverName = selected?.name ?: getString(R.string.header_no_server),
                routingSummary = if (state.bypassRu) {
                    getString(R.string.mode_bypass_ru)
                } else {
                    getString(R.string.mode_full_tunnel)
                },
                appsSummary = appsLine,
                strategySummary = strategyLine,
                blocklistSummary = "Server blocklist: sites ${state.blockedSitesCount}, apps ${state.blockedAppsCount}",
                noticesSummary = noticesLine
            )
        )
    }

    private fun activateInviteCode() {
        val code = inviteCodeInput.text?.toString().orEmpty()
        viewModel.setInviteCode(code)
        activateCodeButton.isEnabled = false
        statusTitle.text = getString(R.string.invite_code_activating)
        statusDetail.text = getString(R.string.invite_code_loading_detail)

        lifecycleScope.launch {
            runCatching {
                viewModel.activateInviteCode()
            }.onSuccess { redeemResult ->
                renderState(viewModel.state.value)
                lifecycleScope.launch {
                    viewModel.refreshGatewayPolicy()
                    renderState(viewModel.state.value)
                }
                when (redeemResult.kind) {
                    CodeRedeemKind.INVITE -> {
                        val profileName = redeemResult.profileName.ifBlank { viewModel.selectedProfileName() }
                        Toast.makeText(
                            this@MainActivity,
                            getString(R.string.invite_code_activated, profileName),
                            Toast.LENGTH_SHORT
                        ).show()
                        beginVpnStartFlow()
                    }
                    CodeRedeemKind.PROMO -> {
                        val trialIssued = redeemResult.activationMode == "trial" ||
                            redeemResult.profilePayloads.isNotEmpty() ||
                            redeemResult.profilePayload.isNotBlank()
                        Toast.makeText(
                            this@MainActivity,
                            if (trialIssued) {
                                "Promo activated: trial profile issued (${formatBytes(redeemResult.bonusBytes)})."
                            } else {
                                "Promo activated: +${formatBytes(redeemResult.bonusBytes)}"
                            },
                            Toast.LENGTH_SHORT
                        ).show()
                        if (trialIssued) {
                            beginVpnStartFlow()
                        }
                    }
                }
            }.onFailure { error ->
                renderState(viewModel.state.value)
                statusTitle.text = getString(R.string.invite_code_activation_failed)
                statusDetail.text = error.message ?: getString(R.string.import_profile_failed)
                Toast.makeText(
                    this@MainActivity,
                    error.message ?: getString(R.string.invite_code_activation_failed),
                    Toast.LENGTH_LONG
                ).show()
            }
            activateCodeButton.isEnabled = true
        }
    }

    private fun disconnectDeviceFromCode() {
        disconnectCodeButton.isEnabled = false
        lifecycleScope.launch {
            runCatching {
                if (viewModel.state.value.runtimeRunning) {
                    viewModel.markRuntimeStopping()
                    renderState(viewModel.state.value)
                    startService(NoVpnService.stopIntent(this@MainActivity))
                }
                viewModel.disconnectCurrentDevice()
            }.onSuccess {
                renderState(viewModel.state.value)
                Toast.makeText(
                    this@MainActivity,
                    "Device disconnected from the code.",
                    Toast.LENGTH_SHORT
                ).show()
            }.onFailure { error ->
                renderState(viewModel.state.value)
                Toast.makeText(
                    this@MainActivity,
                    error.message ?: "Failed to disconnect device.",
                    Toast.LENGTH_LONG
                ).show()
            }
            disconnectCodeButton.isEnabled = true
        }
    }

    private fun runDiagnostics() {
        viewModel.markDiagnosticsStarted()
        renderState(viewModel.state.value)

        lifecycleScope.launch {
            runCatching {
                viewModel.runNetworkDiagnostics()
            }.onSuccess {
                renderState(viewModel.state.value)
                Toast.makeText(
                    this@MainActivity,
                    getString(R.string.diagnostics_completed),
                    Toast.LENGTH_SHORT
                ).show()
            }.onFailure { error ->
                viewModel.markDiagnosticsFailed(
                    error.message ?: getString(R.string.diagnostics_failed)
                )
                renderState(viewModel.state.value)
                Toast.makeText(
                    this@MainActivity,
                    error.message ?: getString(R.string.diagnostics_failed),
                    Toast.LENGTH_LONG
                ).show()
            }
        }
    }

    private fun startVpnRuntime() {
        runCatching {
            viewModel.runtimePreflight().requireReady()
            val request = viewModel.buildRuntimeRequest()
            updateTextWithFade(statusTitle, getString(R.string.runtime_starting))
            updateTextWithFade(statusDetail, getString(R.string.runtime_starting_detail))
            val intent = NoVpnService.startIntent(
                context = this,
                profileId = request.profileId,
                bypassRu = request.bypassRu,
                appRoutingMode = request.appRoutingMode,
                selectedPackages = request.selectedPackages,
                trafficStrategy = request.trafficStrategy,
                patternStrategy = request.patternStrategy
            )
            ContextCompat.startForegroundService(this, intent)
            lifecycleScope.launch {
                repeat(8) {
                    delay(500)
                    viewModel.refreshStateFromPreferences()
                    renderState(viewModel.state.value)
                }
            }
        }.onFailure { error ->
            statusTitle.text = getString(R.string.runtime_profile_incomplete)
            statusDetail.text = error.message ?: getString(R.string.import_profile_failed)
            Toast.makeText(
                this,
                error.message ?: getString(R.string.runtime_profile_incomplete),
                Toast.LENGTH_LONG
                ).show()
        }
    }

    private fun animatePowerState(state: TunnelState) {
        val visualState = when {
            state.runtimeRunning -> PowerVisualState.CONNECTED
            state.runtimeStatus == getString(R.string.runtime_starting) ||
                state.runtimeStatus == VpnRuntimeStatusStore.STATUS_STOPPING -> PowerVisualState.CONNECTING
            state.runtimeStatus == getString(R.string.runtime_start_failed) -> PowerVisualState.ERROR
            else -> PowerVisualState.IDLE
        }
        if (visualState == lastPowerVisualState) {
            return
        }
        lastPowerVisualState = visualState

        when (visualState) {
            PowerVisualState.IDLE -> stopPowerPulse(1f)
            PowerVisualState.CONNECTING -> startPowerPulse()
            PowerVisualState.CONNECTED -> {
                stopPowerPulse(1f)
                powerButton.animate().scaleX(1.08f).scaleY(1.08f).setDuration(180).withEndAction {
                    powerButton.animate().scaleX(1f).scaleY(1f).setDuration(220).start()
                }.start()
            }
            PowerVisualState.ERROR -> {
                stopPowerPulse(1f)
                powerButton.animate().scaleX(0.94f).scaleY(0.94f).setDuration(120).withEndAction {
                    powerButton.animate().scaleX(1f).scaleY(1f).setDuration(180).start()
                }.start()
            }
        }
    }

    private fun startPowerPulse() {
        if (powerPulseAnimator != null) {
            return
        }
        powerPulseAnimator = ValueAnimator.ofFloat(0.96f, 1.06f).apply {
            duration = 900L
            repeatCount = ValueAnimator.INFINITE
            repeatMode = ValueAnimator.REVERSE
            addUpdateListener { animator ->
                val value = animator.animatedValue as Float
                powerButton.scaleX = value
                powerButton.scaleY = value
                powerButton.alpha = 0.88f + ((value - 0.96f) / 0.10f) * 0.12f
            }
            start()
        }
    }

    private fun stopPowerPulse(targetScale: Float) {
        powerPulseAnimator?.cancel()
        powerPulseAnimator = null
        powerButton.animate()
            .scaleX(targetScale)
            .scaleY(targetScale)
            .alpha(1f)
            .setDuration(220)
            .start()
    }

    private fun animateDisconnectTransition() {
        stopPowerPulse(1f)
        powerButton.animate().scaleX(0.92f).scaleY(0.92f).setDuration(120).withEndAction {
            powerButton.animate().scaleX(1f).scaleY(1f).setDuration(180).start()
        }.start()
    }

    private fun updateTextWithFade(view: TextView, text: String) {
        if (view.text.toString() == text) {
            return
        }
        view.animate().cancel()
        view.animate().alpha(0f).setDuration(120).withEndAction {
            view.text = text
            view.animate().alpha(1f).setDuration(200).start()
        }.start()
    }

    private fun trafficStrategyLabel(strategy: com.novpn.data.TrafficObfuscationStrategy): String {
        return when (strategy) {
            com.novpn.data.TrafficObfuscationStrategy.BALANCED ->
                getString(R.string.traffic_strategy_balanced_short)
            com.novpn.data.TrafficObfuscationStrategy.CDN_MIMIC ->
                getString(R.string.traffic_strategy_cdn_short)
            com.novpn.data.TrafficObfuscationStrategy.FRAGMENTED ->
                getString(R.string.traffic_strategy_fragmented_short)
            com.novpn.data.TrafficObfuscationStrategy.MOBILE_MIX ->
                getString(R.string.traffic_strategy_mobile_short)
            com.novpn.data.TrafficObfuscationStrategy.TLS_BLEND ->
                getString(R.string.traffic_strategy_tls_short)
        }
    }

    private fun patternStrategyLabel(strategy: com.novpn.data.PatternMaskingStrategy): String {
        return when (strategy) {
            com.novpn.data.PatternMaskingStrategy.STEADY ->
                getString(R.string.pattern_strategy_steady_short)
            com.novpn.data.PatternMaskingStrategy.PULSE ->
                getString(R.string.pattern_strategy_pulse_short)
            com.novpn.data.PatternMaskingStrategy.RANDOMIZED ->
                getString(R.string.pattern_strategy_randomized_short)
            com.novpn.data.PatternMaskingStrategy.BURST_FADE ->
                getString(R.string.pattern_strategy_burst_short)
            com.novpn.data.PatternMaskingStrategy.QUIET_SWEEP ->
                getString(R.string.pattern_strategy_quiet_short)
        }
    }

    private fun label(text: String, sizeSp: Float, color: String, bold: Boolean): TextView {
        return TextView(this).apply {
            this.text = text
            setTextColor(Color.parseColor(color))
            setTextSize(TypedValue.COMPLEX_UNIT_SP, sizeSp)
            if (bold) {
                typeface = Typeface.DEFAULT_BOLD
            }
        }
    }

    private fun formatBytes(value: Long): String {
        if (value <= 0L) {
            return "0 B"
        }
        val units = arrayOf("B", "KiB", "MiB", "GiB", "TiB")
        var unitIndex = 0
        var current = value.toDouble()
        while (current >= 1024.0 && unitIndex < units.lastIndex) {
            current /= 1024.0
            unitIndex++
        }
        return String.format(java.util.Locale.US, "%.1f %s", current, units[unitIndex])
    }

    private fun formatHeaderServerName(value: String?): String {
        val normalized = value?.trim().orEmpty()
        if (normalized.isBlank()) {
            return getString(R.string.header_no_server)
        }
        if (normalized.length <= 10) {
            return normalized
        }
        return normalized.take(10) + "..."
    }

    private fun roundedDrawable(
        fillColor: String,
        strokeColor: String,
        radiusDp: Float,
        strokeWidthDp: Int
    ): GradientDrawable {
        return GradientDrawable().apply {
            shape = GradientDrawable.RECTANGLE
            cornerRadius = dp(radiusDp)
            setColor(Color.parseColor(fillColor))
            setStroke(dp(strokeWidthDp), Color.parseColor(strokeColor))
        }
    }

    private fun powerDrawable(active: Boolean): GradientDrawable {
        return GradientDrawable().apply {
            shape = GradientDrawable.OVAL
            setColor(Color.parseColor(if (active) "#133627" else "#101722"))
            setStroke(dp(3), Color.parseColor(if (active) "#5FD4A6" else "#283547"))
        }
    }

    private fun dp(value: Int): Int {
        return TypedValue.applyDimension(
            TypedValue.COMPLEX_UNIT_DIP,
            value.toFloat(),
            resources.displayMetrics
        ).toInt()
    }

    private fun dp(value: Float): Float {
        return TypedValue.applyDimension(
            TypedValue.COMPLEX_UNIT_DIP,
            value,
            resources.displayMetrics
        )
    }

    companion object {
        private const val RUNTIME_STATUS_SYNC_INTERVAL_MS = 350L
        private const val STARTING_BUTTON_TEXT = "Подключаем..."
        private const val STOPPING_BUTTON_TEXT = "Отключаем..."
    }

    private enum class PowerVisualState {
        IDLE,
        CONNECTING,
        CONNECTED,
        ERROR
    }

    private data class RuntimeStatusSnapshot(
        val running: Boolean,
        val status: String,
        val detail: String
    ) {
        companion object {
            fun from(state: TunnelState): RuntimeStatusSnapshot {
                return RuntimeStatusSnapshot(
                    running = state.runtimeRunning,
                    status = state.runtimeStatus,
                    detail = state.runtimeDetail
                )
            }
        }
    }
}
