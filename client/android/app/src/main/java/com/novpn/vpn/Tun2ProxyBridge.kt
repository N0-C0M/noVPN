package com.novpn.vpn

import android.os.ParcelFileDescriptor
import java.net.Socket
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.TimeUnit

class Tun2ProxyBridge {
    private val executor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "novpn-tun2proxy").apply { isDaemon = true }
    }
    private var task: Future<*>? = null

    fun start(tunnel: ParcelFileDescriptor, proxy: RuntimeLocalProxyConfig, mtu: Int) {
        stop()
        waitForLocalProxy(proxy)

        val proxyUrl = proxy.socksUrl()
        val detachedFd = ParcelFileDescriptor.dup(tunnel.fileDescriptor).detachFd()
        task = executor.submit {
            nativeRunWithFd(
                proxyUrl = proxyUrl,
                tunFd = detachedFd,
                mtu = mtu,
                dnsStrategy = DnsStrategy.VIRTUAL.value,
                verbosity = Verbosity.INFO.value
            )
        }
    }

    fun stop() {
        nativeStop()
        task?.cancel(true)
        task = null
    }

    private fun waitForLocalProxy(proxy: RuntimeLocalProxyConfig) {
        val deadlineNanos = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
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
        init {
            System.loadLibrary("tun2proxy")
            System.loadLibrary("novpn_tun2proxy_jni")
        }
    }
}
