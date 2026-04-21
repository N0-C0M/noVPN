package com.novpn.vpn

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class VpnScreenStateReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        val controller = VpnScreenAutomationController(context)
        when (intent?.action) {
            Intent.ACTION_SCREEN_OFF -> controller.handleScreenOff()
            Intent.ACTION_USER_PRESENT -> controller.resumeIfNeeded()
        }
    }
}
