#include <android/log.h>
#include <jni.h>
#include <mutex>
#include <string>

#include "tun2proxy.h"

namespace {
constexpr const char *kTag = "NoVPNTun2Proxy";

std::mutex gJvmMutex;
JavaVM *gJvm = nullptr;

void log_callback(Tun2proxyVerbosity verbosity, const char *message, void *) {
    int priority = ANDROID_LOG_INFO;
    switch (verbosity) {
        case Tun2proxyVerbosity_Error:
            priority = ANDROID_LOG_ERROR;
            break;
        case Tun2proxyVerbosity_Warn:
            priority = ANDROID_LOG_WARN;
            break;
        case Tun2proxyVerbosity_Info:
            priority = ANDROID_LOG_INFO;
            break;
        case Tun2proxyVerbosity_Debug:
        case Tun2proxyVerbosity_Trace:
            priority = ANDROID_LOG_DEBUG;
            break;
        case Tun2proxyVerbosity_Off:
        default:
            priority = ANDROID_LOG_VERBOSE;
            break;
    }

    __android_log_print(priority, kTag, "%s", message == nullptr ? "" : message);
}
}  // namespace

extern "C" JNIEXPORT jint JNICALL JNI_OnLoad(JavaVM *vm, void *) {
    std::lock_guard<std::mutex> lock(gJvmMutex);
    gJvm = vm;
    tun2proxy_set_log_callback(log_callback, nullptr);
    return JNI_VERSION_1_6;
}

extern "C" JNIEXPORT jint JNICALL
Java_com_novpn_vpn_Tun2ProxyBridge_nativeRunWithFd(JNIEnv *env,
                                                   jobject /* this */,
                                                   jstring proxyUrl,
                                                   jint tunFd,
                                                   jint mtu,
                                                   jint dnsStrategy,
                                                   jint verbosity) {
    const char *proxy_url = env->GetStringUTFChars(proxyUrl, nullptr);
    const int result = tun2proxy_with_fd_run(
        proxy_url,
        tunFd,
        true,
        false,
        static_cast<unsigned short>(mtu),
        static_cast<Tun2proxyDns>(dnsStrategy),
        static_cast<Tun2proxyVerbosity>(verbosity)
    );
    env->ReleaseStringUTFChars(proxyUrl, proxy_url);
    return result;
}

extern "C" JNIEXPORT jint JNICALL
Java_com_novpn_vpn_Tun2ProxyBridge_nativeStop(JNIEnv * /* env */, jobject /* this */) {
    return tun2proxy_stop();
}
