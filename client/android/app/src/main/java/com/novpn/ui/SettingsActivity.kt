package com.novpn.ui

import android.app.Activity
import android.app.AppOpsManager
import android.content.Context
import android.content.Intent
import android.content.res.ColorStateList
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Build
import android.os.Bundle
import android.os.Process
import android.provider.Settings
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.CheckBox
import android.widget.FrameLayout
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.Switch
import android.widget.TextView
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.lifecycle.lifecycleScope
import com.novpn.R
import com.novpn.data.AppRoutingMode
import com.novpn.data.ClientPreferences
import com.novpn.data.PatternMaskingStrategy
import com.novpn.data.TrafficObfuscationStrategy
import com.novpn.split.InstalledAppEntry
import com.novpn.split.InstalledAppsScanner
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class SettingsActivity : ComponentActivity() {
    private val preferences by lazy { ClientPreferences(this) }
    private val appScanner by lazy { InstalledAppsScanner(this) }

    private val selectedPackages = linkedSetOf<String>()
    private val trafficButtons = mutableMapOf<TrafficObfuscationStrategy, Button>()
    private val patternButtons = mutableMapOf<PatternMaskingStrategy, Button>()

    private lateinit var bypassRuSwitch: Switch
    private lateinit var forceServerIpSwitch: Switch
    private lateinit var autoScreenToggleSwitch: Switch
    private lateinit var whitelistForegroundSwitch: Switch
    private lateinit var usageAccessSummary: TextView
    private lateinit var defaultWhitelistSwitch: Switch
    private lateinit var modeExcludeButton: Button
    private lateinit var modeOnlySelectedButton: Button
    private lateinit var appsSummary: TextView
    private lateinit var appsToggleButton: Button
    private lateinit var appsListContainer: LinearLayout

    private var appRoutingMode = AppRoutingMode.EXCLUDE_SELECTED
    private var defaultWhitelistEnabled = true
    private var trafficStrategy = TrafficObfuscationStrategy.BALANCED
    private var patternStrategy = PatternMaskingStrategy.STEADY
    private var appsExpanded = false
    private var appsLoaded = false
    private var appsLoading = false
    private var appEntries: List<InstalledAppEntry> = emptyList()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        selectedPackages += preferences.excludedPackages()
        defaultWhitelistEnabled = preferences.isDefaultWhitelistEnabled()
        appRoutingMode = preferences.appRoutingMode()
        if (defaultWhitelistEnabled) {
            appRoutingMode = AppRoutingMode.ONLY_SELECTED
            if (selectedPackages.isEmpty()) {
                selectedPackages += preferences.defaultWhitelistPackages()
            }
        }
        trafficStrategy = preferences.trafficObfuscationStrategy()
        patternStrategy = preferences.patternMaskingStrategy()

        setContentView(buildContentView())
        refreshViews()
    }

    override fun onResume() {
        super.onResume()
        refreshViews()
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

        content.addView(buildHeader())
        content.addView(buildBehaviorCard())
        content.addView(buildRoutingCard())
        content.addView(buildObfuscationCard())

        scroll.addView(content)
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

            addView(
                label("Настройки", 28f, "#F5F7FA", true).apply {
                    layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                }
            )

            addView(
                Button(this@SettingsActivity).apply {
                    text = "Готово"
                    isAllCaps = false
                    setTextColor(Color.parseColor("#D9E2F2"))
                    textSize = 13f
                    typeface = Typeface.DEFAULT_BOLD
                    background = roundedDrawable("#11151D", "#232C3C", 12f, 1)
                    setPadding(dp(12), dp(8), dp(12), dp(8))
                    setOnClickListener { finish() }
                }
            )
        }
    }

    private fun buildBehaviorCard(): View {
        return card(dp(18)).apply {
            addView(label("Поведение VPN", 16f, "#F5F7FA", true))
            addView(
                label("Автоматические сценарии и базовые параметры подключения", 12f, "#8A94A6", false).apply {
                    setPadding(0, dp(6), 0, dp(12))
                }
            )

            bypassRuSwitch = settingSwitch(
                title = getString(R.string.do_not_proxy_ru),
                checked = preferences.isBypassRuEnabled()
            ) {
                persistSettings()
            }
            addView(bypassRuSwitch)

            forceServerIpSwitch = settingSwitch(
                title = "Использовать IP сервера, если домен недоступен",
                checked = preferences.forceServerIpMode(),
                topMargin = dp(8)
            ) {
                persistSettings()
            }
            addView(forceServerIpSwitch)

            autoScreenToggleSwitch = settingSwitch(
                title = "Авто-пауза VPN при выключении экрана",
                checked = preferences.isScreenOffAutoToggleEnabled(),
                topMargin = dp(8)
            ) {
                persistSettings()
            }
            addView(autoScreenToggleSwitch)

            whitelistForegroundSwitch = settingSwitch(
                title = "Запускать VPN только для активного whitelist-приложения",
                checked = preferences.isWhitelistForegroundModeEnabled(),
                topMargin = dp(8)
            ) {
                if (whitelistForegroundSwitch.isChecked) {
                    appRoutingMode = AppRoutingMode.ONLY_SELECTED
                    if (selectedPackages.isEmpty()) {
                        selectedPackages += preferences.defaultWhitelistPackages()
                    }
                }
                persistSettings()
                refreshViews()
            }
            addView(whitelistForegroundSwitch)

            usageAccessSummary = label("", 12f, "#8A94A6", false).apply {
                setPadding(0, dp(10), 0, 0)
            }
            addView(usageAccessSummary)

            addView(
                choiceButton("Открыть доступ к статистике использования") {
                    startActivity(Intent(Settings.ACTION_USAGE_ACCESS_SETTINGS))
                }.apply {
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.MATCH_PARENT,
                        LinearLayout.LayoutParams.WRAP_CONTENT
                    ).apply {
                        topMargin = dp(10)
                    }
                }
            )
        }
    }

    private fun buildRoutingCard(): View {
        return card(dp(14)).apply {
            addView(label("Маршрутизация приложений", 16f, "#F5F7FA", true))
            addView(
                label("Простой режим: отмеченные приложения работают через VPN", 12f, "#8A94A6", false).apply {
                    setPadding(0, dp(6), 0, dp(12))
                }
            )

            defaultWhitelistSwitch = settingSwitch(
                title = "Использовать базовый whitelist популярных приложений",
                checked = defaultWhitelistEnabled
            ) {
                applyChange {
                    defaultWhitelistEnabled = defaultWhitelistSwitch.isChecked
                    if (defaultWhitelistEnabled) {
                        appRoutingMode = AppRoutingMode.ONLY_SELECTED
                        if (selectedPackages.isEmpty()) {
                            selectedPackages += preferences.defaultWhitelistPackages()
                        }
                    }
                }
            }
            addView(defaultWhitelistSwitch)

            val modeGroup = LinearLayout(this@SettingsActivity).apply {
                orientation = LinearLayout.VERTICAL
                setPadding(0, dp(12), 0, 0)
            }
            modeExcludeButton = choiceButton(getString(R.string.apps_mode_exclude)) {
                if (whitelistForegroundSwitch.isChecked || defaultWhitelistEnabled) {
                    return@choiceButton
                }
                applyChange {
                    appRoutingMode = AppRoutingMode.EXCLUDE_SELECTED
                }
            }
            modeGroup.addView(modeExcludeButton)

            modeOnlySelectedButton = choiceButton(getString(R.string.apps_mode_only_selected)) {
                applyChange {
                    appRoutingMode = AppRoutingMode.ONLY_SELECTED
                }
            }.apply {
                layoutParams = stackedButtonParams()
            }
            modeGroup.addView(modeOnlySelectedButton)
            addView(modeGroup)

            appsSummary = label("", 12f, "#8A94A6", false).apply {
                setPadding(0, dp(12), 0, 0)
            }
            addView(appsSummary)

            appsToggleButton = choiceButton("Выбрать приложения") {
                toggleAppsList()
            }.apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    topMargin = dp(10)
                }
            }
            addView(appsToggleButton)

            appsListContainer = LinearLayout(this@SettingsActivity).apply {
                orientation = LinearLayout.VERTICAL
                visibility = View.GONE
            }
            addView(
                appsListContainer,
                LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    topMargin = dp(10)
                }
            )
        }
    }

    private fun buildObfuscationCard(): View {
        return card(dp(14)).apply {
            addView(label("Маскировка", 16f, "#F5F7FA", true))
            addView(
                label("Оставлено в простом виде: выбирайте профиль трафика и паттерна", 12f, "#8A94A6", false).apply {
                    setPadding(0, dp(6), 0, dp(12))
                }
            )

            addView(label("Профиль трафика", 13f, "#D9E2F2", true))
            trafficButtons.clear()
            addTrafficButton(TrafficObfuscationStrategy.BALANCED, getString(R.string.traffic_strategy_balanced), first = true)
            addTrafficButton(TrafficObfuscationStrategy.CDN_MIMIC, getString(R.string.traffic_strategy_cdn), first = false)
            addTrafficButton(TrafficObfuscationStrategy.FRAGMENTED, getString(R.string.traffic_strategy_fragmented), first = false)
            addTrafficButton(TrafficObfuscationStrategy.MOBILE_MIX, "Mobile browser blend", first = false)
            addTrafficButton(TrafficObfuscationStrategy.TLS_BLEND, "Mixed TLS profile", first = false)

            addView(label("Профиль паттерна", 13f, "#D9E2F2", true).apply {
                setPadding(0, dp(14), 0, 0)
            })
            patternButtons.clear()
            addPatternButton(PatternMaskingStrategy.STEADY, getString(R.string.pattern_strategy_steady), first = true)
            addPatternButton(PatternMaskingStrategy.PULSE, getString(R.string.pattern_strategy_pulse), first = false)
            addPatternButton(PatternMaskingStrategy.RANDOMIZED, getString(R.string.pattern_strategy_randomized), first = false)
            addPatternButton(PatternMaskingStrategy.BURST_FADE, "Short bursts and fade", first = false)
            addPatternButton(PatternMaskingStrategy.QUIET_SWEEP, "Quiet pattern rotation", first = false)
        }
    }

    private fun LinearLayout.addTrafficButton(
        strategy: TrafficObfuscationStrategy,
        title: String,
        first: Boolean
    ) {
        val button = choiceButton(title) {
            applyChange {
                trafficStrategy = strategy
            }
        }
        if (!first) {
            button.layoutParams = stackedButtonParams()
        }
        trafficButtons[strategy] = button
        addView(button)
    }

    private fun LinearLayout.addPatternButton(
        strategy: PatternMaskingStrategy,
        title: String,
        first: Boolean
    ) {
        val button = choiceButton(title) {
            applyChange {
                patternStrategy = strategy
            }
        }
        if (!first) {
            button.layoutParams = stackedButtonParams()
        }
        patternButtons[strategy] = button
        addView(button)
    }

    private fun toggleAppsList() {
        appsExpanded = !appsExpanded
        if (appsExpanded) {
            appsListContainer.visibility = View.VISIBLE
            if (!appsLoaded && !appsLoading) {
                loadApps()
            }
        } else {
            appsListContainer.visibility = View.GONE
        }
        refreshViews()
    }

    private fun loadApps() {
        appsLoading = true
        appsListContainer.removeAllViews()
        appsListContainer.addView(label("Загрузка списка приложений...", 12f, "#8A94A6", false))

        lifecycleScope.launch {
            val loadedEntries = withContext(Dispatchers.Default) {
                appScanner.loadLaunchableEntries(limit = Int.MAX_VALUE)
            }
            appEntries = loadedEntries
            appsLoaded = true
            appsLoading = false
            renderAppsList()
            refreshViews()
        }
    }

    private fun renderAppsList() {
        appsListContainer.removeAllViews()

        if (appEntries.isEmpty()) {
            appsListContainer.addView(label("Приложения не найдены", 12f, "#8A94A6", false))
            return
        }

        appEntries.forEachIndexed { index, app ->
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                background = roundedDrawable("#11151D", "#232C3C", 14f, 1)
                setPadding(dp(12), dp(10), dp(12), dp(10))
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    if (index > 0) {
                        topMargin = dp(8)
                    }
                }
            }

            val icon = ImageView(this).apply {
                setImageDrawable(app.icon)
                layoutParams = LinearLayout.LayoutParams(dp(30), dp(30)).apply {
                    marginEnd = dp(10)
                }
            }

            val info = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }
            info.addView(label(app.label, 13f, "#F5F7FA", true))
            info.addView(label(app.packageName, 11f, "#8A94A6", false).apply {
                setPadding(0, dp(3), 0, 0)
            })

            val checkBox = CheckBox(this).apply {
                isChecked = app.packageName in selectedPackages
                buttonTintList = ColorStateList.valueOf(Color.parseColor("#A7F259"))
                setOnCheckedChangeListener { _, checked ->
                    if (checked) {
                        selectedPackages += app.packageName
                    } else {
                        selectedPackages -= app.packageName
                    }
                    persistSettings()
                    refreshViews()
                }
            }

            row.setOnClickListener { checkBox.toggle() }
            row.addView(icon)
            row.addView(info)
            row.addView(checkBox)
            appsListContainer.addView(row)
        }
    }

    private fun refreshViews() {
        defaultWhitelistSwitch.isChecked = defaultWhitelistEnabled

        val usageGranted = isUsageAccessGranted()
        usageAccessSummary.text = if (usageGranted) {
            "Доступ к статистике использования: включен"
        } else {
            "Доступ к статистике использования: выключен"
        }
        usageAccessSummary.setTextColor(
            Color.parseColor(if (usageGranted) "#89D5A0" else "#E8B38B")
        )

        applyChoiceStyle(modeExcludeButton, appRoutingMode == AppRoutingMode.EXCLUDE_SELECTED)
        applyChoiceStyle(modeOnlySelectedButton, appRoutingMode == AppRoutingMode.ONLY_SELECTED)

        val blockExcludeMode = defaultWhitelistEnabled || whitelistForegroundSwitch.isChecked
        modeExcludeButton.isEnabled = !blockExcludeMode

        trafficButtons.forEach { (strategy, button) ->
            applyChoiceStyle(button, trafficStrategy == strategy)
        }
        patternButtons.forEach { (strategy, button) ->
            applyChoiceStyle(button, patternStrategy == strategy)
        }

        appsSummary.text = when {
            whitelistForegroundSwitch.isChecked -> {
                "Режим по foreground whitelist включен. Выбрано приложений: ${selectedPackages.size}."
            }

            defaultWhitelistEnabled -> {
                "Базовый whitelist включен. Выбрано приложений: ${selectedPackages.size}."
            }

            appRoutingMode == AppRoutingMode.EXCLUDE_SELECTED -> {
                getString(R.string.apps_selection_summary_exclude, selectedPackages.size)
            }

            else -> {
                getString(R.string.apps_selection_summary_include, selectedPackages.size)
            }
        }

        appsToggleButton.text = if (appsExpanded) {
            "Скрыть список приложений"
        } else {
            "Выбрать приложения"
        }
    }

    private fun persistSettings() {
        preferences.saveBypassRu(bypassRuSwitch.isChecked)
        preferences.saveForceServerIpMode(forceServerIpSwitch.isChecked)
        preferences.saveDefaultWhitelistEnabled(defaultWhitelistEnabled)
        preferences.saveAppRoutingMode(appRoutingMode)
        preferences.saveExcludedPackages(selectedPackages.toList())
        preferences.saveTrafficObfuscationStrategy(trafficStrategy)
        preferences.savePatternMaskingStrategy(patternStrategy)
        preferences.saveScreenOffAutoToggleEnabled(autoScreenToggleSwitch.isChecked)
        preferences.saveWhitelistForegroundModeEnabled(whitelistForegroundSwitch.isChecked)
        setResult(Activity.RESULT_OK)
    }

    private fun applyChange(change: () -> Unit) {
        change()
        persistSettings()
        refreshViews()
    }

    private fun buildBottomNav(): View {
        return LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER
            background = roundedDrawable("#0F131A", "#202838", 20f, 1)
            setPadding(dp(6), dp(6), dp(6), dp(6))

            addView(
                navButton("Главная", active = false) {
                    finish()
                },
                navLayoutParams(end = dp(4))
            )

            addView(
                navButton("Магазин", active = false) {
                    Toast.makeText(this@SettingsActivity, "Будет доступно позже", Toast.LENGTH_SHORT).show()
                },
                navLayoutParams(start = dp(2), end = dp(2))
            )

            addView(
                navButton("Настройки", active = true) {
                    // current screen
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

    private fun settingSwitch(
        title: String,
        checked: Boolean,
        topMargin: Int = 0,
        onChange: () -> Unit
    ): Switch {
        return Switch(this).apply {
            text = title
            isChecked = checked
            setTextColor(Color.parseColor("#F5F7FA"))
            textSize = 13f
            trackTintList = ColorStateList.valueOf(Color.parseColor("#434D5E"))
            thumbTintList = ColorStateList.valueOf(Color.parseColor("#E5EBF4"))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                this.topMargin = topMargin
            }
            setOnCheckedChangeListener { _, _ -> onChange() }
        }
    }

    private fun card(topMargin: Int): LinearLayout {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#0D1118", "#1F2735", 20f, 1)
            setPadding(dp(16), dp(16), dp(16), dp(16))
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
            textSize = 13f
            typeface = Typeface.DEFAULT_BOLD
            setTextColor(Color.parseColor("#D9E2F2"))
            background = roundedDrawable("#11151D", "#232C3C", 12f, 1)
            setPadding(dp(12), dp(10), dp(12), dp(10))
            setOnClickListener { onClick() }
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            )
        }
    }

    private fun stackedButtonParams(): LinearLayout.LayoutParams {
        return LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT,
            LinearLayout.LayoutParams.WRAP_CONTENT
        ).apply {
            topMargin = dp(8)
        }
    }

    private fun applyChoiceStyle(button: Button, selected: Boolean) {
        button.background = roundedDrawable(
            fillColor = if (selected) "#1A2230" else "#11151D",
            strokeColor = if (selected) "#4E6A8C" else "#232C3C",
            radiusDp = 12f,
            strokeWidthDp = 1
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

    private fun isUsageAccessGranted(): Boolean {
        val appOpsManager = getSystemService(Context.APP_OPS_SERVICE) as AppOpsManager
        val mode = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            appOpsManager.unsafeCheckOpNoThrow(
                AppOpsManager.OPSTR_GET_USAGE_STATS,
                Process.myUid(),
                packageName
            )
        } else {
            @Suppress("DEPRECATION")
            appOpsManager.checkOpNoThrow(
                AppOpsManager.OPSTR_GET_USAGE_STATS,
                Process.myUid(),
                packageName
            )
        }
        return mode == AppOpsManager.MODE_ALLOWED
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
