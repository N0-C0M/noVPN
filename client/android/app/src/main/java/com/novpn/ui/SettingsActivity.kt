package com.novpn.ui

import android.app.Activity
import android.content.res.ColorStateList
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.util.TypedValue
import android.view.Gravity
import android.widget.Button
import android.widget.CheckBox
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import androidx.activity.ComponentActivity
import com.novpn.R
import com.novpn.data.ClientPreferences
import com.novpn.data.InstalledApp
import com.novpn.split.InstalledAppsScanner

class SettingsActivity : ComponentActivity() {
    private val preferences by lazy { ClientPreferences(this) }
    private val appScanner by lazy { InstalledAppsScanner(this) }
    private val appCheckBoxes = mutableListOf<Pair<InstalledApp, CheckBox>>()
    private lateinit var bypassRuCheckBox: CheckBox

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(buildContentView())
    }

    private fun buildContentView(): ScrollView {
        val selectedPackages = preferences.excludedPackages().toSet()

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
            content.addView(buildAppsCard(selectedPackages))
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
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#0A1018", "#182432", 34f, 2)
            setPadding(dp(18), dp(18), dp(18), dp(18))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(24)
            }

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
        }
    }

    private fun buildAppsCard(selectedPackages: Set<String>): LinearLayout {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#0A1018", "#182432", 34f, 2)
            setPadding(dp(18), dp(18), dp(18), dp(18))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(18)
            }

            addView(label(getString(R.string.apps_title), 16f, "#F3F6FB", true))
            addView(
                label(getString(R.string.apps_subtitle), 12f, "#8091A7", false).apply {
                    setPadding(0, dp(8), 0, dp(14))
                }
            )

            appScanner.loadLaunchableApps().forEach { app ->
                val box = CheckBox(this@SettingsActivity).apply {
                    text = "${app.label} (${app.packageName})"
                    isChecked = app.packageName in selectedPackages
                    setTextColor(Color.parseColor("#F3F6FB"))
                    textSize = 13f
                    buttonTintList = ColorStateList.valueOf(Color.parseColor("#5FD4A6"))
                    setPadding(0, dp(4), 0, dp(4))
                }
                appCheckBoxes += app to box
                addView(box)
            }
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

    private fun saveSettings() {
        val selectedPackages = appCheckBoxes
            .filter { (_, box) -> box.isChecked }
            .map { (app, _) -> app.packageName }

        preferences.saveBypassRu(bypassRuCheckBox.isChecked)
        preferences.saveExcludedPackages(selectedPackages)
        setResult(Activity.RESULT_OK)
        finish()
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
