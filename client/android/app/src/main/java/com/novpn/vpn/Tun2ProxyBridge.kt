package com.novpn.vpn

import android.content.Context
import android.os.ParcelFileDescriptor
import android.util.Log
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.CountDownLatch
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.FutureTask
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException

class Tun2ProxyBridge(context: Context) {
    init {
        ensureNativeLoaded()
    }

    private val logStore = RuntimeLogStore(context)

    fun start(tunnel: ParcelFileDescriptor, proxy: RuntimeLocalProxyConfig, mtu: Int) {
        synchronized(lifecycleLock) {
            stopLocked()

            val proxyUrl = proxy.socksUrl()
            val detachedFd = ParcelFileDescriptor.dup(tunnel.fileDescriptor).detachFd()
            val startedSignal = CountDownLatch(1)
            val sessionId = synchronized(sharedStateLock) {
                sharedSessionId += 1
                sharedActiveTunFd = detachedFd
                sharedSessionId
            }
            logStore.append(
                "tun2proxy",
                "Starting bridge session=$sessionId fd=$detachedFd proxy=${proxy.listenHost}:${proxy.socksPort} mtu=$mtu"
            )

            val futureTask = FutureTask<Int> {
                startedSignal.countDown()
                try {
                    nativeRunWithFd(
                        proxyUrl = proxyUrl,
                        tunFd = detachedFd,
                        mtu = mtu,
                        dnsStrategy = DnsStrategy.OVER_TCP.value,
                        verbosity = Verbosity.DEBUG.value
                    )
                } finally {
                    logStore.append("tun2proxy", "Bridge session=$sessionId finished")
                    synchronized(sharedStateLock) {
                        if (sharedActiveSession?.id == sessionId && sharedActiveTunFd == detachedFd) {
                            closeTunFdQuietly(detachedFd)
                            sharedActiveTunFd = INVALID_TUN_FD
                            sharedActiveSession = null
                        }
                    }
                }
            }

            synchronized(sharedStateLock) {
                sharedActiveSession = ActiveBridgeSession(
                    id = sessionId,
                    tunFd = detachedFd,
                    startedSignal = startedSignal,
                    future = futureTask
                )
            }
            sharedExecutor.execute(futureTask)
        }
    }

    fun confirmStarted(startupDelayMillis: Long = 500) {
        val currentSession = synchronized(lifecycleLock) {
            val session = synchronized(sharedStateLock) { sharedActiveSession }
                ?: throw IllegalStateException("tun2proxy did not start.")
            if (!session.startedSignal.await(START_WAIT_TIMEOUT_SECONDS, TimeUnit.SECONDS)) {
                session.future.cancel(true)
                throw IllegalStateException(
                    "tun2proxy startup is still waiting for the previous bridge session to exit."
                )
            }
            session
        }

        Thread.sleep(startupDelayMillis)
        if (!currentSession.future.isDone) {
            logStore.append("tun2proxy", "Bridge confirmed active after ${startupDelayMillis}ms startup delay")
            return
        }

        val result = try {
            currentSession.future.get()
        } catch (error: ExecutionException) {
            throw IllegalStateException(
                "tun2proxy failed before traffic forwarding became active.",
                error.cause ?: error
            )
        }

        throw IllegalStateException(
            "tun2proxy exited before traffic forwarding became active (code $result)."
        )
    }

    fun stop() {
        synchronized(lifecycleLock) {
            stopLocked()
        }
    }

    private fun stopLocked(): Boolean {
        val pendingSession: ActiveBridgeSession?
        val tunFdToClose: Int
        val sessionStarted: Boolean
        synchronized(sharedStateLock) {
            pendingSession = sharedActiveSession
            sharedActiveSession = null
            tunFdToClose = sharedActiveTunFd
            sharedActiveTunFd = INVALID_TUN_FD
            sessionStarted = pendingSession?.hasEnteredNativeRunLoop() == true
        }

        if (pendingSession != null && !sessionStarted) {
            pendingSession.future.cancel(true)
        }

        if (sessionStarted || tunFdToClose != INVALID_TUN_FD) {
            runCatching {
                val stopResult = nativeStop()
                logStore.append("tun2proxy", "Requested native bridge stop result=$stopResult")
            }.onFailure { error ->
                logStore.append(
                    "tun2proxy",
                    "Native bridge stop request failed: ${error.message ?: error.javaClass.simpleName}"
                )
            }
        }

        closeTunFdQuietly(tunFdToClose)

        if (pendingSession == null) {
            return true
        }

        return try {
            pendingSession.future.get(STOP_WAIT_TIMEOUT_SECONDS, TimeUnit.SECONDS)
            true
        } catch (_: TimeoutException) {
            Log.w(TAG, "tun2proxy did not stop within timeout after closing TUN fd")
            logStore.append("tun2proxy", "Bridge stop timed out after ${STOP_WAIT_TIMEOUT_SECONDS}s")
            false
        } catch (_: Exception) {
            // The bridge thread is already terminating; no extra action needed here.
            true
        }
    }

    fun waitForLocalProxy(proxy: RuntimeLocalProxyConfig, timeoutSeconds: Long = 10) {
        logStore.append(
            "tun2proxy",
            "Waiting for local proxy ${proxy.listenHost}:${proxy.socksPort} for up to ${timeoutSeconds}s"
        )
        val deadlineNanos = System.nanoTime() + TimeUnit.SECONDS.toNanos(timeoutSeconds)
        while (System.nanoTime() < deadlineNanos) {
            if (isLocalProxyReachable(proxy)) {
                logStore.append("tun2proxy", "Local proxy became reachable")
                return
            }
            Thread.sleep(100)
        }
        logStore.append("tun2proxy", "Local proxy did not become reachable in time")
        throw IllegalStateException("Local SOCKS bridge did not become ready in time.")
    }

    fun isLocalProxyReachable(proxy: RuntimeLocalProxyConfig, timeoutMillis: Int = 1200): Boolean {
        return runCatching {
            Socket().use { socket ->
                socket.connect(InetSocketAddress(proxy.listenHost, proxy.socksPort), timeoutMillis)
            }
            true
        }.getOrDefault(false)
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
        private const val TAG = "NoVPNTun2Proxy"
        private const val INVALID_TUN_FD = -1
        private const val STOP_WAIT_TIMEOUT_SECONDS = 4L
        private const val START_WAIT_TIMEOUT_SECONDS = 5L
        private val lifecycleLock = Any()
        private val sharedStateLock = Any()
        private val sharedExecutor = Executors.newSingleThreadExecutor { runnable ->
            Thread(runnable, "novpn-tun2proxy").apply { isDaemon = true }
        }
        private var sharedActiveSession: ActiveBridgeSession? = null
        private var sharedActiveTunFd: Int = INVALID_TUN_FD
        private var sharedSessionId: Long = 0L
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

        private fun closeTunFdQuietly(fd: Int) {
            if (fd == INVALID_TUN_FD) {
                return
            }
            runCatching { ParcelFileDescriptor.adoptFd(fd).close() }
        }
    }

    private data class ActiveBridgeSession(
        val id: Long,
        val tunFd: Int,
        val startedSignal: CountDownLatch,
        val future: Future<Int>
    ) {
        fun hasEnteredNativeRunLoop(): Boolean = startedSignal.await(0, TimeUnit.SECONDS)
    }
}
