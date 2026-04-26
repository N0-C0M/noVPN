#include <android/log.h>
#include <dlfcn.h>
#include <jni.h>
#include <mutex>
#include <string>

#include "tun2proxy.h"

namespace {
constexpr const char *kTag = "NoVPNTun2Proxy";

std::mutex gJvmMutex;
std::mutex gLoadMutex;
JavaVM *gJvm = nullptr;
void *gTun2ProxyHandle = nullptr;
std::string gLoadError;
std::string gLoadedLibraryPath;

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

bool load_tun2proxy_from(const char *library_path) {
    if (library_path == nullptr || library_path[0] == '\0') {
        gLoadError = "tun2proxy library path is empty";
        __android_log_print(ANDROID_LOG_ERROR, kTag, "%s", gLoadError.c_str());
        return false;
    }

    if (gTun2ProxyHandle != nullptr &&
        gLoadedLibraryPath == library_path &&
        gSetLogCallback != nullptr &&
        gRunWithFd != nullptr &&
        gStop != nullptr) {
        return true;
    }

    dlerror();
    void *new_handle = dlopen(library_path, RTLD_NOW | RTLD_LOCAL);
    if (new_handle == nullptr) {
        const char *error = dlerror();
        gLoadError = error == nullptr ? "dlopen(tun2proxy session library) failed" : error;
        __android_log_print(ANDROID_LOG_ERROR, kTag, "%s", gLoadError.c_str());
        return false;
    }

    auto *new_set_log_callback =
        reinterpret_cast<SetLogCallbackFn>(dlsym(new_handle, "tun2proxy_set_log_callback"));
    auto *new_run_with_fd =
        reinterpret_cast<RunWithFdFn>(dlsym(new_handle, "tun2proxy_with_fd_run"));
    auto *new_stop = reinterpret_cast<StopFn>(dlsym(new_handle, "tun2proxy_stop"));

    if (new_set_log_callback == nullptr || new_run_with_fd == nullptr || new_stop == nullptr) {
        const char *error = dlerror();
        gLoadError = error == nullptr ? "dlsym(tun2proxy symbols) failed" : error;
        __android_log_print(ANDROID_LOG_ERROR, kTag, "%s", gLoadError.c_str());
        dlclose(new_handle);
        return false;
    }

    gTun2ProxyHandle = new_handle;
    gSetLogCallback = new_set_log_callback;
    gRunWithFd = new_run_with_fd;
    gStop = new_stop;
    gLoadedLibraryPath = library_path;
    gSetLogCallback(log_callback, nullptr);
    gLoadError.clear();
    return true;
}
}  // namespace

extern "C" JNIEXPORT jint JNICALL JNI_OnLoad(JavaVM *vm, void *) {
    std::lock_guard<std::mutex> lock(gJvmMutex);
    gJvm = vm;
    return JNI_VERSION_1_6;
}

extern "C" JNIEXPORT jint JNICALL
Java_com_novpn_vpn_Tun2ProxyBridge_nativeRunWithFd(JNIEnv *env,
                                                   jobject /* this */,
                                                   jstring libraryPath,
                                                   jstring proxyUrl,
                                                   jint tunFd,
                                                   jint mtu,
                                                   jint dnsStrategy,
                                                   jint verbosity) {
    const char *library_path = env->GetStringUTFChars(libraryPath, nullptr);
    RunWithFdFn run_with_fd = nullptr;
    {
        std::lock_guard<std::mutex> lock(gLoadMutex);
        if (load_tun2proxy_from(library_path)) {
            run_with_fd = gRunWithFd;
        }
    }
    if (run_with_fd == nullptr) {
        env->ReleaseStringUTFChars(libraryPath, library_path);
        return -1;
    }

    const char *proxy_url = env->GetStringUTFChars(proxyUrl, nullptr);
    const int result = run_with_fd(
        proxy_url,
        tunFd,
        false,
        false,
        static_cast<unsigned short>(mtu),
        static_cast<Tun2proxyDns>(dnsStrategy),
        static_cast<Tun2proxyVerbosity>(verbosity)
    );
    env->ReleaseStringUTFChars(proxyUrl, proxy_url);
    env->ReleaseStringUTFChars(libraryPath, library_path);
    return result;
}

extern "C" JNIEXPORT jint JNICALL
Java_com_novpn_vpn_Tun2ProxyBridge_nativeStop(JNIEnv * /* env */, jobject /* this */) {
    std::lock_guard<std::mutex> lock(gLoadMutex);
    if (gStop == nullptr) {
        return -1;
    }
    return gStop();
}
