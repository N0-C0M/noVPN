package com.novpn.ui

import android.app.Activity
import android.content.Intent
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.net.VpnService
import android.os.Bundle
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.HorizontalScrollView
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.core.content.ContextCompat
import com.novpn.data.AvailableProfile
import com.novpn.vpn.NoVpnService

class MainActivity : ComponentActivity() {
    private val viewModel by viewModels<TunnelViewModel>()

    private lateinit var statusTitle: TextView
    private lateinit var statusDetail: TextView
    private lateinit var powerButton: Button
    private lateinit var serverStrip: LinearLayout
    private lateinit var settingsButton: Button

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            startVpnRuntime()
        } else {
            statusTitle.text = "Permission required"
            statusDetail.text = "VPN permission was not granted."
        }
    }

    private val settingsLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) {
        viewModel.refreshStateFromPreferences()
        renderState(viewModel.state.value)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(buildContentView())
        viewModel.refreshStateFromPreferences()
        renderState(viewModel.state.value)
    }

    override fun onResume() {
        super.onResume()
        viewModel.refreshStateFromPreferences()
        renderState(viewModel.state.value)
    }

    private fun buildContentView(): View {
        val root = ScrollView(this).apply {
            background = GradientDrawable(
                GradientDrawable.Orientation.TOP_BOTTOM,
                intArrayOf(Color.parseColor("#08111F"), Color.parseColor("#0E1C30"))
            )
        }

        val content = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(20), dp(22), dp(20), dp(24))
        }
        root.addView(
            content,
            LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            )
        )

        content.addView(buildHeader())
        content.addView(buildHeroSection())
        content.addView(buildServerSection())
        return root
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

        titleBlock.addView(label("NoVPN", 28f, "#F4F7FB", true))
        titleBlock.addView(
            label(
                "Fast switching between servers and split tunnel presets.",
                13f,
                "#96A9C5",
                false
            ).apply {
                setPadding(0, dp(4), 0, 0)
            }
        )

        settingsButton = Button(this).apply {
            text = "Settings"
            isAllCaps = false
            setTextColor(Color.parseColor("#F4F7FB"))
            textSize = 12f
            typeface = Typeface.DEFAULT_BOLD
            background = roundedDrawable("#152740", 18f)
            setPadding(dp(16), dp(10), dp(16), dp(10))
            setOnClickListener {
                settingsLauncher.launch(Intent(this@MainActivity, SettingsActivity::class.java))
            }
        }

        row.addView(titleBlock)
        row.addView(settingsButton)
        return row
    }

    private fun buildHeroSection(): View {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            background = roundedDrawable("#102038", 28f)
            setPadding(dp(18), dp(26), dp(18), dp(28))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(22)
            }

            addView(
                label("Tap to connect", 14f, "#A7B8D2", false).apply {
                    gravity = Gravity.CENTER
                }
            )

            powerButton = Button(this@MainActivity).apply {
                isAllCaps = false
                textSize = 20f
                typeface = Typeface.DEFAULT_BOLD
                setTextColor(Color.parseColor("#08111F"))
                gravity = Gravity.CENTER
                background = ovalDrawable("#F78764")
                layoutParams = LinearLayout.LayoutParams(dp(210), dp(210)).apply {
                    topMargin = dp(20)
                    bottomMargin = dp(22)
                }
                setOnClickListener { toggleRuntime() }
            }
            addView(powerButton)

            statusTitle = label("Ready", 24f, "#F4F7FB", true).apply {
                gravity = Gravity.CENTER
            }
            addView(statusTitle)

            statusDetail = label("", 13f, "#A7B8D2", false).apply {
                gravity = Gravity.CENTER
                setPadding(dp(8), dp(10), dp(8), 0)
            }
            addView(statusDetail)
        }
    }

    private fun buildServerSection(): View {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#102038", 28f)
            setPadding(dp(18), dp(18), dp(18), dp(18))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(18)
            }

            addView(label("Available servers", 16f, "#F4F7FB", true))
            addView(
                label(
                    "Pick a server below. Split tunnel settings live behind the top-right button.",
                    12f,
                    "#96A9C5",
                    false
                ).apply {
                    setPadding(0, dp(6), 0, dp(12))
                }
            )

            val scroll = HorizontalScrollView(this@MainActivity).apply {
                isHorizontalScrollBarEnabled = false
            }
            serverStrip = LinearLayout(this@MainActivity).apply {
                orientation = LinearLayout.HORIZONTAL
            }
            scroll.addView(serverStrip)
            addView(scroll)
        }
    }

    private fun renderState(state: TunnelState) {
        val selected = state.availableProfiles.firstOrNull { it.assetName == state.selectedProfileAsset }
            ?: state.availableProfiles.firstOrNull()

        val modeLine = if (state.bypassRu) "RU traffic bypass enabled" else "Full tunnel mode"
        val appsLine = "${state.excludedPackages.size} app exclusions"
        val serverLine = selected?.let { "${it.name}  |  ${it.address}" } ?: "No server profiles found"

        if (state.runtimeRunning) {
            powerButton.text = "Disconnect"
            powerButton.background = ovalDrawable("#58E0B5")
            statusTitle.text = "Connected"
        } else {
            powerButton.text = "Connect"
            powerButton.background = ovalDrawable("#F78764")
            statusTitle.text = if (state.runtimeStatus == "Service stopped") "Ready" else state.runtimeStatus
        }

        statusDetail.text = buildString {
            appendLine(serverLine)
            appendLine(modeLine)
            append(appsLine)
        }

        rebuildServerCards(state.availableProfiles, state.selectedProfileAsset)
    }

    private fun rebuildServerCards(profiles: List<AvailableProfile>, selectedAsset: String) {
        serverStrip.removeAllViews()

        if (profiles.isEmpty()) {
            serverStrip.addView(
                label("Add profile.*.json assets to populate this list.", 12f, "#96A9C5", false)
            )
            return
        }

        profiles.forEachIndexed { index, profile ->
            val selected = profile.assetName == selectedAsset
            val card = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                background = roundedDrawable(if (selected) "#1B3553" else "#152740", 24f)
                setPadding(dp(16), dp(16), dp(16), dp(16))
                isClickable = true
                isFocusable = true
                setOnClickListener {
                    viewModel.selectProfile(profile.assetName)
                    viewModel.generateConfig()
                    renderState(viewModel.state.value)
                }
                layoutParams = LinearLayout.LayoutParams(dp(220), LinearLayout.LayoutParams.WRAP_CONTENT).apply {
                    rightMargin = if (index == profiles.lastIndex) 0 else dp(12)
                }
            }

            card.addView(label(profile.name, 15f, "#F4F7FB", true))
            card.addView(
                label(profile.address, 12f, if (selected) "#DCEAFF" else "#A7B8D2", false).apply {
                    setPadding(0, dp(8), 0, 0)
                }
            )
            card.addView(
                label("SNI ${profile.serverName}", 12f, "#7FA0C6", false).apply {
                    setPadding(0, dp(10), 0, 0)
                }
            )

            serverStrip.addView(card)
        }
    }

    private fun toggleRuntime() {
        if (viewModel.state.value.runtimeRunning) {
            startService(NoVpnService.stopIntent(this))
            viewModel.markRuntimeStopped()
            renderState(viewModel.state.value)
            return
        }

        val prepareIntent = VpnService.prepare(this)
        if (prepareIntent != null) {
            vpnPermissionLauncher.launch(prepareIntent)
        } else {
            startVpnRuntime()
        }
    }

    private fun startVpnRuntime() {
        val request = viewModel.buildRuntimeRequest()
        viewModel.generateConfig()
        val configPath = viewModel.state.value.generatedConfigPath
        val intent = NoVpnService.startIntent(
            context = this,
            profileAsset = request.profileAsset,
            bypassRu = request.bypassRu,
            excludedPackages = request.excludedPackages
        )
        ContextCompat.startForegroundService(this, intent)
        viewModel.markRuntimeStarted(configPath)
        renderState(viewModel.state.value)
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

    private fun roundedDrawable(fillColor: String, radiusDp: Float): GradientDrawable {
        return GradientDrawable().apply {
            shape = GradientDrawable.RECTANGLE
            cornerRadius = dp(radiusDp)
            setColor(Color.parseColor(fillColor))
        }
    }

    private fun ovalDrawable(fillColor: String): GradientDrawable {
        return GradientDrawable().apply {
            shape = GradientDrawable.OVAL
            setColor(Color.parseColor(fillColor))
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
}
