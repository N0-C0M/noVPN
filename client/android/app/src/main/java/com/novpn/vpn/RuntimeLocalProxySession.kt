package com.novpn.vpn

object RuntimeLocalProxySession {
    @Volatile
    private var current: RuntimeLocalProxyConfig? = null

    fun update(config: RuntimeLocalProxyConfig?) {
        current = config
    }

    fun current(): RuntimeLocalProxyConfig? = current
}
