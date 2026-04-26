package com.novpn.vpn

import android.content.Context
import android.util.Log
import android.os.ParcelFileDescriptor
import java.util.concurrent.ExecutionException
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import java.util.concurrent.CancellationException

class Tun2ProxyBridge(context: Context) {
    init {
        ensureNativeLoaded()
    }

    private val logStore = RuntimeLogStore(context)
    private val stateLock = Any()
    private val executor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "novpn-tun2proxy").apply { isDaemon = true }
    }
    private var task: Future<Int>? = null
    private var activeTunFd: Int = INVALID_TUN_FD
    private var activeSessionId: Long = 0L

    fun start(tunnel: ParcelFileDescriptor, proxy: RuntimeLocalProxyConfig, mtu: Int) {
        stop(requireCompletion = true)

        val proxyUrl = proxy.socksUrl()
        val detachedFd = ParcelFileDescriptor.dup(tunnel.fileDescriptor).detachFd()
        val sessionId = synchronized(stateLock) {
            activeSessionId += 1
            activeTunFd = detachedFd
            activeSessionId
        }
        logStore.append(
            "tun2proxy",
            "Starting bridge session=$sessionId fd=$detachedFd proxy=${proxy.listenHost}:${proxy.socksPort} mtu=$mtu"
        )
        val newTask = executor.submit<Int> {
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
                synchronized(stateLock) {
                    if (activeSessionId == sessionId && activeTunFd == detachedFd) {
                        closeTunFdQuietly(detachedFd)
                        activeTunFd = INVALID_TUN_FD
                    }
                }
            }
        }
        synchronized(stateLock) {
            task = newTask
        }
    }

    fun confirmStarted(startupDelayMillis: Long = 500) {
        val currentTask = synchronized(stateLock) { task }
            ?: throw IllegalStateException("tun2proxy did not start.")

        Thread.sleep(startupDelayMillis)
        if (!currentTask.isDone) {
            logStore.append("tun2proxy", "Bridge confirmed active after ${startupDelayMillis}ms startup delay")
            return
        }

        val result = try {
            currentTask.get()
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
        stop(requireCompletion = false)
    }

    private fun stop(requireCompletion: Boolean) {
        val pendingTask: Future<*>?
        val tunFdToClose: Int
        val shouldSignalNativeStop: Boolean
        synchronized(stateLock) {
            pendingTask = task
            tunFdToClose = activeTunFd
            shouldSignalNativeStop = pendingTask != null && !pendingTask.isDone
            activeTunFd = INVALID_TUN_FD
            activeSessionId += 1
        }

        if (pendingTask == null && tunFdToClose == INVALID_TUN_FD) {
            return
        }

        if (shouldSignalNativeStop) {
            requestNativeStop("initial")
        }
        closeTunFdQuietly(tunFdToClose)

        if (pendingTask == null) {
            return
        }

        if (waitForTaskStop(pendingTask, STOP_WAIT_TIMEOUT_SECONDS)) {
            clearTaskReference(pendingTask)
            logStore.append("tun2proxy", "Bridge stop completed within timeout")
            return
        }

        Log.w(TAG, "tun2proxy did not stop within timeout after initial stop attempt")
        logStore.append(
            "tun2proxy",
            "Bridge stop timed out after ${STOP_WAIT_TIMEOUT_SECONDS}s; sending second stop signal"
        )

        if (shouldSignalNativeStop) {
            requestNativeStop("retry")
        }
        if (waitForTaskStop(pendingTask, STOP_FORCE_WAIT_TIMEOUT_SECONDS)) {
            clearTaskReference(pendingTask)
            logStore.append("tun2proxy", "Bridge stop completed after second stop signal")
            return
        }

        if (requireCompletion) {
            throw IllegalStateException(
                "Previous tun2proxy instance did not stop within " +
                    "${STOP_WAIT_TIMEOUT_SECONDS + STOP_FORCE_WAIT_TIMEOUT_SECONDS}s."
            )
        }

        Log.w(TAG, "tun2proxy is still running after repeated stop attempts")
        logStore.append(
            "tun2proxy",
            "Bridge is still running after repeated stop attempts " +
                "(${STOP_WAIT_TIMEOUT_SECONDS + STOP_FORCE_WAIT_TIMEOUT_SECONDS}s total)"
        )
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

    private fun requestNativeStop(stage: String) {
        val stopResult = runCatching { nativeStop() }
        stopResult.onFailure { error ->
            Log.w(TAG, "nativeStop failed on $stage stage", error)
            logStore.append(
                "tun2proxy",
                "nativeStop failed on $stage stage: ${error.message ?: error.javaClass.simpleName}"
            )
        }
    }

    private fun waitForTaskStop(pendingTask: Future<*>, timeoutSeconds: Long): Boolean {
        return try {
            pendingTask.get(timeoutSeconds, TimeUnit.SECONDS)
            true
        } catch (_: TimeoutException) {
            false
        } catch (_: ExecutionException) {
            true
        } catch (_: CancellationException) {
            true
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
            false
        }
    }

    private fun clearTaskReference(completedTask: Future<*>) {
        synchronized(stateLock) {
            if (task === completedTask) {
                task = null
            }
        }
    }

    private fun closeTunFdQuietly(fd: Int) {
        if (fd == INVALID_TUN_FD) {
            return
        }
        runCatching { ParcelFileDescriptor.adoptFd(fd).close() }
    }

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
        private const val STOP_WAIT_TIMEOUT_SECONDS = 5L
        private const val STOP_FORCE_WAIT_TIMEOUT_SECONDS = 3L
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
