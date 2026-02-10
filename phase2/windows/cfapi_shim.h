/*
 * cfapi_shim.h - C API wrapper for Windows Cloud Files API (CfAPI)
 *
 * This header provides a C-linkage interface to the Windows CfAPI,
 * callable from Go via CGO. The implementation is in cfapi_shim.cpp.
 *
 * Build requirements:
 *   - Windows 10 1809+ SDK
 *   - Link against: cldapi.lib, ole32.lib
 *
 * This file is committed as source only; it is built on Windows.
 */

#ifndef CFAPI_SHIM_H
#define CFAPI_SHIM_H

#ifdef _WIN32

#include <cfapi.h>
#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Initialize COM for the current thread. Call once before other functions. */
long cfapi_init(void);

/*
 * Register a sync root directory with CfAPI.
 *   sync_root_path: absolute path to the local sync directory (UTF-8)
 *   display_name:   name shown in Explorer (UTF-8)
 *   version:        provider version string (UTF-8)
 * Returns HRESULT (0 = S_OK).
 */
long cfapi_register_sync_root(const char *sync_root_path,
                               const char *display_name,
                               const char *version);

/*
 * Connect to a registered sync root and start receiving callbacks.
 *   sync_root_path: same path used in register (UTF-8)
 *   out_key:        receives the connection key (used for transfers)
 * Returns HRESULT.
 */
long cfapi_connect_sync_root(const char *sync_root_path,
                              CF_CONNECTION_KEY *out_key);

/* Disconnect from a sync root. */
void cfapi_disconnect_sync_root(CF_CONNECTION_KEY key);

/* Unregister a sync root (removes CfAPI association). */
long cfapi_unregister_sync_root(const char *sync_root_path);

/*
 * Create a placeholder file or directory in the sync root.
 *   parent_path: absolute path to parent directory (UTF-8)
 *   name:        file/directory name (UTF-8)
 *   file_identity: opaque identity blob (file ID, UTF-8)
 *   file_size:   file size in bytes (0 for directories)
 *   mtime_unix:  modification time as Unix timestamp
 *   is_directory: 1 for directory, 0 for file
 * Returns HRESULT.
 */
long cfapi_create_placeholder(const char *parent_path,
                               const char *name,
                               const char *file_identity,
                               long long file_size,
                               long long mtime_unix,
                               int is_directory);

/*
 * Update an existing placeholder's metadata.
 *   file_path:     absolute path to the placeholder (UTF-8)
 *   file_identity: new identity blob (UTF-8)
 *   file_size:     new file size
 *   mtime_unix:    new modification time
 * Returns HRESULT.
 */
long cfapi_update_placeholder(const char *file_path,
                               const char *file_identity,
                               long long file_size,
                               long long mtime_unix);

/*
 * Dehydrate a placeholder (remove local content, keep placeholder).
 *   file_path: absolute path to the file (UTF-8)
 * Returns HRESULT.
 */
long cfapi_dehydrate_placeholder(const char *file_path);

/*
 * Transfer data to satisfy a hydration request.
 *   conn_key:     connection key from cfapi_connect_sync_root
 *   transfer_key: transfer key from the hydration callback
 *   data:         pointer to the data buffer
 *   offset:       byte offset in the file
 *   length:       number of bytes to transfer
 * Returns HRESULT.
 */
long cfapi_transfer_data(CF_CONNECTION_KEY conn_key,
                          CF_TRANSFER_KEY transfer_key,
                          const void *data,
                          long long offset,
                          long long length);

/*
 * Report a transfer error to CfAPI.
 *   conn_key:     connection key
 *   transfer_key: transfer key from the hydration callback
 *   offset:       byte offset where error occurred
 *   hr:           HRESULT error code
 */
void cfapi_transfer_error(CF_CONNECTION_KEY conn_key,
                           CF_TRANSFER_KEY transfer_key,
                           long long offset,
                           long hr);

#ifdef __cplusplus
}
#endif

/*
 * Go callback declaration (implemented in cfapi_windows.go via //export).
 * Called by the C++ FetchDataCallback when CfAPI requests file data.
 */
extern void goHydrationCallback(const char *fileIdentity, int fileIdentityLen,
                                 long long offset, long long length,
                                 CF_TRANSFER_KEY transferKey);

#else /* !_WIN32 */

/* Provide empty typedefs so the header can be parsed on non-Windows. */
typedef long long CF_CONNECTION_KEY;
typedef long long CF_TRANSFER_KEY;

#endif /* _WIN32 */

#endif /* CFAPI_SHIM_H */
