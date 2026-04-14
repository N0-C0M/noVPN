package com.novpn.ui

import android.app.Activity
import android.content.Intent
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
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.core.content.ContextCompat
import androidx.core.widget.doAfterTextChanged
import androidx.lifecycle.lifecycleScope
import com.novpn.R
import com.novpn.data.AvailableProfile
import com.novpn.data.CodeRedeemKind
import com.novpn.vpn.NoVpnService
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {
    private val viewModel by viewModels<TunnelViewModel>()

    private lateinit var headerServerLabel: TextView
    private lateinit var statusTitle: TextView
    private lateinit var statusDetail: TextView
    private lateinit var powerButton: Button
    private lateinit var subscriptionInput: EditText
    private lateinit var activateSubscriptionButton: Button
    private lateinit var subscriptionStateLabel: TextView
    private lateinit var serversTitle: TextView
    private lateinit var serversContainer: LinearLayout

    private val settingsLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) {
        viewModel.refreshStateFromPreferences()
        renderState(viewModel.state.value)
    }

    private val importProfileLauncher = registerForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri ->
        if (uri == null) {
            return@registerForActivityResult
        }

        runCatching {
            viewModel.importProfile(uri)
        }.onSuccess {
            renderState(viewModel.state.value)
            Toast.makeText(
                this,
                getString(R.string.import_profile_success, viewModel.selectedProfileName()),
                Toast.LENGTH_SHORT
            ).show()
        }.onFailure { error ->
            Toast.makeText(
                this,
                error.message ?: getString(R.string.import_profile_failed),
                Toast.LENGTH_LONG
            ).show()
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

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode != VPN_PERMISSION_REQUEST_CODE) {
            return
        }

        if (resultCode == Activity.RESULT_OK) {
            startVpnRuntime()
        } else {
            Toast.makeText(this, getString(R.string.status_permission_required), Toast.LENGTH_SHORT).show()
        }
    }

    private fun buildContentView(): View {
        val root = FrameLayout(this).apply {
            background = GradientDrawable(
                GradientDrawable.Orientation.TOP_BOTTOM,
                intArrayOf(Color.parseColor("#0A0C10"), Color.parseColor("#050608"))
            )
        }

        val scroll = ScrollView(this)
        val content = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(20), dp(20), dp(20), dp(120))
        }
        scroll.addView(
            content,
            LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            )
        )

        content.addView(buildHeader())
        content.addView(buildMainCard())

        root.addView(
            scroll,
            FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.MATCH_PARENT
            )
        )

        root.addView(
            buildBottomNav(),
            FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                gravity = Gravity.BOTTOM
                marginStart = dp(16)
                marginEnd = dp(16)
                bottomMargin = dp(16)
            }
        )

        return root
    }

    private fun buildHeader(): View {
        return LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL

            val titleBlock = LinearLayout(this@MainActivity).apply {
                orientation = LinearLayout.VERTICAL
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }

            titleBlock.addView(
                label("NOVPN", 13f, "#9AA3B2", true).apply {
                    letterSpacing = 0.08f
                }
            )
            headerServerLabel = label(getString(R.string.header_no_server), 22f, "#F5F7FA", true).apply {
                setPadding(0, dp(4), 0, 0)
            }
            titleBlock.addView(headerServerLabel)

            val importButton = smallHeaderButton(getString(R.string.import_profile)) {
                importProfileLauncher.launch("*/*")
            }
            val settingsButton = smallHeaderButton(getString(R.string.settings)) {
                settingsLauncher.launch(Intent(this@MainActivity, SettingsActivity::class.java))
            }.apply {
                (layoutParams as? LinearLayout.LayoutParams)?.marginStart = dp(8)
            }

            addView(titleBlock)
            addView(importButton)
            addView(settingsButton)
        }
    }

    private fun buildMainCard(): View {
        return card(topMargin = dp(20), corner = 28f).apply {
            gravity = Gravity.CENTER_HORIZONTAL

            addView(
                label("Главный экран", 12f, "#8A94A6", false).apply {
                    gravity = Gravity.CENTER
                }
            )

            statusTitle = label("VPN выключен", 24f, "#F5F7FA", true).apply {
                gravity = Gravity.CENTER
                setPadding(0, dp(10), 0, 0)
            }
            addView(statusTitle)

            statusDetail = label("", 13f, "#97A2B4", false).apply {
                gravity = Gravity.CENTER
                setPadding(dp(8), dp(8), dp(8), 0)
            }
            addView(statusDetail)

            powerButton = Button(this@MainActivity).apply {
                text = "ВКЛ"
                isAllCaps = false
                setTextColor(Color.parseColor("#0A0D12"))
                textSize = 16f
                typeface = Typeface.DEFAULT_BOLD
                background = roundedDrawable("#A7F259", "#B5FF6F", 52f, 1)
                layoutParams = LinearLayout.LayoutParams(dp(132), dp(132)).apply {
                    topMargin = dp(18)
                    bottomMargin = dp(16)
                    gravity = Gravity.CENTER_HORIZONTAL
                }
                setOnClickListener { toggleRuntime() }
            }
            addView(powerButton)

            subscriptionInput = EditText(this@MainActivity).apply {
                hint = "Введите код подписки"
                setHintTextColor(Color.parseColor("#6E7788"))
                setTextColor(Color.parseColor("#F5F7FA"))
                inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
                background = roundedDrawable("#11151D", "#232C3C", 16f, 1)
                setPadding(dp(14), dp(12), dp(14), dp(12))
                doAfterTextChanged {
                    activateSubscriptionButton.isEnabled = !it.isNullOrBlank()
                }
            }
            addView(
                subscriptionInput,
                LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                )
            )

            activateSubscriptionButton = Button(this@MainActivity).apply {
                text = "Активировать подписку"
                isAllCaps = false
                setTextColor(Color.parseColor("#F5F7FA"))
                textSize = 13f
                typeface = Typeface.DEFAULT_BOLD
                background = roundedDrawable("#162430", "#2E455D", 16f, 1)
                setPadding(dp(14), dp(12), dp(14), dp(12))
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    topMargin = dp(10)
                }
                setOnClickListener { activateSubscription() }
            }
            addView(activateSubscriptionButton)

            subscriptionStateLabel = label("Подписка не активирована", 12f, "#8A94A6", false).apply {
                setPadding(0, dp(10), 0, 0)
            }
            addView(subscriptionStateLabel)

            serversTitle = label("Доступные серверы", 14f, "#F5F7FA", true).apply {
                setPadding(0, dp(18), 0, dp(10))
                visibility = View.GONE
            }
            addView(serversTitle)

            serversContainer = LinearLayout(this@MainActivity).apply {
                orientation = LinearLayout.VERTICAL
                visibility = View.GONE
            }
            addView(
                serversContainer,
                LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                )
            )
        }
    }

    private fun renderState(state: TunnelState) {
        val selectedProfile = state.availableProfiles.firstOrNull { it.profileId == state.selectedProfileId }
            ?: state.availableProfiles.firstOrNull()
        headerServerLabel.text = selectedProfile?.name ?: getString(R.string.header_no_server)

        val remainingBytes = state.trafficLimitBytes
            .takeIf { it > 0L }
            ?.let { limit -> (limit - state.trafficUsedBytes).coerceAtLeast(0L) }

        val detailLines = buildList {
            if (selectedProfile != null) {
                add("Сервер: ${selectedProfile.name}")
                if (selectedProfile.locationLabel.isNotBlank()) {
                    add("Локация: ${selectedProfile.locationLabel}")
                }
            }
            if (state.runtimeDetail.isNotBlank()) {
                add(state.runtimeDetail)
            }
            if (remainingBytes != null) {
                add("Трафик: ${formatBytes(remainingBytes)} из ${formatBytes(state.trafficLimitBytes)}")
            }
        }

        statusTitle.text = when {
            state.runtimeRunning -> "VPN включен"
            state.runtimeStatus.isNotBlank() && state.runtimeStatus != getString(R.string.service_stopped) -> state.runtimeStatus
            else -> "VPN выключен"
        }
        statusDetail.text = if (detailLines.isEmpty()) {
            "Нажмите кнопку для подключения"
        } else {
            detailLines.joinToString("\n")
        }

        val buttonEnabled = !state.runtimeRunning
        powerButton.text = if (buttonEnabled) "ВКЛ" else "ВЫКЛ"
        powerButton.background = if (buttonEnabled) {
            roundedDrawable("#A7F259", "#B5FF6F", 52f, 1)
        } else {
            roundedDrawable("#FF7676", "#FF9595", 52f, 1)
        }

        val currentCode = subscriptionInput.text?.toString().orEmpty()
        if (currentCode != state.inviteCode) {
            subscriptionInput.setText(state.inviteCode)
            subscriptionInput.setSelection(subscriptionInput.text?.length ?: 0)
        }
        activateSubscriptionButton.isEnabled = subscriptionInput.text?.isNotBlank() == true

        val subscriptionActive = state.inviteCode.isNotBlank() ||
            state.trafficLimitBytes > 0L ||
            state.availableProfiles.any { it.isImported }
        subscriptionStateLabel.text = if (subscriptionActive) {
            "Подписка активна"
        } else {
            "Подписка не активирована"
        }

        val showServers = subscriptionActive
        serversTitle.visibility = if (showServers) View.VISIBLE else View.GONE
        serversContainer.visibility = if (showServers) View.VISIBLE else View.GONE
        rebuildServerCards(state.availableProfiles, state.selectedProfileId, showServers)
    }

    private fun rebuildServerCards(
        profiles: List<AvailableProfile>,
        selectedProfileId: String,
        visible: Boolean
    ) {
        serversContainer.removeAllViews()
        if (!visible) {
            return
        }

        if (profiles.isEmpty()) {
            serversContainer.addView(
                label("После активации здесь появятся серверы", 12f, "#8A94A6", false)
            )
            return
        }

        profiles.forEachIndexed { index, profile ->
            val isSelected = profile.profileId == selectedProfileId
            val card = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                background = roundedDrawable(
                    if (isSelected) "#19232F" else "#11151D",
                    if (isSelected) "#5E7698" else "#232C3C",
                    16f,
                    1
                )
                setPadding(dp(14), dp(12), dp(14), dp(12))
                isClickable = true
                isFocusable = true
                setOnClickListener {
                    viewModel.selectProfile(profile.profileId)
                    runCatching { viewModel.generateConfig() }
                    renderState(viewModel.state.value)
                }
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    bottomMargin = if (index == profiles.lastIndex) 0 else dp(8)
                }
            }
            card.addView(label(profile.name, 14f, "#F5F7FA", true))
            val location = profile.locationLabel.ifBlank { "Локация не указана" }
            card.addView(label(location, 12f, "#97A2B4", false).apply { setPadding(0, dp(4), 0, 0) })
            serversContainer.addView(card)
        }
    }

    private fun activateSubscription() {
        val code = subscriptionInput.text?.toString().orEmpty().trim()
        if (code.isBlank()) {
            Toast.makeText(this, getString(R.string.invite_code_missing), Toast.LENGTH_SHORT).show()
            return
        }

        viewModel.setInviteCode(code)
        activateSubscriptionButton.isEnabled = false
        statusTitle.text = "Активация подписки"
        statusDetail.text = "Проверяем код и загружаем доступные серверы"

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
                    }

                    CodeRedeemKind.PROMO -> {
                        Toast.makeText(
                            this@MainActivity,
                            "Промокод активирован",
                            Toast.LENGTH_SHORT
                        ).show()
                    }
                }
            }.onFailure { error ->
                renderState(viewModel.state.value)
                Toast.makeText(
                    this@MainActivity,
                    error.message ?: getString(R.string.invite_code_activation_failed),
                    Toast.LENGTH_LONG
                ).show()
            }
            activateSubscriptionButton.isEnabled = true
        }
    }

    private fun toggleRuntime() {
        if (viewModel.state.value.runtimeRunning) {
            startService(NoVpnService.stopIntent(this))
            viewModel.markRuntimeStopped()
            renderState(viewModel.state.value)
            return
        }

        beginVpnStartFlow()
    }

    private fun beginVpnStartFlow() {
        val prepareIntent = VpnService.prepare(this)
        if (prepareIntent != null) {
            startActivityForResult(prepareIntent, VPN_PERMISSION_REQUEST_CODE)
        } else {
            startVpnRuntime()
        }
    }

    private fun startVpnRuntime() {
        runCatching {
            val request = viewModel.buildRuntimeRequest()
            viewModel.generateConfig()
            val configPath = viewModel.state.value.generatedConfigPath
            val intent = NoVpnService.startIntent(
                context = this,
                profileId = request.profileId,
                bypassRu = request.bypassRu,
                appRoutingMode = request.appRoutingMode,
                selectedPackages = request.selectedPackages,
                trafficStrategy = request.trafficStrategy,
                patternStrategy = request.patternStrategy,
                autoToggleByScreenState = request.autoToggleByScreenState,
                startOnlyForWhitelistApps = request.startOnlyForWhitelistApps
            )
            ContextCompat.startForegroundService(this, intent)
            viewModel.markRuntimeStarted(configPath)
            renderState(viewModel.state.value)
        }.onFailure { error ->
            Toast.makeText(
                this,
                error.message ?: getString(R.string.runtime_profile_incomplete),
                Toast.LENGTH_LONG
            ).show()
        }
    }

    private fun buildBottomNav(): View {
        return LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER
            background = roundedDrawable("#0F131A", "#202838", 20f, 1)
            setPadding(dp(6), dp(6), dp(6), dp(6))

            addView(
                navButton("Главная", active = true) {
                    // current screen
                },
                navLayoutParams(end = dp(4))
            )

            addView(
                navButton("Магазин", active = false) {
                    Toast.makeText(this@MainActivity, "Будет доступно позже", Toast.LENGTH_SHORT).show()
                },
                navLayoutParams(start = dp(2), end = dp(2))
            )

            addView(
                navButton("Настройки", active = false) {
                    settingsLauncher.launch(Intent(this@MainActivity, SettingsActivity::class.java))
                },
                navLayoutParams(start = dp(4))
            )
        }
    }

    private fun navLayoutParams(start: Int = 0, end: Int = 0): LinearLayout.LayoutParams {
        return LinearLayout.LayoutParams(0, dp(50), 1f).apply {
            marginStart = start
            marginEnd = end
        }
    }

    private fun navButton(title: String, active: Boolean, onClick: () -> Unit): Button {
        return Button(this).apply {
            text = title
            isAllCaps = false
            textSize = 13f
            typeface = Typeface.DEFAULT_BOLD
            setTextColor(Color.parseColor(if (active) "#D9E2F2" else "#8C97AA"))
            background = roundedDrawable(
                fillColor = if (active) "#1A2230" else "#11151D",
                strokeColor = if (active) "#2E455D" else "#232C3C",
                radiusDp = 14f,
                strokeWidthDp = 1
            )
            gravity = Gravity.CENTER
            minHeight = 0
            minimumHeight = 0
            setPadding(dp(8), 0, dp(8), 0)
            setOnClickListener { onClick() }
        }
    }

    private fun smallHeaderButton(textValue: String, onClick: () -> Unit): Button {
        return Button(this).apply {
            text = textValue
            isAllCaps = false
            setTextColor(Color.parseColor("#D9E2F2"))
            textSize = 12f
            typeface = Typeface.DEFAULT_BOLD
            background = roundedDrawable("#11151D", "#232C3C", 12f, 1)
            setPadding(dp(10), dp(8), dp(10), dp(8))
            setOnClickListener { onClick() }
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.WRAP_CONTENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            )
        }
    }

    private fun card(topMargin: Int, corner: Float): LinearLayout {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#0D1118", "#1F2735", corner, 1)
            setPadding(dp(16), dp(16), dp(16), dp(16))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                this.topMargin = topMargin
            }
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
        private const val VPN_PERMISSION_REQUEST_CODE = 4107
    }
}
