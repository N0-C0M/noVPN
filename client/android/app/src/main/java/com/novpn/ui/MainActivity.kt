package com.novpn.ui

import android.app.Activity
import android.net.VpnService
import android.os.Bundle
import android.widget.Button
import android.widget.CheckBox
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.core.content.ContextCompat
import com.novpn.data.InstalledApp
import com.novpn.vpn.NoVpnService


class MainActivity : ComponentActivity() {
    private val viewModel by viewModels<TunnelViewModel>()
    private val appCheckBoxes = mutableListOf<Pair<InstalledApp, CheckBox>>()
    private lateinit var status: TextView

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            startVpnRuntime()
        } else {
            status.text = "VPN permission was not granted"
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val root = ScrollView(this)
        val content = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(32, 32, 32, 32)
        }

        val bypassRu = CheckBox(this).apply {
            text = "Bypass RU traffic"
            isChecked = true
            setOnCheckedChangeListener { _, checked -> viewModel.setBypassRu(checked) }
        }

        val appsInfo = TextView(this).apply {
            text = "Excluded packages will bypass VPN through addDisallowedApplication()."
        }

        val appsContainer = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
        }
        populateAppCheckboxes(appsContainer, viewModel.state.value.installedApps.take(8))

        val generateButton = Button(this).apply {
            text = "Generate local config.json"
            setOnClickListener {
                syncExcludedPackages()
                viewModel.generateConfig()
                status.text = buildStatus(viewModel.state.value)
            }
        }

        val startButton = Button(this).apply {
            text = "Start VPN runtime"
            setOnClickListener {
                syncExcludedPackages()
                val prepareIntent = VpnService.prepare(this@MainActivity)
                if (prepareIntent != null) {
                    vpnPermissionLauncher.launch(prepareIntent)
                } else {
                    startVpnRuntime()
                }
            }
        }

        val stopButton = Button(this).apply {
            text = "Stop VPN runtime"
            setOnClickListener {
                startService(NoVpnService.stopIntent(this@MainActivity))
                viewModel.markRuntimeStopped()
                status.text = buildStatus(viewModel.state.value)
            }
        }

        status = TextView(this).apply {
            text = buildStatus(viewModel.state.value)
        }

        content.addView(bypassRu)
        content.addView(appsInfo)
        content.addView(appsContainer)
        content.addView(generateButton)
        content.addView(startButton)
        content.addView(stopButton)
        content.addView(status)
        root.addView(content)
        setContentView(root)
    }

    private fun buildStatus(state: TunnelState): String {
        val previewApps = state.installedApps.take(5).joinToString { it.packageName }
        return buildString {
            appendLine("Installed apps preview: ${state.installedApps.size}")
            appendLine(previewApps)
            appendLine("Config path: ${state.generatedConfigPath.ifEmpty { "not generated" }}")
            appendLine("Runtime: ${state.runtimeStatus}")
        }
    }

    private fun populateAppCheckboxes(container: LinearLayout, apps: List<InstalledApp>) {
        container.removeAllViews()
        appCheckBoxes.clear()
        apps.forEach { app ->
            val checkbox = CheckBox(this).apply {
                text = "${app.label} (${app.packageName})"
            }
            appCheckBoxes += app to checkbox
            container.addView(checkbox)
        }
    }

    private fun syncExcludedPackages() {
        val selected = appCheckBoxes
            .filter { (_, checkbox) -> checkbox.isChecked }
            .map { (app, _) -> app.packageName }
        viewModel.setExcludedPackages(selected)
    }

    private fun startVpnRuntime() {
        val request = viewModel.buildRuntimeRequest()
        viewModel.generateConfig()
        val configPath = viewModel.state.value.generatedConfigPath
        val intent = NoVpnService.startIntent(
            context = this,
            bypassRu = request.bypassRu,
            excludedPackages = request.excludedPackages
        )
        ContextCompat.startForegroundService(this, intent)
        viewModel.markRuntimeStarted(configPath)
        status.text = buildStatus(viewModel.state.value)
    }
}
