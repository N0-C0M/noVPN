package com.novpn.ui

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.text.method.ScrollingMovementMethod
import android.util.TypedValue
import android.view.Gravity
import android.widget.Button
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import androidx.activity.ComponentActivity
import com.novpn.vpn.RuntimeLogStore
import java.io.File

class LogsActivity : ComponentActivity() {
    private val logStore by lazy { RuntimeLogStore(this) }
    private lateinit var summaryView: TextView
    private lateinit var appLogView: TextView
    private lateinit var xrayLogView: TextView
    private lateinit var obfuscatorLogView: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(buildContentView())
        renderLogs()
    }

    override fun onResume() {
        super.onResume()
        renderLogs()
    }

    private fun buildContentView(): ScrollView {
        return ScrollView(this).apply {
            background = GradientDrawable(
                GradientDrawable.Orientation.TOP_BOTTOM,
                intArrayOf(Color.parseColor("#02040A"), Color.parseColor("#060B12"))
            )

            val content = LinearLayout(this@LogsActivity).apply {
                orientation = LinearLayout.VERTICAL
                setPadding(dp(20), dp(22), dp(20), dp(28))
            }

            content.addView(buildHeader())
            summaryView = label("", 12f, "#9DB1C6", false).apply {
                setPadding(0, dp(18), 0, 0)
            }
            content.addView(summaryView)
            appLogView = buildLogCard(content, "Application runtime")
            xrayLogView = buildLogCard(content, "Xray")
            obfuscatorLogView = buildLogCard(content, "Obfuscator")

            addView(content)
        }
    }

    private fun buildHeader(): LinearLayout {
        return LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL

            val titleBlock = LinearLayout(this@LogsActivity).apply {
                orientation = LinearLayout.VERTICAL
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }
            titleBlock.addView(label("Runtime logs", 26f, "#F3F6FB", true))
            titleBlock.addView(
                label(
                    "Detailed app, Xray and obfuscator logs from the current device runtime.",
                    13f,
                    "#8091A7",
                    false
                ).apply { setPadding(0, dp(4), 0, 0) }
            )

            val controls = LinearLayout(this@LogsActivity).apply {
                orientation = LinearLayout.HORIZONTAL
            }
            controls.addView(actionButton("Refresh") { renderLogs() }.apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { marginEnd = dp(8) }
            })
            controls.addView(actionButton("Copy all") { copyAllLogs() }.apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { marginEnd = dp(8) }
            })
            controls.addView(actionButton("Close") { finish() })

            addView(titleBlock)
            addView(controls)
        }
    }

    private fun buildLogCard(parent: LinearLayout, title: String): TextView {
        val card = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = roundedDrawable("#0A1018", "#182432", 34f, 2)
            setPadding(dp(18), dp(18), dp(18), dp(18))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { topMargin = dp(18) }
        }
        card.addView(label(title, 16f, "#F3F6FB", true))
        val value = TextView(this).apply {
            setTextColor(Color.parseColor("#9DB1C6"))
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 12f)
            typeface = Typeface.MONOSPACE
            setPadding(0, dp(10), 0, 0)
            movementMethod = ScrollingMovementMethod()
        }
        card.addView(value)
        parent.addView(card)
        return value
    }

    private fun renderLogs() {
        val logsDir = logStore.logsDirectory()
        summaryView.text = "Directory: ${logsDir.absolutePath}"
        appLogView.text = formatLog(logStore.appLogFile())
        xrayLogView.text = formatLog(File(logsDir, "xray.log"))
        obfuscatorLogView.text = formatLog(File(logsDir, "obfuscator.log"))
    }

    private fun formatLog(file: File): String {
        val content = logStore.readFull(file)
        return if (content.isBlank()) "No log entries yet." else content
    }

    private fun copyAllLogs() {
        val logsDir = logStore.logsDirectory()
        val payload = buildString {
            appendLine("=== app.log ===")
            appendLine(formatLog(logStore.appLogFile()))
            appendLine()
            appendLine("=== xray.log ===")
            appendLine(formatLog(File(logsDir, "xray.log")))
            appendLine()
            appendLine("=== obfuscator.log ===")
            appendLine(formatLog(File(logsDir, "obfuscator.log")))
        }
        val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        clipboard.setPrimaryClip(ClipData.newPlainText("novpn_runtime_logs", payload))
    }

    private fun actionButton(text: String, onClick: () -> Unit): Button {
        return Button(this).apply {
            this.text = text
            isAllCaps = false
            setTextColor(Color.parseColor("#F3F6FB"))
            background = roundedDrawable("#0E1520", "#243244", 22f, 2)
            setPadding(dp(16), dp(12), dp(16), dp(12))
            setOnClickListener { onClick() }
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

    private fun roundedDrawable(fillColor: String, strokeColor: String, radiusDp: Float, strokeWidthDp: Int): GradientDrawable {
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

    companion object {
        fun createIntent(context: Context): Intent = Intent(context, LogsActivity::class.java)
    }
}
