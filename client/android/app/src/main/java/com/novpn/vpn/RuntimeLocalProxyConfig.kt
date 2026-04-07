package com.novpn.vpn

import java.net.ServerSocket
import java.security.SecureRandom

data class RuntimeLocalProxyConfig(
    val listenHost: String,
    val socksPort: Int,
    val username: String,
    val password: String,
    val udpEnabled: Boolean
)

object RuntimeLocalProxyFactory {
    private const val LOOPBACK_HOST = "127.0.0.1"
    private const val USERNAME_PREFIX = "novpn_"
    private const val PASSWORD_LENGTH = 24
    private const val USERNAME_TOKEN_LENGTH = 12
    private const val ALPHABET = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

    private val secureRandom = SecureRandom()

    fun create(): RuntimeLocalProxyConfig {
        return RuntimeLocalProxyConfig(
            listenHost = LOOPBACK_HOST,
            socksPort = reserveTcpPort(),
            username = USERNAME_PREFIX + randomToken(USERNAME_TOKEN_LENGTH),
            password = randomToken(PASSWORD_LENGTH),
            udpEnabled = false
        )
    }

    private fun reserveTcpPort(): Int {
        return ServerSocket(0).use { socket ->
            socket.reuseAddress = false
            socket.localPort
        }
    }

    private fun randomToken(length: Int): String {
        val lastIndex = ALPHABET.length
        return buildString(length) {
            repeat(length) {
                append(ALPHABET[secureRandom.nextInt(lastIndex)])
            }
        }
    }
}
