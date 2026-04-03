package com.novpn.ui

import android.os.Bundle
import android.widget.Button
import android.widget.CheckBox
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import androidx.activity.ComponentActivity
import androidx.activity.viewModels

class MainActivity : ComponentActivity() {
    private val viewModel by viewModels<TunnelViewModel>()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val root = ScrollView(this)
        val content = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(32, 32, 32, 32)
        }

        val bypassRu = CheckBox(this).apply {
            text = "Не проксировать РФ"
            isChecked = true
            setOnCheckedChangeListener { _, checked -> viewModel.setBypassRu(checked) }
        }

        val appsInfo = TextView(this).apply {
            text = "APK exclusions are managed through VpnService.addDisallowedApplication()."
        }

        val generateButton = Button(this).apply {
            text = "Generate local config.json"
            setOnClickListener {
                viewModel.generateConfig()
                val state = viewModel.state.value
                status.text = buildStatus(state)
            }
        }

        status = TextView(this).apply {
            text = buildStatus(viewModel.state.value)
        }

        content.addView(bypassRu)
        content.addView(appsInfo)
        content.addView(generateButton)
        content.addView(status)
        root.addView(content)
        setContentView(root)
    }

    private lateinit var status: TextView

    private fun buildStatus(state: TunnelState): String {
        val previewApps = state.installedApps.take(5).joinToString { it.packageName }
        return buildString {
            appendLine("Installed apps preview: ${state.installedApps.size}")
            appendLine(previewApps)
            appendLine("Config path: ${state.generatedConfigPath.ifEmpty { "not generated" }}")
        }
    }
}
