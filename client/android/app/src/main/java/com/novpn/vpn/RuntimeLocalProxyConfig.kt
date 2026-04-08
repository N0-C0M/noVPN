package com.novpn.vpn

import java.net.ServerSocket
import java.util.UUID

data class RuntimeLocalProxyConfig(
    val listenHost: String,
    val socksPort: Int,
    val username: String,
    val password: String,
    val udpEnabled: Boolean
) {
    fun socksUrl(): String {
        return if (username.isBlank() || password.isBlank()) {
            "socks5://$listenHost:$socksPort"
        } else {
            "socks5://$username:$password@$listenHost:$socksPort"
        }
    }
}

object RuntimeLocalProxyFactory {
    private const val LOOPBACK_HOST = "127.0.0.1"

    fun create(): RuntimeLocalProxyConfig {
        return createOpen()
    }

    fun createOpen(): RuntimeLocalProxyConfig {
        return RuntimeLocalProxyConfig(
            listenHost = LOOPBACK_HOST,
            socksPort = reserveTcpPort(),
            username = "",
            password = "",
            udpEnabled = false
        )
    }

    fun createProtected(): RuntimeLocalProxyConfig {
        return RuntimeLocalProxyConfig(
            listenHost = LOOPBACK_HOST,
            socksPort = reserveTcpPort(),
            username = randomToken("u"),
            password = randomToken("p"),
            udpEnabled = false
        )
    }

    private fun reserveTcpPort(): Int {
        return ServerSocket(0).use { socket ->
            socket.reuseAddress = false
            socket.localPort
        }
    }

    private fun randomToken(prefix: String): String {
        return prefix + UUID.randomUUID().toString().replace("-", "")
    }
}
