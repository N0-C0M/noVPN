package com.novpn.ui

import android.app.Activity
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
                intArrayOf(Color.parseColor("#08111F"), Color.parseColor("#0F1F35"))
            )

            val content = LinearLayout(this@SettingsActivity).apply {
                orientation = LinearLayout.VERTICAL
                setPadding(dp(20), dp(22), dp(20), dp(24))
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
            titleBlock.addView(label("Settings", 26f, "#F4F7FB", true))
            titleBlock.addView(
                label(
                    "Fine tune split tunnel routing and app exclusions.",
                    13f,
                    "#98A9C4",
                    false
                ).apply {
                    setPadding(0, dp(4), 0, 0)
                }
            )

            val closeButton = Button(this@SettingsActivity).apply {
                text = "Close"
                isAllCaps = false
                setTextColor(Color.parseColor("#F4F7FB"))
                background = roundedDrawable("#152740", 18f)
                setPadding(dp(16), dp(10), dp(16), dp(10))
                setOnClickListener { finish() }
            }

            addView(titleBlock)
            addView(closeButton)
        }
    }

    private fun buildRoutingCard(): LinearLayout {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#102038", 24f)
            setPadding(dp(18), dp(18), dp(18), dp(18))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(22)
            }

            addView(label("Routing", 16f, "#F4F7FB", true))
            addView(
                label(
                    "Keep RU traffic direct instead of sending it through the VPN.",
                    12f,
                    "#97ABC7",
                    false
                ).apply {
                    setPadding(0, dp(8), 0, dp(10))
                }
            )

            bypassRuCheckBox = CheckBox(this@SettingsActivity).apply {
                text = "Do not proxy RU traffic"
                isChecked = preferences.isBypassRuEnabled()
                setTextColor(Color.parseColor("#F4F7FB"))
                textSize = 14f
            }
            addView(bypassRuCheckBox)
        }
    }

    private fun buildAppsCard(selectedPackages: Set<String>): LinearLayout {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#102038", 24f)
            setPadding(dp(18), dp(18), dp(18), dp(18))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply {
                topMargin = dp(18)
            }

            addView(label("App exclusions", 16f, "#F4F7FB", true))
            addView(
                label(
                    "Checked apps will stay outside the VPN through addDisallowedApplication().",
                    12f,
                    "#97ABC7",
                    false
                ).apply {
                    setPadding(0, dp(8), 0, dp(14))
                }
            )

            appScanner.loadLaunchableApps().forEach { app ->
                val box = CheckBox(this@SettingsActivity).apply {
                    text = "${app.label} (${app.packageName})"
                    isChecked = app.packageName in selectedPackages
                    setTextColor(Color.parseColor("#F4F7FB"))
                    textSize = 13f
                    setPadding(0, dp(4), 0, dp(4))
                }
                appCheckBoxes += app to box
                addView(box)
            }
        }
    }

    private fun buildSaveButton(): Button {
        return Button(this).apply {
            text = "Save settings"
            isAllCaps = false
            setTextColor(Color.parseColor("#08111F"))
            textSize = 16f
            typeface = Typeface.DEFAULT_BOLD
            background = roundedDrawable("#58E0B5", 22f)
            setPadding(dp(18), dp(14), dp(18), dp(14))
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

    private fun roundedDrawable(fillColor: String, radiusDp: Float): GradientDrawable {
        return GradientDrawable().apply {
            shape = GradientDrawable.RECTANGLE
            cornerRadius = dp(radiusDp)
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
