#include <windows.h>
#include <fwpmu.h>
#include <fwptypes.h>

#include <stdexcept>
#include <string>
#include <vector>

#pragma comment(lib, "Fwpuclnt.lib")

namespace novpn::wfp {

namespace {

constexpr GUID kProviderKey = {
    0x7bb03bc1, 0x4cd9, 0x437c, {0x89, 0x66, 0xb7, 0x75, 0x3d, 0x14, 0x68, 0xc1}
};

constexpr GUID kSubLayerKey = {
    0x1d946e4f, 0xe70e, 0x4f9d, {0xae, 0x33, 0x66, 0x79, 0xa4, 0xb3, 0x7f, 0x01}
};

std::runtime_error Win32Error(const char* message, DWORD code) {
    return std::runtime_error(std::string(message) + " failed with code " + std::to_string(code));
}

class EngineHandle {
  public:
    EngineHandle() = default;
    ~EngineHandle() {
        if (handle_ != nullptr) {
            FwpmEngineClose0(handle_);
        }
    }

    EngineHandle(const EngineHandle&) = delete;
    EngineHandle& operator=(const EngineHandle&) = delete;

    void Open() {
        DWORD result = FwpmEngineOpen0(nullptr, RPC_C_AUTHN_WINNT, nullptr, nullptr, &handle_);
        if (result != ERROR_SUCCESS) {
            throw Win32Error("FwpmEngineOpen0", result);
        }
    }

    HANDLE Get() const {
        return handle_;
    }

  private:
    HANDLE handle_ = nullptr;
};

void EnsureProvider(HANDLE engine) {
    FWPM_PROVIDER0 provider = {};
    provider.providerKey = kProviderKey;
    provider.displayData.name = const_cast<wchar_t*>(L"NoVPN Desktop WFP Provider");
    provider.displayData.description = const_cast<wchar_t*>(L"Provider for NoVPN desktop split-tunnel filters.");

    const DWORD result = FwpmProviderAdd0(engine, &provider, nullptr);
    if (result != ERROR_SUCCESS && result != FWP_E_ALREADY_EXISTS) {
        throw Win32Error("FwpmProviderAdd0", result);
    }
}

void EnsureSubLayer(HANDLE engine) {
    FWPM_SUBLAYER0 subLayer = {};
    subLayer.subLayerKey = kSubLayerKey;
    subLayer.providerKey = const_cast<GUID*>(&kProviderKey);
    subLayer.displayData.name = const_cast<wchar_t*>(L"NoVPN Desktop Split Tunnel");
    subLayer.displayData.description = const_cast<wchar_t*>(L"App-scoped split tunnel filters.");
    subLayer.weight = 0x7FFF;

    const DWORD result = FwpmSubLayerAdd0(engine, &subLayer, nullptr);
    if (result != ERROR_SUCCESS && result != FWP_E_ALREADY_EXISTS) {
        throw Win32Error("FwpmSubLayerAdd0", result);
    }
}

void AddAppFilter(HANDLE engine, const wchar_t* exePath, const GUID& layerKey) {
    FWP_BYTE_BLOB* appId = nullptr;
    DWORD result = FwpmGetAppIdFromFileName0(exePath, &appId);
    if (result != ERROR_SUCCESS) {
        throw Win32Error("FwpmGetAppIdFromFileName0", result);
    }

    FWPM_FILTER_CONDITION0 condition = {};
    condition.fieldKey = FWPM_CONDITION_ALE_APP_ID;
    condition.matchType = FWP_MATCH_EQUAL;
    condition.conditionValue.type = FWP_BYTE_BLOB_TYPE;
    condition.conditionValue.byteBlob = appId;

    FWPM_FILTER0 filter = {};
    filter.providerKey = const_cast<GUID*>(&kProviderKey);
    filter.subLayerKey = kSubLayerKey;
    filter.layerKey = layerKey;
    filter.displayData.name = const_cast<wchar_t*>(L"NoVPN App Bypass");
    filter.action.type = FWP_ACTION_PERMIT;
    filter.weight.type = FWP_EMPTY;
    filter.numFilterConditions = 1;
    filter.filterCondition = &condition;

    result = FwpmFilterAdd0(engine, &filter, nullptr, nullptr);
    FwpmFreeMemory0(reinterpret_cast<void**>(&appId));
    if (result != ERROR_SUCCESS) {
        throw Win32Error("FwpmFilterAdd0", result);
    }
}

void RemoveProviderObjects(HANDLE engine) {
    FWPM_FILTER_ENUM_TEMPLATE0 filterTemplate = {};
    filterTemplate.providerKey = const_cast<GUID*>(&kProviderKey);

    HANDLE enumHandle = nullptr;
    DWORD result = FwpmFilterCreateEnumHandle0(engine, &filterTemplate, &enumHandle);
    if (result != ERROR_SUCCESS) {
        throw Win32Error("FwpmFilterCreateEnumHandle0", result);
    }

    FWPM_FILTER0** filters = nullptr;
    UINT32 entryCount = 0;
    result = FwpmFilterEnum0(engine, enumHandle, 1024, &filters, &entryCount);
    if (result == ERROR_SUCCESS) {
        for (UINT32 index = 0; index < entryCount; ++index) {
            FwpmFilterDeleteById0(engine, filters[index]->filterId);
        }
    } else if (result != FWP_E_FILTER_NOT_FOUND) {
        FwpmFilterDestroyEnumHandle0(engine, enumHandle);
        throw Win32Error("FwpmFilterEnum0", result);
    }

    if (filters != nullptr) {
        FwpmFreeMemory0(reinterpret_cast<void**>(&filters));
    }
    FwpmFilterDestroyEnumHandle0(engine, enumHandle);

    result = FwpmSubLayerDeleteByKey0(engine, &kSubLayerKey);
    if (result != ERROR_SUCCESS && result != FWP_E_SUBLAYER_NOT_FOUND) {
        throw Win32Error("FwpmSubLayerDeleteByKey0", result);
    }

    result = FwpmProviderDeleteByKey0(engine, &kProviderKey);
    if (result != ERROR_SUCCESS && result != FWP_E_PROVIDER_NOT_FOUND) {
        throw Win32Error("FwpmProviderDeleteByKey0", result);
    }
}

void Usage() {
    OutputDebugStringA("Usage: novpn_wfp_helper.exe install <exe> [<exe>...] | clear\n");
}

}  // namespace

void InstallDirectBypassFilters(const std::vector<std::wstring>& exePaths) {
    EngineHandle engine;
    engine.Open();

    DWORD result = FwpmTransactionBegin0(engine.Get(), 0);
    if (result != ERROR_SUCCESS) {
        throw Win32Error("FwpmTransactionBegin0", result);
    }

    try {
        EnsureProvider(engine.Get());
        EnsureSubLayer(engine.Get());

        for (const auto& exePath : exePaths) {
            AddAppFilter(engine.Get(), exePath.c_str(), FWPM_LAYER_ALE_AUTH_CONNECT_V4);
            AddAppFilter(engine.Get(), exePath.c_str(), FWPM_LAYER_ALE_AUTH_CONNECT_V6);
        }

        result = FwpmTransactionCommit0(engine.Get());
        if (result != ERROR_SUCCESS) {
            throw Win32Error("FwpmTransactionCommit0", result);
        }
    } catch (...) {
        FwpmTransactionAbort0(engine.Get());
        throw;
    }
}

void ClearFilters() {
    EngineHandle engine;
    engine.Open();

    DWORD result = FwpmTransactionBegin0(engine.Get(), 0);
    if (result != ERROR_SUCCESS) {
        throw Win32Error("FwpmTransactionBegin0", result);
    }

    try {
        RemoveProviderObjects(engine.Get());
        result = FwpmTransactionCommit0(engine.Get());
        if (result != ERROR_SUCCESS) {
            throw Win32Error("FwpmTransactionCommit0", result);
        }
    } catch (...) {
        FwpmTransactionAbort0(engine.Get());
        throw;
    }
}

}  // namespace novpn::wfp

int wmain(int argc, wchar_t** argv) {
    try {
        if (argc < 2) {
            novpn::wfp::Usage();
            return 2;
        }

        const std::wstring command = argv[1];
        if (command == L"clear") {
            novpn::wfp::ClearFilters();
            return 0;
        }

        if (command == L"install") {
            if (argc < 3) {
                novpn::wfp::Usage();
                return 2;
            }
            std::vector<std::wstring> exePaths;
            exePaths.reserve(static_cast<size_t>(argc - 2));
            for (int index = 2; index < argc; ++index) {
                exePaths.emplace_back(argv[index]);
            }
            novpn::wfp::InstallDirectBypassFilters(exePaths);
            return 0;
        }

        novpn::wfp::Usage();
        return 2;
    } catch (const std::exception& ex) {
        OutputDebugStringA(ex.what());
        return 1;
    }
}
