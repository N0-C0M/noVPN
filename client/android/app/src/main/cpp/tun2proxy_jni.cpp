#include <android/log.h>
#include <dlfcn.h>
#include <jni.h>
#include <mutex>
#include <string>

#include "tun2proxy.h"

namespace {
constexpr const char *kTag = "NoVPNTun2Proxy";

std::mutex gJvmMutex;
std::mutex gRunStateMutex;
JavaVM *gJvm = nullptr;
void *gTun2ProxyHandle = nullptr;
std::string gLoadError;
bool gRunActive = false;

using SetLogCallbackFn = void (*)(void (*)(enum Tun2proxyVerbosity, const char *, void *), void *);
using RunWithFdFn = int (*)(const char *,
                            int,
                            bool,
                            bool,
                            unsigned short,
                            enum Tun2proxyDns,
                            enum Tun2proxyVerbosity);
using StopFn = int (*)();

SetLogCallbackFn gSetLogCallback = nullptr;
RunWithFdFn gRunWithFd = nullptr;
StopFn gStop = nullptr;

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

bool ensure_tun2proxy_loaded() {
    if (gTun2ProxyHandle != nullptr && gSetLogCallback != nullptr && gRunWithFd != nullptr && gStop != nullptr) {
        return true;
    }

    dlerror();
    gTun2ProxyHandle = dlopen("libtun2proxy.so", RTLD_NOW | RTLD_GLOBAL);
    if (gTun2ProxyHandle == nullptr) {
        const char *error = dlerror();
        gLoadError = error == nullptr ? "dlopen(libtun2proxy.so) failed" : error;
        __android_log_print(ANDROID_LOG_ERROR, kTag, "%s", gLoadError.c_str());
        return false;
    }

    gSetLogCallback = reinterpret_cast<SetLogCallbackFn>(dlsym(gTun2ProxyHandle, "tun2proxy_set_log_callback"));
    gRunWithFd = reinterpret_cast<RunWithFdFn>(dlsym(gTun2ProxyHandle, "tun2proxy_with_fd_run"));
    gStop = reinterpret_cast<StopFn>(dlsym(gTun2ProxyHandle, "tun2proxy_stop"));

    if (gSetLogCallback == nullptr || gRunWithFd == nullptr || gStop == nullptr) {
        const char *error = dlerror();
        gLoadError = error == nullptr ? "dlsym(tun2proxy symbols) failed" : error;
        __android_log_print(ANDROID_LOG_ERROR, kTag, "%s", gLoadError.c_str());
        return false;
    }

    gLoadError.clear();
    return true;
}
}  // namespace

extern "C" JNIEXPORT jint JNICALL JNI_OnLoad(JavaVM *vm, void *) {
    std::lock_guard<std::mutex> lock(gJvmMutex);
    gJvm = vm;
    if (ensure_tun2proxy_loaded()) {
        gSetLogCallback(log_callback, nullptr);
    }
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
    if (!ensure_tun2proxy_loaded()) {
        return -1;
    }

    const char *proxy_url = env->GetStringUTFChars(proxyUrl, nullptr);
    {
        std::lock_guard<std::mutex> lock(gRunStateMutex);
        gRunActive = true;
    }
    const int result = gRunWithFd(
        proxy_url,
        tunFd,
        true,
        false,
        static_cast<unsigned short>(mtu),
        static_cast<Tun2proxyDns>(dnsStrategy),
        static_cast<Tun2proxyVerbosity>(verbosity)
    );
    {
        std::lock_guard<std::mutex> lock(gRunStateMutex);
        gRunActive = false;
    }
    env->ReleaseStringUTFChars(proxyUrl, proxy_url);
    return result;
}

extern "C" JNIEXPORT jint JNICALL
Java_com_novpn_vpn_Tun2ProxyBridge_nativeStop(JNIEnv * /* env */, jobject /* this */) {
    if (!ensure_tun2proxy_loaded()) {
        return -1;
    }

    {
        std::lock_guard<std::mutex> lock(gRunStateMutex);
        if (!gRunActive) {
            return 0;
        }
    }

    return gStop();
}
