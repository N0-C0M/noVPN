package com.novpn.vpn

import android.os.ParcelFileDescriptor
import java.net.Socket
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.TimeUnit

class Tun2ProxyBridge {
    init {
        ensureNativeLoaded()
    }

    private val executor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "novpn-tun2proxy").apply { isDaemon = true }
    }
    private var task: Future<*>? = null

    fun start(tunnel: ParcelFileDescriptor, proxy: RuntimeLocalProxyConfig, mtu: Int) {
        stop()

        val proxyUrl = proxy.socksUrl()
        val detachedFd = ParcelFileDescriptor.dup(tunnel.fileDescriptor).detachFd()
        task = executor.submit {
            nativeRunWithFd(
                proxyUrl = proxyUrl,
                tunFd = detachedFd,
                mtu = mtu,
                dnsStrategy = DnsStrategy.OVER_TCP.value,
                verbosity = Verbosity.INFO.value
            )
        }
    }

    fun stop() {
        nativeStop()
        task?.cancel(true)
        task = null
    }

    fun waitForLocalProxy(proxy: RuntimeLocalProxyConfig, timeoutSeconds: Long = 10) {
        val deadlineNanos = System.nanoTime() + TimeUnit.SECONDS.toNanos(timeoutSeconds)
        while (System.nanoTime() < deadlineNanos) {
            runCatching {
                Socket(proxy.listenHost, proxy.socksPort).use { return }
            }
            Thread.sleep(100)
        }
        throw IllegalStateException("Local Xray SOCKS bridge did not become ready in time.")
    }

    private external fun nativeRunWithFd(
        proxyUrl: String,
        tunFd: Int,
        mtu: Int,
        dnsStrategy: Int,
        verbosity: Int
    ): Int

    private external fun nativeStop(): Int

    private enum class DnsStrategy(val value: Int) {
        VIRTUAL(0),
        OVER_TCP(1),
        DIRECT(2)
    }

    private enum class Verbosity(val value: Int) {
        OFF(0),
        ERROR(1),
        WARN(2),
        INFO(3),
        DEBUG(4),
        TRACE(5)
    }

    companion object {
        private val nativeLoadError: Throwable? by lazy {
            runCatching {
                ensureNativeLoaded()
            }.exceptionOrNull()
        }

        fun isNativeReady(): Boolean = nativeLoadError == null

        fun nativeLoadFailureMessage(): String? = nativeLoadError?.message

        private fun ensureNativeLoaded() {
            System.loadLibrary("tun2proxy")
            System.loadLibrary("novpn_tun2proxy_jni")
        }
    }
}
