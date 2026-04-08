package com.novpn.ui

import android.app.Activity
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.res.ColorStateList
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.CheckBox
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import androidx.activity.ComponentActivity
import com.novpn.R
import com.novpn.data.AppRoutingMode
import com.novpn.data.ClientPreferences
import com.novpn.data.DisguiseIdentity
import com.novpn.data.DisguiseIdentityGenerator
import com.novpn.data.PatternMaskingStrategy
import com.novpn.data.TrafficObfuscationStrategy
import com.novpn.split.InstalledAppEntry
import com.novpn.split.InstalledAppsScanner

class SettingsActivity : ComponentActivity() {
    private val preferences by lazy { ClientPreferences(this) }
    private val appScanner by lazy { InstalledAppsScanner(this) }

    private val selectedPackages = linkedSetOf<String>()
    private val appCheckBoxes = mutableListOf<Pair<InstalledAppEntry, CheckBox>>()

    private lateinit var bypassRuCheckBox: CheckBox
    private lateinit var forceServerIpCheckBox: CheckBox
    private lateinit var modeExcludeButton: Button
    private lateinit var modeOnlySelectedButton: Button
    private lateinit var trafficBalancedButton: Button
    private lateinit var trafficCdnButton: Button
    private lateinit var trafficFragmentedButton: Button
    private lateinit var trafficMobileButton: Button
    private lateinit var trafficTlsButton: Button
    private lateinit var patternSteadyButton: Button
    private lateinit var patternPulseButton: Button
    private lateinit var patternRandomizedButton: Button
    private lateinit var patternBurstButton: Button
    private lateinit var patternQuietButton: Button
    private lateinit var appsToggleButton: Button
    private lateinit var appsSummary: TextView
    private lateinit var appsListContainer: LinearLayout
    private lateinit var disguiseNameValue: TextView
    private lateinit var disguisePackageValue: TextView
    private lateinit var disguiseCommandValue: TextView

    private var appRoutingMode = AppRoutingMode.EXCLUDE_SELECTED
    private var trafficStrategy = TrafficObfuscationStrategy.BALANCED
    private var patternStrategy = PatternMaskingStrategy.STEADY
    private var disguiseIdentity = DisguiseIdentityGenerator.defaultIdentity()
    private var appsLoaded = false
    private var appsExpanded = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        selectedPackages += preferences.excludedPackages()
        appRoutingMode = preferences.appRoutingMode()
        trafficStrategy = preferences.trafficObfuscationStrategy()
        patternStrategy = preferences.patternMaskingStrategy()
        disguiseIdentity = preferences.disguiseIdentity()
        setContentView(buildContentView())
        refreshSelectionViews()
    }

    private fun buildContentView(): ScrollView {
        return ScrollView(this).apply {
            background = GradientDrawable(
                GradientDrawable.Orientation.TOP_BOTTOM,
                intArrayOf(Color.parseColor("#02040A"), Color.parseColor("#060B12"))
            )

            val content = LinearLayout(this@SettingsActivity).apply {
                orientation = LinearLayout.VERTICAL
                setPadding(dp(20), dp(22), dp(20), dp(28))
            }

            content.addView(buildHeader())
            content.addView(buildRoutingCard())
            content.addView(buildStrategiesCard())
            content.addView(buildAppsCard())
            content.addView(buildDisguiseCard())
            content.addView(buildSaveButton())
            addView(content)
        }
    }

    private fun buildHeader(): LinearLayout {
        return LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL

            val titleBlock = LinearLayout(this@SettingsActivity).apply {
                orientation = LinearLayout.VERTICAL
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }
            titleBlock.addView(label(getString(R.string.settings_title), 26f, "#F3F6FB", true))
            titleBlock.addView(
                label(getString(R.string.settings_subtitle), 13f, "#8091A7", false).apply {
                    setPadding(0, dp(4), 0, 0)
                }
            )

            val closeButton = Button(this@SettingsActivity).apply {
                text = getString(R.string.close)
                isAllCaps = false
                setTextColor(Color.parseColor("#F3F6FB"))
                background = roundedDrawable("#0E1520", "#243244", 22f, 2)
                setPadding(dp(18), dp(12), dp(18), dp(12))
                setOnClickListener { finish() }
            }

            addView(titleBlock)
            addView(closeButton)
        }
    }

    private fun buildRoutingCard(): LinearLayout {
        return card(dp(24)).apply {
            addView(label(getString(R.string.routing_title), 16f, "#F3F6FB", true))
            addView(
                label(getString(R.string.routing_subtitle), 12f, "#8091A7", false).apply {
                    setPadding(0, dp(8), 0, dp(10))
                }
            )

            bypassRuCheckBox = CheckBox(this@SettingsActivity).apply {
                text = getString(R.string.do_not_proxy_ru)
                isChecked = preferences.isBypassRuEnabled()
                setTextColor(Color.parseColor("#F3F6FB"))
                textSize = 14f
                buttonTintList = ColorStateList.valueOf(Color.parseColor("#5FD4A6"))
            }
            addView(bypassRuCheckBox)

            forceServerIpCheckBox = CheckBox(this@SettingsActivity).apply {
                text = "Пока домен не активен, использовать только IP сервера"
                isChecked = preferences.forceServerIpMode()
                setTextColor(Color.parseColor("#F3F6FB"))
                textSize = 14f
                buttonTintList = ColorStateList.valueOf(Color.parseColor("#5FD4A6"))
                setPadding(0, dp(10), 0, 0)
            }
            addView(forceServerIpCheckBox)

            addView(
                label(getString(R.string.apps_mode_title), 14f, "#F3F6FB", true).apply {
                    setPadding(0, dp(16), 0, dp(6))
                }
            )
            addView(
                label(getString(R.string.apps_mode_subtitle), 12f, "#8091A7", false).apply {
                    setPadding(0, 0, 0, dp(12))
                }
            )

            val modeRow = LinearLayout(this@SettingsActivity).apply {
                orientation = LinearLayout.VERTICAL
            }
            modeExcludeButton = choiceButton(getString(R.string.apps_mode_exclude)) {
                appRoutingMode = AppRoutingMode.EXCLUDE_SELECTED
                refreshSelectionViews()
            }
            modeOnlySelectedButton = choiceButton(getString(R.string.apps_mode_only_selected)) {
                appRoutingMode = AppRoutingMode.ONLY_SELECTED
                refreshSelectionViews()
            }.apply {
                val params = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                )
                params.topMargin = dp(10)
                layoutParams = params
            }
            modeRow.addView(modeExcludeButton)
            modeRow.addView(modeOnlySelectedButton)
            addView(modeRow)
        }
    }

    private fun buildStrategiesCard(): LinearLayout {
        return card(dp(18)).apply {
            addView(label(getString(R.string.strategies_title), 16f, "#F3F6FB", true))
            addView(
                label(getString(R.string.strategies_subtitle), 12f, "#8091A7", false).apply {
                    setPadding(0, dp(8), 0, dp(14))
                }
            )

            addView(label(getString(R.string.traffic_strategy_title), 14f, "#F3F6FB", true))
            addView(
                label(getString(R.string.traffic_strategy_subtitle), 12f, "#8091A7", false).apply {
                    setPadding(0, dp(6), 0, dp(10))
                }
            )

            trafficBalancedButton = choiceButton(getString(R.string.traffic_strategy_balanced)) {
                trafficStrategy = TrafficObfuscationStrategy.BALANCED
                refreshSelectionViews()
            }
            addView(trafficBalancedButton)

            trafficCdnButton = choiceButton(getString(R.string.traffic_strategy_cdn)) {
                trafficStrategy = TrafficObfuscationStrategy.CDN_MIMIC
                refreshSelectionViews()
            }.apply {
                layoutParams = stackedChoiceParams()
            }
            addView(trafficCdnButton)

            trafficFragmentedButton = choiceButton(getString(R.string.traffic_strategy_fragmented)) {
                trafficStrategy = TrafficObfuscationStrategy.FRAGMENTED
                refreshSelectionViews()
            }.apply {
                layoutParams = stackedChoiceParams()
            }
            addView(trafficFragmentedButton)

            trafficMobileButton = choiceButton("Под мобильный браузер") {
                trafficStrategy = TrafficObfuscationStrategy.MOBILE_MIX
                refreshSelectionViews()
            }.apply {
                layoutParams = stackedChoiceParams()
            }
            addView(trafficMobileButton)

            trafficTlsButton = choiceButton("Смешанный TLS-профиль") {
                trafficStrategy = TrafficObfuscationStrategy.TLS_BLEND
                refreshSelectionViews()
            }.apply {
                layoutParams = stackedChoiceParams()
            }
            addView(trafficTlsButton)

            addView(
                label(getString(R.string.pattern_strategy_title), 14f, "#F3F6FB", true).apply {
                    setPadding(0, dp(18), 0, dp(6))
                }
            )
            addView(
                label(getString(R.string.pattern_strategy_subtitle), 12f, "#8091A7", false).apply {
                    setPadding(0, 0, 0, dp(10))
                }
            )

            patternSteadyButton = choiceButton(getString(R.string.pattern_strategy_steady)) {
                patternStrategy = PatternMaskingStrategy.STEADY
                refreshSelectionViews()
            }
            addView(patternSteadyButton)

            patternPulseButton = choiceButton(getString(R.string.pattern_strategy_pulse)) {
                patternStrategy = PatternMaskingStrategy.PULSE
                refreshSelectionViews()
            }.apply {
                layoutParams = stackedChoiceParams()
            }
            addView(patternPulseButton)

            patternRandomizedButton = choiceButton(getString(R.string.pattern_strategy_randomized)) {
                patternStrategy = PatternMaskingStrategy.RANDOMIZED
                refreshSelectionViews()
            }.apply {
                layoutParams = stackedChoiceParams()
            }
            addView(patternRandomizedButton)

            patternBurstButton = choiceButton("Короткие всплески и затухание") {
                patternStrategy = PatternMaskingStrategy.BURST_FADE
                refreshSelectionViews()
            }.apply {
                layoutParams = stackedChoiceParams()
            }
            addView(patternBurstButton)

            patternQuietButton = choiceButton("Тихая ротация паттернов") {
                patternStrategy = PatternMaskingStrategy.QUIET_SWEEP
                refreshSelectionViews()
            }.apply {
                layoutParams = stackedChoiceParams()
            }
            addView(patternQuietButton)
        }
    }

    private fun buildAppsCard(): LinearLayout {
        return card(dp(18)).apply {
            addView(label(getString(R.string.apps_title), 16f, "#F3F6FB", true))
            addView(
                label(getString(R.string.apps_subtitle), 12f, "#8091A7", false).apply {
                    setPadding(0, dp(8), 0, dp(10))
                }
            )

            appsSummary = label("", 12f, "#9DB1C6", false).apply {
                setPadding(0, 0, 0, dp(12))
            }
            addView(appsSummary)

            appsToggleButton = Button(this@SettingsActivity).apply {
                isAllCaps = false
                setTextColor(Color.parseColor("#F3F6FB"))
                textSize = 13f
                typeface = Typeface.DEFAULT_BOLD
                background = roundedDrawable("#0E1520", "#243244", 22f, 2)
                setPadding(dp(18), dp(12), dp(18), dp(12))
                setOnClickListener { toggleAppsList() }
            }
            addView(appsToggleButton)

            appsListContainer = LinearLayout(this@SettingsActivity).apply {
                orientation = LinearLayout.VERTICAL
                visibility = View.GONE
                alpha = 0f
            }
            addView(
                appsListContainer,
                LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    topMargin = dp(14)
                }
            )
        }
    }

    private fun buildDisguiseCard(): LinearLayout {
        return card(dp(18)).apply {
            addView(label("Маска приложения", 16f, "#F3F6FB", true))
            addView(
                label(
                    "Можно подготовить новую identity для следующей переустановки APK. Текущая установленная сборка не меняет package name без новой установки.",
                    12f,
                    "#8091A7",
                    false
                ).apply {
                    setPadding(0, dp(8), 0, dp(14))
                }
            )

            addView(label("Имя приложения", 13f, "#9DB1C6", true))
            disguiseNameValue = label("", 14f, "#F3F6FB", false).apply {
                setPadding(0, dp(6), 0, dp(12))
            }
            addView(disguiseNameValue)

            addView(label("Application ID", 13f, "#9DB1C6", true))
            disguisePackageValue = label("", 14f, "#F3F6FB", false).apply {
                setPadding(0, dp(6), 0, dp(12))
            }
            addView(disguisePackageValue)

            addView(label("Команда сборки", 13f, "#9DB1C6", true))
            disguiseCommandValue = label("", 12f, "#7ACAA7", false).apply {
                setPadding(0, dp(6), 0, dp(14))
            }
            addView(disguiseCommandValue)

            val buttons = LinearLayout(this@SettingsActivity).apply {
                orientation = LinearLayout.VERTICAL
            }

            buttons.addView(
                choiceButton("Случайная маска для следующей сборки") {
                    disguiseIdentity = DisguiseIdentityGenerator.randomIdentity()
                    refreshSelectionViews()
                }
            )

            buttons.addView(
                choiceButton("Вернуть Safaty Turtle / safety.turtle") {
                    disguiseIdentity = DisguiseIdentityGenerator.defaultIdentity()
                    refreshSelectionViews()
                }.apply {
                    layoutParams = stackedChoiceParams()
                }
            )

            buttons.addView(
                choiceButton("Скопировать команду сборки") {
                    copyDisguiseCommand(disguiseIdentity)
                }.apply {
                    layoutParams = stackedChoiceParams()
                }
            )

            addView(buttons)
        }
    }

    private fun buildSaveButton(): Button {
        return Button(this).apply {
            text = getString(R.string.save_settings)
            isAllCaps = false
            setTextColor(Color.parseColor("#F3F6FB"))
            textSize = 16f
            typeface = Typeface.DEFAULT_BOLD
            background = roundedDrawable("#111A25", "#27405A", 26f, 2)
            setPadding(dp(18), dp(16), dp(18), dp(16))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(20)
            }
            setOnClickListener { saveSettings() }
        }
    }

    private fun toggleAppsList() {
        if (!appsLoaded) {
            populateApps()
            appsLoaded = true
        }

        appsExpanded = !appsExpanded
        if (appsExpanded) {
            appsListContainer.visibility = View.VISIBLE
            appsListContainer.animate().alpha(1f).setDuration(220).start()
        } else {
            appsListContainer.animate().alpha(0f).setDuration(180).withEndAction {
                appsListContainer.visibility = View.GONE
            }.start()
        }
        refreshSelectionViews()
    }

    private fun populateApps() {
        appsListContainer.removeAllViews()
        appCheckBoxes.clear()
        appScanner.loadLaunchableEntries().forEachIndexed { index, app ->
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                background = roundedDrawable("#0E1520", "#1C2938", 24f, 1)
                setPadding(dp(14), dp(12), dp(14), dp(12))
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    if (index > 0) {
                        topMargin = dp(10)
                    }
                }
            }

            val iconView = ImageView(this).apply {
                setImageDrawable(app.icon)
                layoutParams = LinearLayout.LayoutParams(dp(36), dp(36)).apply {
                    marginEnd = dp(12)
                }
            }

            val textBlock = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }
            textBlock.addView(label(app.label, 14f, "#F3F6FB", true))
            textBlock.addView(
                label(app.packageName, 11f, "#7B8DA3", false).apply {
                    setPadding(0, dp(4), 0, 0)
                }
            )

            val box = CheckBox(this).apply {
                isChecked = app.packageName in selectedPackages
                buttonTintList = ColorStateList.valueOf(Color.parseColor("#5FD4A6"))
                setOnCheckedChangeListener { _, isChecked ->
                    if (isChecked) {
                        selectedPackages += app.packageName
                    } else {
                        selectedPackages -= app.packageName
                    }
                    refreshSelectionViews()
                }
            }

            row.setOnClickListener { box.toggle() }
            row.addView(iconView)
            row.addView(textBlock)
            row.addView(box)
            appsListContainer.addView(row)
            appCheckBoxes += app to box
        }
    }

    private fun refreshSelectionViews() {
        applyChoiceButtonStyle(modeExcludeButton, appRoutingMode == AppRoutingMode.EXCLUDE_SELECTED)
        applyChoiceButtonStyle(modeOnlySelectedButton, appRoutingMode == AppRoutingMode.ONLY_SELECTED)

        applyChoiceButtonStyle(trafficBalancedButton, trafficStrategy == TrafficObfuscationStrategy.BALANCED)
        applyChoiceButtonStyle(trafficCdnButton, trafficStrategy == TrafficObfuscationStrategy.CDN_MIMIC)
        applyChoiceButtonStyle(trafficFragmentedButton, trafficStrategy == TrafficObfuscationStrategy.FRAGMENTED)
        applyChoiceButtonStyle(trafficMobileButton, trafficStrategy == TrafficObfuscationStrategy.MOBILE_MIX)
        applyChoiceButtonStyle(trafficTlsButton, trafficStrategy == TrafficObfuscationStrategy.TLS_BLEND)

        applyChoiceButtonStyle(patternSteadyButton, patternStrategy == PatternMaskingStrategy.STEADY)
        applyChoiceButtonStyle(patternPulseButton, patternStrategy == PatternMaskingStrategy.PULSE)
        applyChoiceButtonStyle(patternRandomizedButton, patternStrategy == PatternMaskingStrategy.RANDOMIZED)
        applyChoiceButtonStyle(patternBurstButton, patternStrategy == PatternMaskingStrategy.BURST_FADE)
        applyChoiceButtonStyle(patternQuietButton, patternStrategy == PatternMaskingStrategy.QUIET_SWEEP)

        appsSummary.text = when (appRoutingMode) {
            AppRoutingMode.EXCLUDE_SELECTED -> getString(R.string.apps_selection_summary_exclude, selectedPackages.size)
            AppRoutingMode.ONLY_SELECTED -> getString(R.string.apps_selection_summary_include, selectedPackages.size)
        }
        appsToggleButton.text = if (appsExpanded) {
            getString(R.string.apps_hide_list)
        } else {
            getString(R.string.apps_pick_button)
        }
        if (::disguiseNameValue.isInitialized) {
            disguiseNameValue.text = disguiseIdentity.appName
        }
        if (::disguisePackageValue.isInitialized) {
            disguisePackageValue.text = disguiseIdentity.applicationId
        }
        if (::disguiseCommandValue.isInitialized) {
            disguiseCommandValue.text = disguiseIdentity.rebuildCommand
        }
    }

    private fun saveSettings() {
        preferences.saveBypassRu(bypassRuCheckBox.isChecked)
        preferences.saveForceServerIpMode(forceServerIpCheckBox.isChecked)
        preferences.saveAppRoutingMode(appRoutingMode)
        preferences.saveExcludedPackages(selectedPackages.toList())
        preferences.saveTrafficObfuscationStrategy(trafficStrategy)
        preferences.savePatternMaskingStrategy(patternStrategy)
        preferences.saveDisguiseIdentity(disguiseIdentity)
        setResult(Activity.RESULT_OK)
        finish()
    }

    private fun copyDisguiseCommand(identity: DisguiseIdentity) {
        val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        clipboard.setPrimaryClip(ClipData.newPlainText("novpn_disguise_build", identity.rebuildCommand))
    }

    private fun card(topMargin: Int): LinearLayout {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#0A1018", "#182432", 34f, 2)
            setPadding(dp(18), dp(18), dp(18), dp(18))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                this.topMargin = topMargin
            }
        }
    }

    private fun choiceButton(text: String, onClick: () -> Unit): Button {
        return Button(this).apply {
            this.text = text
            isAllCaps = false
            setTextColor(Color.parseColor("#F3F6FB"))
            textSize = 13f
            typeface = Typeface.DEFAULT_BOLD
            setPadding(dp(18), dp(14), dp(18), dp(14))
            background = roundedDrawable("#0E1520", "#243244", 24f, 2)
            setOnClickListener { onClick() }
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            )
        }
    }

    private fun stackedChoiceParams(): LinearLayout.LayoutParams {
        return LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT,
            LinearLayout.LayoutParams.WRAP_CONTENT
        ).apply {
            topMargin = dp(10)
        }
    }

    private fun applyChoiceButtonStyle(button: Button, selected: Boolean) {
        button.background = roundedDrawable(
            if (selected) "#153047" else "#0E1520",
            if (selected) "#5FD4A6" else "#243244",
            24f,
            2
        )
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
