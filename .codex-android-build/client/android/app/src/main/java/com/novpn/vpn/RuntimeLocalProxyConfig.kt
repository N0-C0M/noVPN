package com.novpn.vpn

import java.net.ServerSocket
import java.security.SecureRandom
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
    private const val LOOPBACK_PREFIX = "127"
    private const val LOOPBACK_MIN_OCTET = 1
    private const val LOOPBACK_MAX_OCTET = 254
    private val secureRandom = SecureRandom()

    fun create(): RuntimeLocalProxyConfig {
        return createProtected()
    }

    fun createProtected(udpEnabled: Boolean = false): RuntimeLocalProxyConfig {
        return RuntimeLocalProxyConfig(
            listenHost = randomLoopbackHost(),
            socksPort = reserveTcpPort(),
            username = randomToken("u"),
            password = randomToken("p"),
            udpEnabled = udpEnabled
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

    private fun randomLoopbackHost(): String {
        val octet2 = secureRandom.nextInt(LOOPBACK_MAX_OCTET - LOOPBACK_MIN_OCTET + 1) + LOOPBACK_MIN_OCTET
        val octet3 = secureRandom.nextInt(LOOPBACK_MAX_OCTET - LOOPBACK_MIN_OCTET + 1) + LOOPBACK_MIN_OCTET
        val octet4 = secureRandom.nextInt(LOOPBACK_MAX_OCTET - LOOPBACK_MIN_OCTET + 1) + LOOPBACK_MIN_OCTET
        return "$LOOPBACK_PREFIX.$octet2.$octet3.$octet4"
    }
}
