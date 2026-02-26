// Transfer Queue — chunked uploads with pause/resume/retry + IndexedDB persistence
var Transfers = (function() {
    var CHUNK_THRESHOLD = 5 * 1024 * 1024; // 5 MB — files below this use single POST
    var DB_NAME = 'fruitsalade-transfers';
    var DB_VERSION = 1;
    var STORE_NAME = 'uploads';
    var CONCURRENT = 1; // sequential uploads
    var db = null;
    var activeXHR = null;
    var activeTransferId = null;
    var transfers = []; // in-memory mirror of IndexedDB
    var panelVisible = false;
    var panelCollapsed = false;
    var listeners = {};
    var autoHideTimer = null;

    // ─── IndexedDB ──────────────────────────────────────────────────────

    function openDB() {
        return new Promise(function(resolve, reject) {
            if (db) { resolve(db); return; }
            var req = indexedDB.open(DB_NAME, DB_VERSION);
            req.onupgradeneeded = function(e) {
                var store = e.target.result.createObjectStore(STORE_NAME, { keyPath: 'id', autoIncrement: true });
                store.createIndex('status', 'status', { unique: false });
            };
            req.onsuccess = function(e) { db = e.target.result; resolve(db); };
            req.onerror = function() { reject(req.error); };
        });
    }

    function dbPut(record) {
        return openDB().then(function(db) {
            return new Promise(function(resolve, reject) {
                var tx = db.transaction(STORE_NAME, 'readwrite');
                var store = tx.objectStore(STORE_NAME);
                var req = store.put(record);
                req.onsuccess = function() { resolve(req.result); };
                req.onerror = function() { reject(req.error); };
            });
        });
    }

    function dbGet(id) {
        return openDB().then(function(db) {
            return new Promise(function(resolve, reject) {
                var tx = db.transaction(STORE_NAME, 'readonly');
                var req = tx.objectStore(STORE_NAME).get(id);
                req.onsuccess = function() { resolve(req.result); };
                req.onerror = function() { reject(req.error); };
            });
        });
    }

    function dbDelete(id) {
        return openDB().then(function(db) {
            return new Promise(function(resolve, reject) {
                var tx = db.transaction(STORE_NAME, 'readwrite');
                var req = tx.objectStore(STORE_NAME).delete(id);
                req.onsuccess = function() { resolve(); };
                req.onerror = function() { reject(req.error); };
            });
        });
    }

    function dbGetAll() {
        return openDB().then(function(db) {
            return new Promise(function(resolve, reject) {
                var tx = db.transaction(STORE_NAME, 'readonly');
                var req = tx.objectStore(STORE_NAME).getAll();
                req.onsuccess = function() { resolve(req.result || []); };
                req.onerror = function() { reject(req.error); };
            });
        });
    }

    // ─── Init ───────────────────────────────────────────────────────────

    function init() {
        dbGetAll().then(function(records) {
            transfers = records;
            // Mark any "uploading" as "paused" (page was reloaded mid-upload)
            var updates = [];
            for (var i = 0; i < transfers.length; i++) {
                if (transfers[i].status === 'uploading') {
                    transfers[i].status = 'paused';
                    transfers[i].needsFile = true;
                    updates.push(dbPut(transfers[i]));
                }
            }
            Promise.all(updates).then(function() {
                updateBadge();
                if (hasPending()) renderPanel();
            });
        });

        window.addEventListener('online', function() {
            resumeAllPaused();
        });
        window.addEventListener('offline', function() {
            pauseActive();
        });
    }

    // ─── Enqueue ────────────────────────────────────────────────────────

    function enqueue(file, path) {
        var record = {
            path: path,
            fileName: file.name,
            fileSize: file.size,
            uploadId: null,       // server-side upload ID (set after init)
            chunkSize: null,
            totalChunks: null,
            completedChunks: [],
            status: 'queued',
            addedAt: Date.now(),
            speed: 0,
            loaded: 0,
            error: null,
            needsFile: false
        };

        // Store file reference in memory (can't persist File objects in IndexedDB)
        return dbPut(record).then(function(id) {
            record.id = id;
            record._file = file; // ephemeral reference
            transfers.push(record);
            updateBadge();
            showPanel();
            renderPanel();
            processQueue();
            return id;
        });
    }

    // ─── Queue Processing ───────────────────────────────────────────────

    function hasPending() {
        for (var i = 0; i < transfers.length; i++) {
            if (transfers[i].status === 'queued' || transfers[i].status === 'uploading' || transfers[i].status === 'paused') return true;
        }
        return false;
    }

    function activeCount() {
        var count = 0;
        for (var i = 0; i < transfers.length; i++) {
            if (transfers[i].status === 'uploading') count++;
        }
        return count;
    }

    function processQueue() {
        if (activeCount() >= CONCURRENT) return;

        for (var i = 0; i < transfers.length; i++) {
            if (transfers[i].status === 'queued' && transfers[i]._file) {
                startUpload(transfers[i]);
                return;
            }
        }
    }

    // ─── Upload Logic ───────────────────────────────────────────────────

    function startUpload(transfer) {
        transfer.status = 'uploading';
        transfer.error = null;
        dbPut(sanitize(transfer));
        renderPanel();
        updateBadge();

        // Small file: use existing single-POST path
        if (transfer.fileSize < CHUNK_THRESHOLD) {
            uploadSmall(transfer);
            return;
        }

        // Large file: chunked upload
        if (transfer.uploadId) {
            // Resume: skip init, go straight to chunks
            resumeChunked(transfer);
        } else {
            initChunked(transfer);
        }
    }

    function uploadSmall(transfer) {
        var file = transfer._file;
        var uploadPath = transfer.path.replace(/^\//, '');
        var url = '/api/v1/content/' + API.encodeURIPath(uploadPath);

        var xhr = new XMLHttpRequest();
        activeXHR = xhr;
        activeTransferId = transfer.id;

        xhr.open('POST', url, true);
        var token = API.getToken();
        if (token) xhr.setRequestHeader('Authorization', 'Bearer ' + token);

        xhr.upload.onprogress = function(e) {
            if (e.lengthComputable) {
                transfer.loaded = e.loaded;
                transfer.speed = e.loaded / ((Date.now() - transfer._startTime) / 1000) || 0;
                renderPanel();
            }
        };

        xhr.onload = function() {
            activeXHR = null;
            activeTransferId = null;
            if (xhr.status >= 200 && xhr.status < 300) {
                completeTransfer(transfer);
            } else {
                failTransfer(transfer, 'Upload failed (HTTP ' + xhr.status + ')');
            }
        };
        xhr.onerror = function() {
            activeXHR = null;
            activeTransferId = null;
            pauseTransfer(transfer, 'Network error');
        };
        xhr.ontimeout = function() {
            activeXHR = null;
            activeTransferId = null;
            pauseTransfer(transfer, 'Timeout');
        };

        transfer._startTime = Date.now();
        xhr.send(file);
    }

    function initChunked(transfer) {
        var body = JSON.stringify({
            path: transfer.path,
            fileName: transfer.fileName,
            fileSize: transfer.fileSize
        });

        fetch('/api/v1/uploads/init', {
            method: 'POST',
            headers: {
                'Authorization': 'Bearer ' + API.getToken(),
                'Content-Type': 'application/json'
            },
            body: body
        }).then(function(resp) {
            if (!resp.ok) {
                return resp.json().then(function(err) {
                    throw new Error(err.error || 'Init failed');
                });
            }
            return resp.json();
        }).then(function(data) {
            transfer.uploadId = data.uploadId;
            transfer.chunkSize = data.chunkSize;
            transfer.totalChunks = data.totalChunks;
            transfer.completedChunks = [];
            dbPut(sanitize(transfer));
            uploadChunks(transfer);
        }).catch(function(err) {
            failTransfer(transfer, err.message || 'Init failed');
        });
    }

    function resumeChunked(transfer) {
        // Ask server which chunks it has
        fetch('/api/v1/uploads/' + transfer.uploadId + '/status', {
            headers: { 'Authorization': 'Bearer ' + API.getToken() }
        }).then(function(resp) {
            if (!resp.ok) throw new Error('Status check failed');
            return resp.json();
        }).then(function(data) {
            if (data.status !== 'active') {
                // Upload expired or was aborted server-side, restart
                transfer.uploadId = null;
                transfer.completedChunks = [];
                dbPut(sanitize(transfer));
                initChunked(transfer);
                return;
            }
            transfer.completedChunks = data.received || [];
            transfer.totalChunks = data.totalChunks;
            dbPut(sanitize(transfer));
            uploadChunks(transfer);
        }).catch(function(err) {
            // Server may have cleaned up, restart
            transfer.uploadId = null;
            transfer.completedChunks = [];
            dbPut(sanitize(transfer));
            initChunked(transfer);
        });
    }

    function uploadChunks(transfer) {
        var file = transfer._file;
        if (!file) {
            transfer.status = 'paused';
            transfer.needsFile = true;
            transfer.error = 'File reference lost — please re-attach the file';
            dbPut(sanitize(transfer));
            renderPanel();
            updateBadge();
            return;
        }

        transfer._startTime = Date.now();
        transfer._startLoaded = (transfer.completedChunks.length || 0) * transfer.chunkSize;
        transfer.loaded = transfer._startLoaded;

        sendNextChunk(transfer, 0);
    }

    function sendNextChunk(transfer, fromIndex) {
        if (transfer.status !== 'uploading') return;

        // Find next chunk that hasn't been completed
        var nextIndex = -1;
        for (var i = fromIndex; i < transfer.totalChunks; i++) {
            if (transfer.completedChunks.indexOf(i) === -1) {
                nextIndex = i;
                break;
            }
        }

        if (nextIndex === -1) {
            // All chunks sent — finalize
            finalizeChunked(transfer);
            return;
        }

        var file = transfer._file;
        var start = nextIndex * transfer.chunkSize;
        var end = Math.min(start + transfer.chunkSize, transfer.fileSize);
        var blob = file.slice(start, end);

        var url = '/api/v1/uploads/' + transfer.uploadId + '/' + nextIndex;
        var xhr = new XMLHttpRequest();
        activeXHR = xhr;
        activeTransferId = transfer.id;

        xhr.open('PUT', url, true);
        xhr.setRequestHeader('Authorization', 'Bearer ' + API.getToken());

        xhr.upload.onprogress = function(e) {
            if (e.lengthComputable) {
                var chunkLoaded = (transfer.completedChunks.length * transfer.chunkSize) + e.loaded;
                transfer.loaded = chunkLoaded;
                var elapsed = (Date.now() - transfer._startTime) / 1000;
                if (elapsed > 0) {
                    transfer.speed = (transfer.loaded - transfer._startLoaded) / elapsed;
                }
                renderPanel();
            }
        };

        xhr.onload = function() {
            activeXHR = null;
            activeTransferId = null;
            if (xhr.status >= 200 && xhr.status < 300) {
                transfer.completedChunks.push(nextIndex);
                dbPut(sanitize(transfer));
                renderPanel();
                // Send next
                sendNextChunk(transfer, nextIndex + 1);
            } else {
                pauseTransfer(transfer, 'Chunk upload failed (HTTP ' + xhr.status + ')');
            }
        };
        xhr.onerror = function() {
            activeXHR = null;
            activeTransferId = null;
            pauseTransfer(transfer, 'Network error');
        };
        xhr.ontimeout = function() {
            activeXHR = null;
            activeTransferId = null;
            pauseTransfer(transfer, 'Timeout');
        };

        xhr.send(blob);
    }

    function finalizeChunked(transfer) {
        fetch('/api/v1/uploads/' + transfer.uploadId + '/complete', {
            method: 'POST',
            headers: { 'Authorization': 'Bearer ' + API.getToken() }
        }).then(function(resp) {
            if (!resp.ok) {
                return resp.json().then(function(err) {
                    throw new Error(err.error || 'Complete failed');
                });
            }
            return resp.json();
        }).then(function() {
            completeTransfer(transfer);
        }).catch(function(err) {
            failTransfer(transfer, err.message || 'Finalize failed');
        });
    }

    // ─── State Transitions ──────────────────────────────────────────────

    function completeTransfer(transfer) {
        transfer.status = 'completed';
        transfer.loaded = transfer.fileSize;
        transfer.speed = 0;
        dbPut(sanitize(transfer));
        renderPanel();
        updateBadge();
        Toast.success('Uploaded: ' + transfer.fileName);

        // Trigger file list reload via custom event
        window.dispatchEvent(new CustomEvent('transfer-complete', {
            detail: { path: transfer.path }
        }));

        processQueue();
        scheduleAutoHide();
    }

    function failTransfer(transfer, msg) {
        transfer.status = 'failed';
        transfer.error = msg;
        transfer.speed = 0;
        dbPut(sanitize(transfer));
        renderPanel();
        updateBadge();
        Toast.error('Upload failed: ' + transfer.fileName + ' — ' + msg);
        processQueue();
    }

    function pauseTransfer(transfer, reason) {
        transfer.status = 'paused';
        transfer.error = reason || null;
        transfer.speed = 0;
        dbPut(sanitize(transfer));
        renderPanel();
        updateBadge();
        processQueue();
    }

    // ─── User Actions ───────────────────────────────────────────────────

    function pause(id) {
        var t = findTransfer(id);
        if (!t || t.status !== 'uploading') return;

        // Abort active XHR if this is the active upload
        if (activeTransferId === id && activeXHR) {
            activeXHR.abort();
            activeXHR = null;
            activeTransferId = null;
        }

        pauseTransfer(t, 'Paused by user');
    }

    function resume(id, file) {
        var t = findTransfer(id);
        if (!t) return;
        if (t.status !== 'paused' && t.status !== 'failed') return;

        if (file) {
            t._file = file;
            t.needsFile = false;
        }

        if (!t._file) {
            t.needsFile = true;
            t.error = 'File reference lost — please re-attach the file';
            dbPut(sanitize(t));
            renderPanel();
            return;
        }

        t.status = 'queued';
        t.error = null;
        dbPut(sanitize(t));
        renderPanel();
        updateBadge();
        processQueue();
    }

    function retry(id) {
        resume(id);
    }

    function cancel(id) {
        var t = findTransfer(id);
        if (!t) return;

        // Abort active XHR
        if (activeTransferId === id && activeXHR) {
            activeXHR.abort();
            activeXHR = null;
            activeTransferId = null;
        }

        // Abort server-side if chunked
        if (t.uploadId) {
            fetch('/api/v1/uploads/' + t.uploadId, {
                method: 'DELETE',
                headers: { 'Authorization': 'Bearer ' + API.getToken() }
            }).catch(function() {});
        }

        // Remove from memory and DB
        var idx = transfers.indexOf(t);
        if (idx !== -1) transfers.splice(idx, 1);
        dbDelete(id);
        renderPanel();
        updateBadge();
        processQueue();
    }

    function pauseAll() {
        for (var i = 0; i < transfers.length; i++) {
            if (transfers[i].status === 'uploading' || transfers[i].status === 'queued') {
                pause(transfers[i].id);
                if (transfers[i].status === 'queued') {
                    transfers[i].status = 'paused';
                    dbPut(sanitize(transfers[i]));
                }
            }
        }
        renderPanel();
        updateBadge();
    }

    function resumeAll() {
        for (var i = 0; i < transfers.length; i++) {
            if (transfers[i].status === 'paused' && !transfers[i].needsFile) {
                transfers[i].status = 'queued';
                transfers[i].error = null;
                dbPut(sanitize(transfers[i]));
            }
        }
        renderPanel();
        updateBadge();
        processQueue();
    }

    function resumeAllPaused() {
        resumeAll();
    }

    function pauseActive() {
        for (var i = 0; i < transfers.length; i++) {
            if (transfers[i].status === 'uploading') {
                pause(transfers[i].id);
            }
        }
    }

    function clearCompleted() {
        var remaining = [];
        for (var i = 0; i < transfers.length; i++) {
            if (transfers[i].status === 'completed') {
                dbDelete(transfers[i].id);
            } else {
                remaining.push(transfers[i]);
            }
        }
        transfers = remaining;
        renderPanel();
        updateBadge();
        if (!hasPending()) hidePanel();
    }

    // ─── Helpers ────────────────────────────────────────────────────────

    function findTransfer(id) {
        for (var i = 0; i < transfers.length; i++) {
            if (transfers[i].id === id) return transfers[i];
        }
        return null;
    }

    // Strip ephemeral fields before storing in IndexedDB
    function sanitize(t) {
        var copy = {};
        for (var k in t) {
            if (k.charAt(0) !== '_' && t.hasOwnProperty(k)) copy[k] = t[k];
        }
        return copy;
    }

    function formatSpeed(bps) {
        if (!bps || bps <= 0) return '';
        if (bps < 1024) return Math.round(bps) + ' B/s';
        if (bps < 1024 * 1024) return (bps / 1024).toFixed(1) + ' KB/s';
        return (bps / (1024 * 1024)).toFixed(1) + ' MB/s';
    }

    function formatETA(bytes, speed) {
        if (!speed || speed <= 0) return '';
        var secs = Math.ceil(bytes / speed);
        if (secs < 60) return '< 1 min';
        var mins = Math.floor(secs / 60);
        var s = secs % 60;
        return mins + ':' + (s < 10 ? '0' : '') + s;
    }

    function scheduleAutoHide() {
        clearTimeout(autoHideTimer);
        autoHideTimer = setTimeout(function() {
            var allDone = true;
            for (var i = 0; i < transfers.length; i++) {
                if (transfers[i].status !== 'completed' && transfers[i].status !== 'failed') {
                    allDone = false;
                    break;
                }
            }
            if (allDone) hidePanel();
        }, 5000);
    }

    // ─── Panel UI ───────────────────────────────────────────────────────

    function showPanel() {
        panelVisible = true;
        var panel = document.getElementById('transfer-panel');
        if (panel) panel.classList.remove('hidden');
        var btn = document.getElementById('transfer-btn');
        if (btn) btn.classList.remove('hidden');
    }

    function hidePanel() {
        panelVisible = false;
        var panel = document.getElementById('transfer-panel');
        if (panel) panel.classList.add('hidden');
    }

    function togglePanel() {
        if (panelVisible) {
            hidePanel();
        } else {
            showPanel();
            renderPanel();
        }
    }

    function updateBadge() {
        var btn = document.getElementById('transfer-btn');
        var badge = document.getElementById('transfer-badge');
        if (!btn || !badge) return;

        var active = 0;
        for (var i = 0; i < transfers.length; i++) {
            if (transfers[i].status === 'queued' || transfers[i].status === 'uploading' || transfers[i].status === 'paused') {
                active++;
            }
        }

        if (active > 0) {
            btn.classList.remove('hidden');
            badge.textContent = active;
            badge.classList.remove('hidden');
        } else if (transfers.length > 0) {
            btn.classList.remove('hidden');
            badge.classList.add('hidden');
        } else {
            btn.classList.add('hidden');
            badge.classList.add('hidden');
        }
    }

    var renderQueued = false;
    function renderPanel() {
        if (renderQueued) return;
        renderQueued = true;
        requestAnimationFrame(function() {
            renderQueued = false;
            doRenderPanel();
        });
    }

    function doRenderPanel() {
        var panel = document.getElementById('transfer-panel');
        if (!panel) return;

        if (transfers.length === 0) {
            panel.innerHTML = '';
            hidePanel();
            return;
        }

        // Count stats
        var activeCount = 0, totalCount = transfers.length, completedCount = 0;
        var totalBytes = 0, loadedBytes = 0;
        for (var i = 0; i < transfers.length; i++) {
            var t = transfers[i];
            totalBytes += t.fileSize;
            loadedBytes += t.loaded || 0;
            if (t.status === 'uploading') activeCount++;
            if (t.status === 'completed') completedCount++;
        }

        var html = '<div class="transfer-panel-header">' +
            '<span class="transfer-panel-title">Transfers (' + completedCount + '/' + totalCount + ')</span>' +
            '<div class="transfer-panel-actions">';

        if (activeCount > 0) {
            html += '<button class="transfer-action-btn" data-action="pause-all" title="Pause All">&#9646;&#9646;</button>';
        } else {
            html += '<button class="transfer-action-btn" data-action="resume-all" title="Resume All">&#9654;</button>';
        }
        html += '<button class="transfer-action-btn" data-action="clear-completed" title="Clear Completed">&#10005;</button>' +
            '<button class="transfer-action-btn" data-action="close-panel" title="Close">&times;</button>' +
            '</div></div>';

        html += '<div class="transfer-panel-list">';
        for (var j = 0; j < transfers.length; j++) {
            html += renderTransferItem(transfers[j]);
        }
        html += '</div>';

        panel.innerHTML = html;

        // Bind actions
        panel.querySelectorAll('[data-action]').forEach(function(btn) {
            btn.addEventListener('click', function(e) {
                var action = e.currentTarget.getAttribute('data-action');
                var id = parseInt(e.currentTarget.getAttribute('data-id'), 10);
                switch (action) {
                    case 'pause-all': pauseAll(); break;
                    case 'resume-all': resumeAll(); break;
                    case 'clear-completed': clearCompleted(); break;
                    case 'close-panel': hidePanel(); break;
                    case 'pause': pause(id); break;
                    case 'resume': resume(id); break;
                    case 'retry': retry(id); break;
                    case 'cancel': cancel(id); break;
                    case 'reattach': promptReattach(id); break;
                }
            });
        });
    }

    function renderTransferItem(t) {
        var pct = t.fileSize > 0 ? Math.round((t.loaded || 0) / t.fileSize * 100) : 0;
        var statusClass = 'transfer-status-' + t.status;
        var barClass = 'transfer-progress-fill';
        if (t.status === 'completed') barClass += ' transfer-progress-complete';
        else if (t.status === 'failed') barClass += ' transfer-progress-failed';
        else if (t.status === 'paused') barClass += ' transfer-progress-paused';

        var html = '<div class="transfer-item ' + statusClass + '">' +
            '<div class="transfer-item-info">' +
            '<span class="transfer-item-icon">' + FileTypes.icon(t.fileName, false) + '</span>' +
            '<div class="transfer-item-details">' +
            '<span class="transfer-item-name" title="' + esc(t.path) + '">' + esc(t.fileName) + '</span>' +
            '<div class="transfer-progress"><div class="' + barClass + '" style="width:' + pct + '%"></div></div>' +
            '<div class="transfer-item-meta">';

        // Status text line
        if (t.status === 'uploading') {
            html += '<span>' + formatBytes(t.loaded || 0) + ' / ' + formatBytes(t.fileSize) + '</span>';
            if (t.speed > 0) {
                html += ' <span class="transfer-speed">' + formatSpeed(t.speed) + '</span>';
                var remaining = t.fileSize - (t.loaded || 0);
                var eta = formatETA(remaining, t.speed);
                if (eta) html += ' <span class="transfer-eta">' + eta + '</span>';
            }
        } else if (t.status === 'completed') {
            html += '<span>' + formatBytes(t.fileSize) + ' — Complete</span>';
        } else if (t.status === 'failed') {
            html += '<span class="transfer-error">' + esc(t.error || 'Failed') + '</span>';
        } else if (t.status === 'paused') {
            html += '<span>' + formatBytes(t.loaded || 0) + ' / ' + formatBytes(t.fileSize) + ' — Paused</span>';
            if (t.error) html += ' <span class="transfer-error">' + esc(t.error) + '</span>';
        } else if (t.status === 'queued') {
            html += '<span>Queued — ' + formatBytes(t.fileSize) + '</span>';
        }

        html += '</div></div></div>'; // close details, info

        // Action buttons
        html += '<div class="transfer-item-actions">';
        if (t.status === 'uploading') {
            html += '<button class="transfer-item-btn" data-action="pause" data-id="' + t.id + '" title="Pause">&#9646;&#9646;</button>';
        } else if (t.status === 'paused') {
            if (t.needsFile) {
                html += '<button class="transfer-item-btn" data-action="reattach" data-id="' + t.id + '" title="Re-attach file">&#128206;</button>';
            } else {
                html += '<button class="transfer-item-btn" data-action="resume" data-id="' + t.id + '" title="Resume">&#9654;</button>';
            }
        } else if (t.status === 'failed') {
            html += '<button class="transfer-item-btn" data-action="retry" data-id="' + t.id + '" title="Retry">&#8635;</button>';
        }
        if (t.status !== 'completed') {
            html += '<button class="transfer-item-btn transfer-item-btn-cancel" data-action="cancel" data-id="' + t.id + '" title="Cancel">&times;</button>';
        }
        html += '</div></div>'; // close actions, item

        return html;
    }

    function promptReattach(id) {
        var t = findTransfer(id);
        if (!t) return;
        var input = document.createElement('input');
        input.type = 'file';
        input.addEventListener('change', function() {
            if (input.files && input.files[0]) {
                resume(id, input.files[0]);
            }
        });
        input.click();
    }

    // ─── Public API ─────────────────────────────────────────────────────

    return {
        init: init,
        enqueue: enqueue,
        pause: pause,
        resume: resume,
        retry: retry,
        cancel: cancel,
        pauseAll: pauseAll,
        resumeAll: resumeAll,
        clearCompleted: clearCompleted,
        togglePanel: togglePanel,
        showPanel: showPanel,
        hidePanel: hidePanel,
        getTransfers: function() { return transfers; }
    };
})();
