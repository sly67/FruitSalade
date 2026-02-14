/*
 * cfapi_shim.cpp - Windows Cloud Files API wrapper implementation
 *
 * This file implements the C-linkage functions declared in cfapi_shim.h.
 * It wraps the Windows CfAPI (Cloud Files API) to provide:
 *   - Sync root registration and connection
 *   - Placeholder creation and updates
 *   - Hydration callback dispatch to Go via goHydrationCallback
 *   - Data transfer for hydration requests
 *
 * Build requirements:
 *   - Windows 10 1809+ SDK
 *   - Link: cldapi.lib ole32.lib
 *   - Compile as C++17
 *
 * This file is committed as source only and built on Windows.
 */

#ifdef _WIN32

#include <windows.h>
#include <cfapi.h>
#include <objbase.h>
#include <pathcch.h>
#include <shlobj.h>
#include <string>
#include <vector>
#include <cstring>

#include "cfapi_shim.h"

#pragma comment(lib, "cldapi.lib")
#pragma comment(lib, "ole32.lib")

/* ---------- Helpers ---------- */

// Convert UTF-8 to wide string.
static std::wstring Utf8ToWide(const char *utf8) {
    if (!utf8 || !*utf8) return L"";
    int len = MultiByteToWideChar(CP_UTF8, 0, utf8, -1, nullptr, 0);
    if (len <= 0) return L"";
    std::wstring result(len - 1, L'\0');
    MultiByteToWideChar(CP_UTF8, 0, utf8, -1, &result[0], len);
    return result;
}

// Convert Unix timestamp to FILETIME.
static FILETIME UnixToFileTime(long long unixTime) {
    // Windows FILETIME epoch: Jan 1, 1601.  Unix epoch: Jan 1, 1970.
    // Difference: 11644473600 seconds.  FILETIME is in 100-ns intervals.
    ULARGE_INTEGER uli;
    uli.QuadPart = (static_cast<unsigned long long>(unixTime) + 11644473600ULL) * 10000000ULL;
    FILETIME ft;
    ft.dwLowDateTime = uli.LowPart;
    ft.dwHighDateTime = uli.HighPart;
    return ft;
}

/* ---------- Hydration Callback ---------- */

// This callback is invoked by CfAPI when Explorer or an application reads a
// placeholder file.  We dispatch to Go via goHydrationCallback.
static void CALLBACK FetchDataCallback(
    _In_ CONST CF_CALLBACK_INFO *callbackInfo,
    _In_ CONST CF_CALLBACK_PARAMETERS *callbackParameters)
{
    // Extract file identity (our file ID stored as a UTF-8 string blob).
    const char *fileIdentity = static_cast<const char *>(callbackInfo->FileIdentity);
    int fileIdentityLen = static_cast<int>(callbackInfo->FileIdentityLength);

    long long offset = callbackParameters->FetchData.RequiredFileOffset.QuadPart;
    long long length = callbackParameters->FetchData.RequiredLength.QuadPart;

    CF_TRANSFER_KEY transferKey = callbackInfo->TransferKey;

    // Dispatch to Go.
    goHydrationCallback(fileIdentity, fileIdentityLen, offset, length, transferKey);
}

// Callback for cancel fetch (no-op; Go side handles timeouts).
static void CALLBACK CancelFetchDataCallback(
    _In_ CONST CF_CALLBACK_INFO *callbackInfo,
    _In_ CONST CF_CALLBACK_PARAMETERS *callbackParameters)
{
    // Intentionally empty.  The Go side will time out or cancel the context.
}

// Callback table registered with CfConnectSyncRoot.
static CF_CALLBACK_REGISTRATION s_callbackTable[] = {
    { CF_CALLBACK_TYPE_FETCH_DATA,         FetchDataCallback },
    { CF_CALLBACK_TYPE_CANCEL_FETCH_DATA,  CancelFetchDataCallback },
    CF_CALLBACK_REGISTRATION_END
};

/* ---------- Public API ---------- */

extern "C" {

long cfapi_init(void) {
    HRESULT hr = CoInitializeEx(nullptr, COINIT_MULTITHREADED);
    // S_OK or S_FALSE (already initialized) are both acceptable.
    if (FAILED(hr) && hr != RPC_E_CHANGED_MODE) {
        return static_cast<long>(hr);
    }
    return 0;
}

long cfapi_register_sync_root(const char *sync_root_path,
                               const char *display_name,
                               const char *version)
{
    std::wstring wRoot = Utf8ToWide(sync_root_path);
    std::wstring wName = Utf8ToWide(display_name);
    std::wstring wVer  = Utf8ToWide(version);

    CF_SYNC_REGISTRATION reg = {};
    reg.StructSize = sizeof(reg);
    reg.ProviderName = wName.c_str();
    reg.ProviderVersion = wVer.c_str();
    // Use a fixed GUID for FruitSalade.
    // {A1B2C3D4-E5F6-7890-ABCD-EF1234567890}
    static const CLSID s_providerId =
        { 0xa1b2c3d4, 0xe5f6, 0x7890, { 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90 } };
    reg.ProviderId = s_providerId;

    CF_SYNC_POLICIES policies = {};
    policies.StructSize = sizeof(policies);
    policies.Hydration.Primary = CF_HYDRATION_POLICY_FULL;
    policies.Population.Primary = CF_POPULATION_POLICY_FULL;
    policies.InSync = CF_INSYNC_POLICY_TRACK_ALL;
    policies.HardLink = CF_HARDLINK_POLICY_NONE;

    HRESULT hr = CfRegisterSyncRoot(wRoot.c_str(), &reg, &policies,
                                     CF_REGISTER_FLAG_UPDATE);
    return static_cast<long>(hr);
}

long cfapi_connect_sync_root(const char *sync_root_path,
                              CF_CONNECTION_KEY *out_key)
{
    std::wstring wRoot = Utf8ToWide(sync_root_path);

    HRESULT hr = CfConnectSyncRoot(
        wRoot.c_str(),
        s_callbackTable,
        nullptr,    // callbackContext (we use the global backend pointer in Go)
        CF_CONNECT_FLAG_REQUIRE_PROCESS_INFO |
            CF_CONNECT_FLAG_REQUIRE_FULL_FILE_PATH,
        out_key);

    return static_cast<long>(hr);
}

void cfapi_disconnect_sync_root(CF_CONNECTION_KEY key) {
    CfDisconnectSyncRoot(key);
}

long cfapi_unregister_sync_root(const char *sync_root_path) {
    std::wstring wRoot = Utf8ToWide(sync_root_path);
    HRESULT hr = CfUnregisterSyncRoot(wRoot.c_str());
    return static_cast<long>(hr);
}

long cfapi_create_placeholder(const char *parent_path,
                               const char *name,
                               const char *file_identity,
                               long long file_size,
                               long long mtime_unix,
                               int is_directory)
{
    std::wstring wParent = Utf8ToWide(parent_path);
    std::wstring wName   = Utf8ToWide(name);

    FILETIME ftMtime = UnixToFileTime(mtime_unix);

    CF_PLACEHOLDER_CREATE_INFO phInfo = {};
    phInfo.FileIdentity = file_identity;
    phInfo.FileIdentityLength = static_cast<DWORD>(strlen(file_identity));
    phInfo.RelativeFileName = wName.c_str();
    phInfo.FsMetadata.FileSize.QuadPart = file_size;
    phInfo.FsMetadata.BasicInfo.CreationTime.QuadPart =
        *reinterpret_cast<LONGLONG *>(&ftMtime);
    phInfo.FsMetadata.BasicInfo.LastWriteTime.QuadPart =
        *reinterpret_cast<LONGLONG *>(&ftMtime);
    phInfo.FsMetadata.BasicInfo.ChangeTime.QuadPart =
        *reinterpret_cast<LONGLONG *>(&ftMtime);
    phInfo.FsMetadata.BasicInfo.LastAccessTime.QuadPart =
        *reinterpret_cast<LONGLONG *>(&ftMtime);

    if (is_directory) {
        phInfo.FsMetadata.BasicInfo.FileAttributes = FILE_ATTRIBUTE_DIRECTORY;
        phInfo.Flags = CF_PLACEHOLDER_CREATE_FLAG_MARK_IN_SYNC |
                       CF_PLACEHOLDER_CREATE_FLAG_DISABLE_ON_DEMAND_POPULATION;
    } else {
        phInfo.FsMetadata.BasicInfo.FileAttributes = FILE_ATTRIBUTE_NORMAL;
        phInfo.Flags = CF_PLACEHOLDER_CREATE_FLAG_MARK_IN_SYNC;
    }

    phInfo.Result = S_OK;

    HRESULT hr = CfCreatePlaceholders(wParent.c_str(), &phInfo, 1,
                                       CF_CREATE_FLAG_NONE, nullptr);
    if (SUCCEEDED(hr) && FAILED(phInfo.Result)) {
        hr = phInfo.Result;
    }
    return static_cast<long>(hr);
}

long cfapi_update_placeholder(const char *file_path,
                               const char *file_identity,
                               long long file_size,
                               long long mtime_unix)
{
    std::wstring wPath = Utf8ToWide(file_path);
    FILETIME ftMtime = UnixToFileTime(mtime_unix);

    HANDLE hFile = CreateFileW(wPath.c_str(),
        WRITE_DAC, FILE_SHARE_READ, nullptr,
        OPEN_EXISTING, FILE_FLAG_BACKUP_SEMANTICS, nullptr);

    if (hFile == INVALID_HANDLE_VALUE) {
        return static_cast<long>(HRESULT_FROM_WIN32(GetLastError()));
    }

    CF_FS_METADATA fsMetadata = {};
    fsMetadata.FileSize.QuadPart = file_size;
    fsMetadata.BasicInfo.LastWriteTime.QuadPart =
        *reinterpret_cast<LONGLONG *>(&ftMtime);
    fsMetadata.BasicInfo.ChangeTime.QuadPart =
        *reinterpret_cast<LONGLONG *>(&ftMtime);

    HRESULT hr = CfUpdatePlaceholder(
        hFile,
        &fsMetadata,
        file_identity,
        static_cast<DWORD>(strlen(file_identity)),
        nullptr, 0,
        CF_UPDATE_FLAG_MARK_IN_SYNC,
        nullptr, nullptr);

    CloseHandle(hFile);
    return static_cast<long>(hr);
}

long cfapi_dehydrate_placeholder(const char *file_path) {
    std::wstring wPath = Utf8ToWide(file_path);

    HANDLE hFile = CreateFileW(wPath.c_str(),
        WRITE_DAC, FILE_SHARE_READ, nullptr,
        OPEN_EXISTING, FILE_FLAG_BACKUP_SEMANTICS, nullptr);

    if (hFile == INVALID_HANDLE_VALUE) {
        return static_cast<long>(HRESULT_FROM_WIN32(GetLastError()));
    }

    HRESULT hr = CfDehydratePlaceholder(hFile, 0, -1,
                                         CF_DEHYDRATE_FLAG_NONE, nullptr);
    CloseHandle(hFile);
    return static_cast<long>(hr);
}

long cfapi_transfer_data(CF_CONNECTION_KEY conn_key,
                          CF_TRANSFER_KEY transfer_key,
                          const void *data,
                          long long offset,
                          long long length)
{
    CF_OPERATION_INFO opInfo = {};
    opInfo.StructSize = sizeof(opInfo);
    opInfo.Type = CF_OPERATION_TYPE_TRANSFER_DATA;
    opInfo.ConnectionKey = conn_key;
    opInfo.TransferKey = transfer_key;

    CF_OPERATION_PARAMETERS opParams = {};
    opParams.ParamSize = CF_SIZE_OF_OP_PARAM(TransferData);
    opParams.TransferData.CompletionStatus = STATUS_SUCCESS;
    opParams.TransferData.Buffer = data;
    opParams.TransferData.Offset.QuadPart = offset;
    opParams.TransferData.Length.QuadPart = length;

    HRESULT hr = CfExecute(&opInfo, &opParams);
    return static_cast<long>(hr);
}

void cfapi_transfer_error(CF_CONNECTION_KEY conn_key,
                           CF_TRANSFER_KEY transfer_key,
                           long long offset,
                           long hr)
{
    CF_OPERATION_INFO opInfo = {};
    opInfo.StructSize = sizeof(opInfo);
    opInfo.Type = CF_OPERATION_TYPE_TRANSFER_DATA;
    opInfo.ConnectionKey = conn_key;
    opInfo.TransferKey = transfer_key;

    CF_OPERATION_PARAMETERS opParams = {};
    opParams.ParamSize = CF_SIZE_OF_OP_PARAM(TransferData);
    opParams.TransferData.CompletionStatus = static_cast<NTSTATUS>(hr);
    opParams.TransferData.Buffer = nullptr;
    opParams.TransferData.Offset.QuadPart = offset;
    opParams.TransferData.Length.QuadPart = 0;

    CfExecute(&opInfo, &opParams);
}

} /* extern "C" */

#endif /* _WIN32 */
