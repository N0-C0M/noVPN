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
import com.novpn.R
import com.novpn.data.AvailableProfile
import com.novpn.vpn.NoVpnService

class MainActivity : ComponentActivity() {
    private val viewModel by viewModels<TunnelViewModel>()

    private lateinit var headerLabel: TextView
    private lateinit var headerServer: TextView
    private lateinit var statusTitle: TextView
    private lateinit var statusDetail: TextView
    private lateinit var powerButton: Button
    private lateinit var serverStrip: LinearLayout

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
                intArrayOf(Color.parseColor("#02040A"), Color.parseColor("#060B12"))
            )
        }

        val content = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(20), dp(22), dp(20), dp(28))
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

        headerLabel = label(getString(R.string.header_profile_label), 12f, "#6F8096", false).apply {
            letterSpacing = 0.08f
        }
        headerServer = label(getString(R.string.header_no_server), 22f, "#F3F6FB", true).apply {
            setPadding(0, dp(6), 0, 0)
        }

        titleBlock.addView(headerLabel)
        titleBlock.addView(headerServer)

        val settingsButton = Button(this).apply {
            text = getString(R.string.settings)
            isAllCaps = false
            setTextColor(Color.parseColor("#F3F6FB"))
            textSize = 12f
            typeface = Typeface.DEFAULT_BOLD
            background = roundedDrawable("#0E1520", "#243244", 22f, 2)
            setPadding(dp(18), dp(12), dp(18), dp(12))
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
            background = roundedDrawable("#0A1018", "#182432", 38f, 2)
            setPadding(dp(18), dp(28), dp(18), dp(30))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(24)
            }

            addView(
                label(getString(R.string.hero_tap_to_connect), 13f, "#7B8DA3", false).apply {
                    gravity = Gravity.CENTER
                }
            )

            powerButton = Button(this@MainActivity).apply {
                isAllCaps = false
                textSize = 20f
                typeface = Typeface.DEFAULT_BOLD
                setTextColor(Color.parseColor("#F3F6FB"))
                gravity = Gravity.CENTER
                background = powerDrawable(false)
                layoutParams = LinearLayout.LayoutParams(dp(214), dp(214)).apply {
                    topMargin = dp(22)
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

        headerServer.text = selected?.name ?: getString(R.string.header_no_server)

        val modeLine = if (state.bypassRu) {
            getString(R.string.mode_bypass_ru)
        } else {
            getString(R.string.mode_full_tunnel)
        }
        val appsLine = getString(R.string.app_exclusions_count, state.excludedPackages.size)
        val serverLine = selected?.let {
            getString(R.string.server_line_format, it.name, it.address)
        } ?: getString(R.string.no_profiles_found)

        if (state.runtimeRunning) {
            powerButton.text = getString(R.string.disconnect)
            powerButton.background = powerDrawable(true)
            statusTitle.text = getString(R.string.status_connected)
        } else {
            powerButton.text = getString(R.string.connect)
            powerButton.background = powerDrawable(false)
            statusTitle.text = if (state.runtimeStatus.isBlank() || state.runtimeStatus == getString(R.string.service_stopped)) {
                getString(R.string.status_ready)
            } else {
                state.runtimeStatus
            }
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
                label(getString(R.string.no_profiles_found), 12f, "#7B8DA3", false)
            )
            return
        }

        profiles.forEachIndexed { index, profile ->
            val selected = profile.assetName == selectedAsset
            val card = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                background = roundedDrawable(
                    if (selected) "#122232" else "#0D141D",
                    if (selected) "#345273" else "#1A2634",
                    30f,
                    2
                )
                setPadding(dp(18), dp(18), dp(18), dp(18))
                isClickable = true
                isFocusable = true
                setOnClickListener {
                    viewModel.selectProfile(profile.assetName)
                    viewModel.generateConfig()
                    renderState(viewModel.state.value)
                }
                layoutParams = LinearLayout.LayoutParams(dp(228), LinearLayout.LayoutParams.WRAP_CONTENT).apply {
                    rightMargin = if (index == profiles.lastIndex) 0 else dp(12)
                }
            }

            card.addView(label(profile.name, 15f, "#F3F6FB", true))
            card.addView(
                label(profile.address, 12f, if (selected) "#D8E5F3" else "#93A3B7", false).apply {
                    setPadding(0, dp(8), 0, 0)
                }
            )
            card.addView(
                label(getString(R.string.server_sni_format, profile.serverName), 12f, "#6E88A6", false).apply {
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
}
