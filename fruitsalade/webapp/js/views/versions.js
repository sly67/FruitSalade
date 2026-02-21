// File Management — Versions tab + Conflicts tab
function renderVersions() {
    var hash = window.location.hash.replace('#versions', '').replace(/^\//, '');

    // If viewing a specific file's versions, go directly there (no tabs)
    if (hash) {
        var filePath = '/' + decodeURIComponent(hash);
        renderFileVersions(filePath);
        return;
    }

    // Top-level: show tabs
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar"><h2>File Management</h2></div>' +
        '<div class="fm-tab-nav">' +
            '<button class="fm-tab active" data-fm-tab="versions">Versions</button>' +
            '<button class="fm-tab" data-fm-tab="conflicts">Conflicts</button>' +
        '</div>' +
        '<div id="fm-tab-content"></div>';

    // Wire tab clicks
    var tabs = app.querySelectorAll('.fm-tab');
    tabs.forEach(function(tab) {
        tab.addEventListener('click', function() {
            tabs.forEach(function(t) { t.classList.remove('active'); });
            tab.classList.add('active');
            var target = tab.getAttribute('data-fm-tab');
            if (target === 'versions') renderVersionExplorerTab();
            else renderConflictsTab();
        });
    });

    // Default tab
    renderVersionExplorerTab();
}

// ─── Version Explorer Tab ───────────────────────────────────────────────────

function renderVersionExplorerTab() {
    var container = document.getElementById('fm-tab-content');
    container.innerHTML =
        '<div style="padding:0 1.5rem">' +
            '<div class="toolbar-actions" style="margin-bottom:0.75rem">' +
                '<div class="search-wrap">' +
                    '<input type="text" id="ver-search" placeholder="Filter files...">' +
                '</div>' +
            '</div>' +
            '<div id="ver-stats" class="ver-stats"></div>' +
            '<div id="ver-table" class="table-wrap">Loading...</div>' +
        '</div>';

    API.get('/api/v1/versions').then(function(files) {
        if (!files || files.length === 0) {
            document.getElementById('ver-table').innerHTML =
                '<p style="padding:1.5rem;color:var(--text-muted)">No files with version history yet. ' +
                'Versions are created automatically when files are modified.</p>';
            document.getElementById('ver-stats').innerHTML = '';
            return;
        }

        // Stats
        var totalVersions = 0;
        for (var i = 0; i < files.length; i++) {
            totalVersions += files[i].version_count;
        }
        document.getElementById('ver-stats').innerHTML =
            '<div class="ver-stat"><strong>' + files.length + '</strong> versioned files</div>' +
            '<div class="ver-stat"><strong>' + totalVersions + '</strong> total versions</div>';

        renderVersionedFileList(files);

        // Search filter
        document.getElementById('ver-search').addEventListener('input', function() {
            var q = this.value.toLowerCase();
            if (!q) {
                renderVersionedFileList(files);
                return;
            }
            var filtered = files.filter(function(f) {
                return f.name.toLowerCase().indexOf(q) !== -1 ||
                       f.path.toLowerCase().indexOf(q) !== -1;
            });
            renderVersionedFileList(filtered);
        });
    }).catch(function() {
        document.getElementById('ver-table').innerHTML =
            '<div class="alert alert-error">Failed to load versioned files</div>';
    });
}

function renderVersionedFileList(files) {
    var table = document.getElementById('ver-table');
    if (!files || files.length === 0) {
        table.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">No matching files</p>';
        return;
    }

    var html = '<table class="responsive-table"><thead><tr>' +
        '<th>File</th><th>Path</th><th>Current Version</th><th>History</th><th>Size</th><th>Last Changed</th><th></th>' +
        '</tr></thead><tbody>';

    for (var i = 0; i < files.length; i++) {
        var f = files[i];
        html += '<tr class="file-row">' +
            '<td data-label="File"><a class="file-name" href="#versions' + esc(f.path) + '">' +
                FileTypes.icon(f.name, false) + esc(f.name) + '</a></td>' +
            '<td data-label="Path" class="ver-path"><code>' + esc(f.path) + '</code></td>' +
            '<td data-label="Version">v' + f.current_version + '</td>' +
            '<td data-label="History"><span class="badge badge-blue">' + f.version_count + ' version' + (f.version_count !== 1 ? 's' : '') + '</span></td>' +
            '<td data-label="Size">' + formatBytes(f.size) + '</td>' +
            '<td data-label="Changed">' + formatDate(f.latest_change) + '</td>' +
            '<td data-label=""><a class="btn btn-sm btn-outline" href="#versions' + esc(f.path) + '">View</a></td>' +
            '</tr>';
    }

    html += '</tbody></table>';
    table.innerHTML = html;
}

// ─── File Version History ───────────────────────────────────────────────────

function renderFileVersions(filePath) {
    var fileName = filePath.split('/').pop();
    var parentDir = filePath.substring(0, filePath.lastIndexOf('/'));

    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Version History</h2>' +
            '<div class="toolbar-actions">' +
                '<a href="#versions" class="btn btn-sm btn-outline">All Files</a>' +
                '<a href="#browser' + esc(parentDir) + '" class="btn btn-sm btn-outline">Open Folder</a>' +
                '<a href="#viewer' + esc(filePath) + '" class="btn btn-sm btn-outline">View File</a>' +
            '</div>' +
        '</div>' +
        '<div class="ver-file-header">' +
            '<span style="font-size:1.5rem">' + FileTypes.icon(fileName, false) + '</span>' +
            '<div>' +
                '<div class="ver-file-name">' + esc(fileName) + '</div>' +
                '<div class="ver-file-path"><code>' + esc(filePath) + '</code></div>' +
            '</div>' +
        '</div>' +
        '<div id="version-status"></div>' +
        '<div id="version-table">Loading...</div>' +
        '<div id="version-diff-area"></div>';

    loadFileVersions(filePath);
}

function loadFileVersions(filePath) {
    API.get('/api/v1/versions/' + API.encodeURIPath(filePath.replace(/^\//, '')))
        .then(function(data) {
            if (data.error) {
                document.getElementById('version-table').innerHTML =
                    '<div class="alert alert-error">' + esc(data.error) + '</div>';
                return;
            }
            renderVersionTimeline(data, filePath);
        }).catch(function() {
            document.getElementById('version-table').innerHTML =
                '<div class="alert alert-error">Failed to load versions</div>';
        });
}

function renderVersionTimeline(data, filePath) {
    var versions = data.versions || [];
    var current = data.current_version;
    var table = document.getElementById('version-table');

    if (versions.length === 0) {
        table.innerHTML =
            '<div class="ver-empty">' +
                '<p>No previous versions</p>' +
                '<p class="props-muted">Current version: v' + current + '. Versions are created when the file is modified.</p>' +
            '</div>';
        return;
    }

    // Build timeline
    var html = '<div class="ver-timeline">';

    // Current version header
    html += '<div class="ver-timeline-item ver-current">' +
        '<div class="ver-timeline-dot ver-dot-current"></div>' +
        '<div class="ver-timeline-content">' +
            '<div class="ver-timeline-header">' +
                '<span class="ver-version-label">v' + current + '</span>' +
                '<span class="badge badge-green">Current</span>' +
            '</div>' +
            '<div class="ver-timeline-meta">Latest version</div>' +
        '</div>' +
        '</div>';

    // Previous versions
    for (var i = 0; i < versions.length; i++) {
        var v = versions[i];
        var isText = FileTypes.isText(filePath);

        html += '<div class="ver-timeline-item">' +
            '<div class="ver-timeline-dot"></div>' +
            '<div class="ver-timeline-content">' +
                '<div class="ver-timeline-header">' +
                    '<span class="ver-version-label">v' + v.version + '</span>' +
                    '<span class="ver-timeline-date">' + formatDate(v.created_at) + '</span>' +
                '</div>' +
                '<div class="ver-timeline-meta">' +
                    formatBytes(v.size) + ' &middot; ' +
                    '<code class="props-hash">' + esc((v.hash || '').substring(0, 16)) + '</code>' +
                '</div>' +
                '<div class="ver-timeline-actions">' +
                    '<a class="btn btn-sm btn-outline" href="/api/v1/versions/' +
                        esc(API.encodeURIPath(filePath.replace(/^\//, ''))) +
                        '?v=' + v.version + '&token=' + encodeURIComponent(API.getToken()) +
                        '" download>Download</a>' +
                    (isText ? '<button class="btn btn-sm btn-outline" data-action="preview" data-version="' + v.version + '">Preview</button>' : '') +
                    (isText && v.version < current ? '<button class="btn btn-sm btn-outline" data-action="diff" data-version="' + v.version + '">Compare with current</button>' : '') +
                    (v.version !== current ?
                        '<button class="btn btn-sm" data-action="rollback" data-version="' + v.version + '">Rollback</button>' : '') +
                '</div>' +
            '</div>' +
            '</div>';
    }

    html += '</div>';
    table.innerHTML = html;

    // Wire actions
    table.querySelectorAll('[data-action]').forEach(function(btn) {
        btn.addEventListener('click', function(e) {
            var action = e.currentTarget.getAttribute('data-action');
            var version = parseInt(e.currentTarget.getAttribute('data-version'), 10);

            if (action === 'rollback') {
                handleRollback(filePath, version);
            } else if (action === 'preview') {
                handlePreview(filePath, version);
            } else if (action === 'diff') {
                handleDiff(filePath, version);
            }
        });
    });
}

function handleRollback(filePath, version) {
    if (!confirm('Rollback to version ' + version + '? The current state will be saved as a new version.')) return;

    API.post('/api/v1/versions/' + API.encodeURIPath(filePath.replace(/^\//, '')),
        { version: version })
        .then(function(resp) { return resp.json(); })
        .then(function(data) {
            if (data.error) {
                document.getElementById('version-status').innerHTML =
                    '<div class="alert alert-error">' + esc(data.error) + '</div>';
                return;
            }
            document.getElementById('version-status').innerHTML =
                '<div class="alert alert-success">Rolled back to v' + version +
                '. New version: v' + data.new_version + '</div>';
            loadFileVersions(filePath);
        });
}

function handlePreview(filePath, version) {
    var area = document.getElementById('version-diff-area');
    area.innerHTML = '<div class="ver-diff-panel"><div class="ver-diff-header">' +
        '<span>Preview: v' + version + '</span>' +
        '<button class="btn btn-sm btn-outline" id="close-preview">Close</button>' +
        '</div><div class="ver-diff-body"><p class="props-muted">Loading...</p></div></div>';

    document.getElementById('close-preview').addEventListener('click', function() {
        area.innerHTML = '';
    });

    var url = '/api/v1/versions/' + API.encodeURIPath(filePath.replace(/^\//, '')) + '?v=' + version;
    API.request('GET', url).then(function(resp) { return resp.text(); }).then(function(text) {
        var body = area.querySelector('.ver-diff-body');
        if (body) {
            body.innerHTML = '<pre class="ver-code">' + esc(text) + '</pre>';
        }
    }).catch(function() {
        var body = area.querySelector('.ver-diff-body');
        if (body) body.innerHTML = '<div class="alert alert-error">Failed to load version content</div>';
    });
}

function handleDiff(filePath, version) {
    var area = document.getElementById('version-diff-area');
    area.innerHTML = '<div class="ver-diff-panel"><div class="ver-diff-header">' +
        '<span>Comparing v' + version + ' with current</span>' +
        '<button class="btn btn-sm btn-outline" id="close-diff">Close</button>' +
        '</div><div class="ver-diff-body"><p class="props-muted">Loading both versions...</p></div></div>';

    document.getElementById('close-diff').addEventListener('click', function() {
        area.innerHTML = '';
    });

    // Fetch both versions in parallel
    var oldUrl = '/api/v1/versions/' + API.encodeURIPath(filePath.replace(/^\//, '')) + '?v=' + version;
    var currentUrl = '/api/v1/content/' + API.encodeURIPath(filePath.replace(/^\//, ''));

    Promise.all([
        API.request('GET', oldUrl).then(function(r) { return r.text(); }),
        API.request('GET', currentUrl).then(function(r) { return r.text(); })
    ]).then(function(results) {
        var oldText = results[0];
        var newText = results[1];
        var body = area.querySelector('.ver-diff-body');
        if (!body) return;

        var diffHtml = computeDiff(oldText, newText, version);
        body.innerHTML = diffHtml;
    }).catch(function() {
        var body = area.querySelector('.ver-diff-body');
        if (body) body.innerHTML = '<div class="alert alert-error">Failed to load version content</div>';
    });
}

// ─── Conflicts Tab ──────────────────────────────────────────────────────────

// Module-scoped state for the conflict compare overlay
var _ccOverlay = null;
var _ccConflicts = [];
var _ccIndex = 0;
var _ccKeyHandler = null;

function renderConflictsTab() {
    var container = document.getElementById('fm-tab-content');
    container.innerHTML = '<div class="fm-conflicts-loading">Scanning for conflicts...</div>';

    API.get('/api/v1/tree').then(function(tree) {
        var conflicts = findConflictFiles(tree.root);
        if (conflicts.length === 0) {
            container.innerHTML =
                '<div class="fm-conflicts-empty">' +
                    '<p>No conflicts found</p>' +
                    '<p class="props-muted">Conflicts are created when two devices edit the same file simultaneously.</p>' +
                '</div>';
            return;
        }
        _ccConflicts = conflicts;
        renderConflictList(container, conflicts);
    }).catch(function() {
        container.innerHTML = '<div class="alert alert-error" style="margin:1.5rem">Failed to load file tree</div>';
    });
}

function flattenTree(node, result) {
    result = result || [];
    if (!node) return result;
    if (!node.is_dir) {
        result.push(node);
    }
    if (node.children) {
        for (var i = 0; i < node.children.length; i++) {
            flattenTree(node.children[i], result);
        }
    }
    return result;
}

function findConflictFiles(tree) {
    var allFiles = flattenTree(tree);
    var conflicts = [];
    var re = /^(.+) \(conflict (\d{4}-\d{2}-\d{2})\)(\.[^.]*)?$/;

    for (var i = 0; i < allFiles.length; i++) {
        var f = allFiles[i];
        var name = f.name;
        var match = name.match(re);
        if (!match) continue;

        var originalBase = match[1];
        var date = match[2];
        var ext = match[3] || '';
        var dir = f.path.substring(0, f.path.lastIndexOf('/'));
        var originalPath = dir + '/' + originalBase + ext;

        // Find the original file in the tree
        var original = null;
        for (var j = 0; j < allFiles.length; j++) {
            if (allFiles[j].path === originalPath) {
                original = allFiles[j];
                break;
            }
        }

        conflicts.push({
            conflictFile: f,
            originalFile: original,
            originalPath: originalPath,
            conflictDate: date
        });
    }

    // Sort by date descending (newest first)
    conflicts.sort(function(a, b) { return b.conflictDate.localeCompare(a.conflictDate); });
    return conflicts;
}

// ─── Resolution Functions (Promise-returning) ───────────────────────────────

function resolveKeepOriginal(c) {
    var conflictApiPath = c.conflictFile.path.replace(/^\//, '');
    return API.del('/api/v1/tree/' + API.encodeURIPath(conflictApiPath))
        .then(function(resp) {
            if (!resp.ok) throw new Error('Delete failed');
        });
}

function resolveKeepConflict(c) {
    var conflictApiPath = c.conflictFile.path.replace(/^\//, '');
    var originalApiPath = c.originalPath.replace(/^\//, '');

    return API.request('GET', '/api/v1/content/' + API.encodeURIPath(conflictApiPath))
        .then(function(resp) {
            if (!resp.ok) throw new Error('Failed to download conflict file');
            return resp.blob();
        })
        .then(function(blob) {
            return API.request('POST', '/api/v1/content/' + API.encodeURIPath(originalApiPath), undefined, blob);
        })
        .then(function(resp) {
            if (!resp.ok) throw new Error('Failed to upload to original path');
            return API.del('/api/v1/tree/' + API.encodeURIPath(conflictApiPath));
        })
        .then(function(resp) {
            if (!resp.ok) throw new Error('Failed to delete conflict copy');
        });
}

function handleKeepOriginal(c) {
    if (!confirm('Delete the conflict copy?\n\n' + c.conflictFile.path)) return;
    resolveKeepOriginal(c).then(function() {
        Toast.success('Conflict resolved: kept original');
        renderConflictsTab();
    }).catch(function() {
        Toast.error('Failed to delete conflict copy');
    });
}

function handleKeepConflict(c) {
    var msg = c.originalFile
        ? 'Replace the original with the conflict copy?\n\nOriginal: ' + c.originalPath + '\nConflict: ' + c.conflictFile.path
        : 'Rename conflict copy to original path?\n\n' + c.conflictFile.path + ' -> ' + c.originalPath;
    if (!confirm(msg)) return;
    resolveKeepConflict(c).then(function() {
        Toast.success('Conflict resolved: kept conflict copy');
        renderConflictsTab();
    }).catch(function(err) {
        Toast.error('Failed to resolve conflict: ' + err.message);
    });
}

// ─── Folder Grouping ────────────────────────────────────────────────────────

function groupConflictsByFolder(conflicts) {
    var groups = {};
    for (var i = 0; i < conflicts.length; i++) {
        var c = conflicts[i];
        var dir = c.conflictFile.path.substring(0, c.conflictFile.path.lastIndexOf('/')) || '/';
        if (!groups[dir]) groups[dir] = [];
        groups[dir].push(c);
    }
    var folders = Object.keys(groups).sort();
    return { groups: groups, folders: folders };
}

// ─── Batch Resolution ───────────────────────────────────────────────────────

function batchResolve(items, resolveFn, label) {
    if (!confirm('Resolve ' + items.length + ' conflict(s) by keeping ' + label + '?')) return;
    var chain = Promise.resolve();
    for (var i = 0; i < items.length; i++) {
        (function(c) {
            chain = chain.then(function() { return resolveFn(c); });
        })(items[i]);
    }
    chain.then(function() {
        Toast.success(items.length + ' conflict(s) resolved');
        renderConflictsTab();
    }).catch(function(err) {
        Toast.error('Batch resolve failed: ' + err.message);
        renderConflictsTab();
    });
}

// ─── View Mode ──────────────────────────────────────────────────────────────

function getConflictViewMode() {
    return localStorage.getItem('conflict-view-mode') || 'cards';
}

function setConflictViewMode(mode) {
    localStorage.setItem('conflict-view-mode', mode);
}

// ─── Conflict List Rendering ────────────────────────────────────────────────

function renderConflictList(container, conflicts) {
    var mode = getConflictViewMode();
    var grouped = groupConflictsByFolder(conflicts);

    // Build a flat array in grouped (folder-alphabetical) order
    // so data-idx matches the array used for overlay navigation
    var flatConflicts = [];
    for (var fi = 0; fi < grouped.folders.length; fi++) {
        var items = grouped.groups[grouped.folders[fi]];
        for (var ci = 0; ci < items.length; ci++) {
            items[ci]._flatIdx = flatConflicts.length;
            flatConflicts.push(items[ci]);
        }
    }

    var html = '<div class="fm-conflicts-toolbar">' +
        '<span class="badge badge-orange">' + conflicts.length + ' conflict' +
        (conflicts.length !== 1 ? 's' : '') + '</span>' +
        '<div class="gallery-view-toggles">' +
            '<button class="gallery-view-btn' + (mode === 'list' ? ' active' : '') + '" data-mode="list" title="List">&#9776;</button>' +
            '<button class="gallery-view-btn' + (mode === 'cards' ? ' active' : '') + '" data-mode="cards" title="Cards">&#9638;</button>' +
            '<button class="gallery-view-btn' + (mode === 'detailed' ? ' active' : '') + '" data-mode="detailed" title="Detailed">&#9641;</button>' +
        '</div>' +
    '</div>';

    html += '<div id="fm-conflict-body"></div>';
    container.innerHTML = html;

    // Wire view toggle
    container.querySelectorAll('.gallery-view-btn').forEach(function(btn) {
        btn.addEventListener('click', function() {
            container.querySelectorAll('.gallery-view-btn').forEach(function(b) { b.classList.remove('active'); });
            btn.classList.add('active');
            var newMode = btn.getAttribute('data-mode');
            setConflictViewMode(newMode);
            renderConflictBody(grouped, flatConflicts, newMode);
        });
    });

    renderConflictBody(grouped, flatConflicts, mode);
}

function renderConflictBody(grouped, allConflicts, mode) {
    var body = document.getElementById('fm-conflict-body');
    var html = '';

    for (var fi = 0; fi < grouped.folders.length; fi++) {
        var folder = grouped.folders[fi];
        var items = grouped.groups[folder];

        // Folder header
        html += '<div class="fm-conflict-folder" data-folder="' + esc(folder) + '">' +
            '<div class="fm-conflict-folder-header">' +
                '<button class="fm-conflict-folder-toggle" title="Toggle folder">&#9660;</button>' +
                '<span class="fm-conflict-folder-path">' + esc(folder) + '</span>' +
                '<span class="badge badge-orange" style="margin-left:0.5rem">' + items.length + '</span>' +
                '<div class="fm-conflict-folder-actions">' +
                    '<button class="btn btn-sm btn-outline" data-batch="keep-original" data-folder="' + esc(folder) + '">Keep All Originals</button>' +
                    '<button class="btn btn-sm btn-outline" data-batch="keep-conflict" data-folder="' + esc(folder) + '">Keep All Conflicts</button>' +
                '</div>' +
            '</div>' +
            '<div class="fm-conflict-folder-body">';

        if (mode === 'list') {
            html += renderConflictListMode(items);
        } else if (mode === 'detailed') {
            html += renderConflictDetailedMode(items);
        } else {
            html += renderConflictCardsMode(items);
        }

        html += '</div></div>';
    }

    body.innerHTML = html;

    // Wire folder toggles
    body.querySelectorAll('.fm-conflict-folder-toggle').forEach(function(btn) {
        btn.addEventListener('click', function() {
            var folder = btn.closest('.fm-conflict-folder');
            folder.classList.toggle('collapsed');
        });
    });

    // Wire batch actions
    body.querySelectorAll('[data-batch]').forEach(function(btn) {
        btn.addEventListener('click', function() {
            var action = btn.getAttribute('data-batch');
            var folderPath = btn.getAttribute('data-folder');
            var items = grouped.groups[folderPath];
            if (action === 'keep-original') {
                var origItems = items.filter(function(c) { return c.originalFile; });
                if (origItems.length === 0) {
                    Toast.error('No conflicts with originals to keep');
                    return;
                }
                batchResolve(origItems, resolveKeepOriginal, 'originals');
            } else {
                batchResolve(items, resolveKeepConflict, 'conflict copies');
            }
        });
    });

    // Wire item actions
    body.querySelectorAll('[data-action]').forEach(function(btn) {
        btn.addEventListener('click', function() {
            var action = btn.getAttribute('data-action');
            var idx = parseInt(btn.getAttribute('data-idx'), 10);
            var c = allConflicts[idx];
            switch (action) {
                case 'compare':
                    openConflictOverlay(allConflicts, idx);
                    break;
                case 'keep-original':
                    handleKeepOriginal(c);
                    break;
                case 'keep-conflict':
                    handleKeepConflict(c);
                    break;
            }
        });
    });
}

// ─── List Mode ──────────────────────────────────────────────────────────────

function renderConflictListMode(items) {
    var html = '<table class="fm-conflict-list responsive-table"><thead><tr>' +
        '<th></th><th>File</th><th>Folder</th><th>Date</th><th>Sizes</th><th></th>' +
        '</tr></thead><tbody>';

    for (var i = 0; i < items.length; i++) {
        var c = items[i];
        var cf = c.conflictFile;
        var of_ = c.originalFile;
        var dir = cf.path.substring(0, cf.path.lastIndexOf('/'));
        var origName = c.originalPath.split('/').pop();
        var sizes = (of_ ? formatBytes(of_.size) : '?') + ' / ' + formatBytes(cf.size);

        html += '<tr>' +
            '<td data-label="">' + FileTypes.icon(origName, false) + '</td>' +
            '<td data-label="File"><span class="fm-conflict-name-compact">' + esc(origName) + '</span></td>' +
            '<td data-label="Folder"><code class="fm-conflict-path-compact">' + esc(dir) + '</code></td>' +
            '<td data-label="Date">' + esc(c.conflictDate) + '</td>' +
            '<td data-label="Sizes">' + sizes + '</td>' +
            '<td data-label="" class="fm-conflict-row-actions">';
        if (of_) {
            html += '<button class="btn btn-sm btn-outline" data-action="compare" data-idx="' + c._flatIdx + '">Compare</button>';
        }
        if (of_) {
            html += '<button class="btn btn-sm btn-outline" data-action="keep-original" data-idx="' + c._flatIdx + '">Keep Orig</button>';
        }
        html += '<button class="btn btn-sm" data-action="keep-conflict" data-idx="' + c._flatIdx + '">Keep Conflict</button>';
        html += '</td></tr>';
    }

    html += '</tbody></table>';
    return html;
}

// ─── Cards Mode ─────────────────────────────────────────────────────────────

function renderConflictCardsMode(items) {
    var html = '<div class="fm-conflict-cards">';
    for (var i = 0; i < items.length; i++) {
        var c = items[i];
        var cf = c.conflictFile;
        var of_ = c.originalFile;

        html += '<div class="fm-conflict-card">' +
            '<div class="fm-conflict-card-header">' +
                '<span class="fm-conflict-icon">&#9888;</span>' +
                '<div class="fm-conflict-info">' +
                    '<div class="fm-conflict-name">' + esc(cf.name) + '</div>' +
                    '<div class="fm-conflict-path"><code>' + esc(cf.path) + '</code></div>' +
                    '<div class="fm-conflict-date">Conflict from ' + esc(c.conflictDate) + '</div>' +
                '</div>' +
            '</div>' +
            '<div class="fm-conflict-comparison">' +
                '<div class="fm-conflict-side">' +
                    '<strong>Original</strong>' +
                    (of_ ? '<div>' + formatBytes(of_.size) + ' &middot; ' + formatDate(of_.mtime) + '</div>'
                         : '<div class="props-muted">Not found (may have been deleted)</div>') +
                '</div>' +
                '<div class="fm-conflict-vs">vs</div>' +
                '<div class="fm-conflict-side">' +
                    '<strong>Conflict Copy</strong>' +
                    '<div>' + formatBytes(cf.size) + ' &middot; ' + formatDate(cf.mtime) + '</div>' +
                '</div>' +
            '</div>' +
            '<div class="fm-conflict-actions">';
        if (of_) {
            html += '<button class="btn btn-sm btn-outline" data-action="compare" data-idx="' + c._flatIdx + '">Compare</button>';
        }
        if (of_) {
            html += '<button class="btn btn-sm btn-outline" data-action="keep-original" data-idx="' + c._flatIdx + '">Keep Original</button>';
        }
        html += '<button class="btn btn-sm" data-action="keep-conflict" data-idx="' + c._flatIdx + '">Keep Conflict</button>';
        html += '</div></div>';
    }
    html += '</div>';
    return html;
}

// ─── Detailed Mode ──────────────────────────────────────────────────────────

function renderConflictDetailedMode(items) {
    var html = '<div class="fm-conflict-cards">';
    for (var i = 0; i < items.length; i++) {
        var c = items[i];
        var cf = c.conflictFile;
        var of_ = c.originalFile;
        var ftype = conflictFileType(cf.name);

        html += '<div class="fm-conflict-card fm-conflict-card-detailed">' +
            '<div class="fm-conflict-card-header">' +
                '<span class="fm-conflict-icon">&#9888;</span>' +
                '<div class="fm-conflict-info">' +
                    '<div class="fm-conflict-name">' + esc(cf.name) + '</div>' +
                    '<div class="fm-conflict-path"><code>' + esc(cf.path) + '</code></div>' +
                    '<div class="fm-conflict-date">Conflict from ' + esc(c.conflictDate) + '</div>' +
                '</div>' +
            '</div>';

        // Thumbnails for images/videos
        if (ftype === 'image' && of_) {
            var origSrc = API.downloadUrl(of_.path.replace(/^\//, ''));
            var confSrc = API.downloadUrl(cf.path.replace(/^\//, ''));
            html += '<div class="fm-conflict-thumbs">' +
                '<div class="fm-conflict-thumb">' +
                    '<div class="fm-conflict-thumb-label">Original</div>' +
                    '<img src="' + esc(origSrc) + '" loading="lazy">' +
                '</div>' +
                '<div class="fm-conflict-thumb">' +
                    '<div class="fm-conflict-thumb-label">Conflict</div>' +
                    '<img src="' + esc(confSrc) + '" loading="lazy">' +
                '</div>' +
            '</div>';
        } else {
            // Extended metadata for non-image files
            html += '<div class="fm-conflict-comparison">' +
                '<div class="fm-conflict-side">' +
                    '<strong>Original</strong>';
            if (of_) {
                html += '<div>' + formatBytes(of_.size) + '</div>' +
                    '<div class="props-muted">' + formatDate(of_.mtime) + '</div>' +
                    (of_.hash ? '<div class="props-muted"><code>' + esc(of_.hash.substring(0, 16)) + '</code></div>' : '');
            } else {
                html += '<div class="props-muted">Not found</div>';
            }
            html += '</div><div class="fm-conflict-vs">vs</div>' +
                '<div class="fm-conflict-side">' +
                    '<strong>Conflict Copy</strong>' +
                    '<div>' + formatBytes(cf.size) + '</div>' +
                    '<div class="props-muted">' + formatDate(cf.mtime) + '</div>' +
                    (cf.hash ? '<div class="props-muted"><code>' + esc(cf.hash.substring(0, 16)) + '</code></div>' : '') +
                '</div></div>';
        }

        html += '<div class="fm-conflict-actions">';
        if (of_) {
            html += '<button class="btn btn-sm btn-outline" data-action="compare" data-idx="' + c._flatIdx + '">Compare</button>';
        }
        if (of_) {
            html += '<button class="btn btn-sm btn-outline" data-action="keep-original" data-idx="' + c._flatIdx + '">Keep Original</button>';
        }
        html += '<button class="btn btn-sm" data-action="keep-conflict" data-idx="' + c._flatIdx + '">Keep Conflict</button>';
        html += '</div></div>';
    }
    html += '</div>';
    return html;
}

// ─── Conflict File Type Detection ───────────────────────────────────────────

function conflictFileType(name) {
    var type = FileTypes.detect(name);
    if (type === 'image') return 'image';
    if (type === 'text') return 'text';
    if (type === 'pdf') return 'pdf';
    var ext = FileTypes.getExt(name);
    if (['.mp4', '.mkv', '.avi', '.mov', '.webm', '.flv', '.wmv'].indexOf(ext) !== -1) return 'video';
    if (['.mp3', '.wav', '.ogg', '.flac', '.aac', '.m4a', '.wma'].indexOf(ext) !== -1) return 'audio';
    return 'other';
}

// ─── Fullscreen Compare Overlay ─────────────────────────────────────────────

function openConflictOverlay(conflicts, idx) {
    _ccConflicts = conflicts;
    _ccIndex = idx;

    // Create overlay
    var overlay = document.createElement('div');
    overlay.className = 'conflict-compare-overlay';
    overlay.innerHTML =
        '<button class="lightbox-close cc-close" title="Close">&times;</button>' +
        '<button class="lightbox-prev cc-prev" title="Previous conflict">&#10094;</button>' +
        '<button class="lightbox-next cc-next" title="Next conflict">&#10095;</button>' +
        '<div class="conflict-compare-body"></div>' +
        '<div class="conflict-compare-toolbar"></div>';

    document.body.appendChild(overlay);
    _ccOverlay = overlay;

    // Wire close
    overlay.querySelector('.cc-close').addEventListener('click', closeConflictOverlay);

    // Wire nav
    overlay.querySelector('.cc-prev').addEventListener('click', function() {
        if (_ccIndex > 0) {
            _ccIndex--;
            renderOverlayContent();
        }
    });
    overlay.querySelector('.cc-next').addEventListener('click', function() {
        if (_ccIndex < _ccConflicts.length - 1) {
            _ccIndex++;
            renderOverlayContent();
        }
    });

    // Keyboard
    _ccKeyHandler = function(e) {
        if (e.key === 'Escape') closeConflictOverlay();
        else if (e.key === 'ArrowLeft' && _ccIndex > 0) { _ccIndex--; renderOverlayContent(); }
        else if (e.key === 'ArrowRight' && _ccIndex < _ccConflicts.length - 1) { _ccIndex++; renderOverlayContent(); }
    };
    document.addEventListener('keydown', _ccKeyHandler);

    renderOverlayContent();
}

function closeConflictOverlay() {
    if (_ccOverlay) {
        _ccOverlay.remove();
        _ccOverlay = null;
    }
    if (_ccKeyHandler) {
        document.removeEventListener('keydown', _ccKeyHandler);
        _ccKeyHandler = null;
    }
}

function renderOverlayContent() {
    if (!_ccOverlay) return;
    var c = _ccConflicts[_ccIndex];
    var body = _ccOverlay.querySelector('.conflict-compare-body');
    var toolbar = _ccOverlay.querySelector('.conflict-compare-toolbar');
    var ftype = conflictFileType(c.conflictFile.name);

    // Update nav arrow visibility
    _ccOverlay.querySelector('.cc-prev').style.display = _ccIndex > 0 ? '' : 'none';
    _ccOverlay.querySelector('.cc-next').style.display = _ccIndex < _ccConflicts.length - 1 ? '' : 'none';

    // Toolbar
    var origName = c.originalPath.split('/').pop();
    var toolbarHtml = '<div class="cc-toolbar-left">';

    if (ftype === 'image') {
        toolbarHtml += '<button class="btn btn-sm btn-outline cc-mode-btn active" data-ccmode="side">Side by Side</button>' +
            '<button class="btn btn-sm btn-outline cc-mode-btn" data-ccmode="slider">Slider</button>';
    }
    if (ftype === 'video') {
        toolbarHtml += '<label class="cc-sync-label"><input type="checkbox" class="cc-sync-check"> Sync Playback</label>';
    }

    toolbarHtml += '</div><div class="cc-toolbar-center">' +
        '<span class="cc-filename">' + esc(origName) + '</span>' +
    '</div><div class="cc-toolbar-right">';

    if (c.originalFile) {
        toolbarHtml += '<button class="btn btn-sm btn-outline conflict-resolve-btn" data-resolve="keep-original">Keep Original</button>';
    }
    toolbarHtml += '<button class="btn btn-sm conflict-resolve-btn" data-resolve="keep-conflict">Keep Conflict</button>' +
        '<span class="cc-counter">' + (_ccIndex + 1) + ' / ' + _ccConflicts.length + '</span>' +
    '</div>';

    toolbar.innerHTML = toolbarHtml;

    // Wire resolve buttons
    toolbar.querySelectorAll('.conflict-resolve-btn').forEach(function(btn) {
        btn.addEventListener('click', function() {
            var action = btn.getAttribute('data-resolve');
            resolveFromOverlay(action);
        });
    });

    // Render body based on type
    if (ftype === 'image') {
        renderOverlayImage(c, body, 'side');
        // Wire mode toggles
        toolbar.querySelectorAll('.cc-mode-btn').forEach(function(btn) {
            btn.addEventListener('click', function() {
                toolbar.querySelectorAll('.cc-mode-btn').forEach(function(b) { b.classList.remove('active'); });
                btn.classList.add('active');
                renderOverlayImage(c, body, btn.getAttribute('data-ccmode'));
            });
        });
    } else if (ftype === 'video') {
        renderOverlayVideo(c, body, toolbar);
    } else if (ftype === 'audio') {
        renderOverlayAudio(c, body);
    } else if (ftype === 'text') {
        renderOverlayText(c, body);
    } else if (ftype === 'pdf') {
        renderOverlayPdf(c, body);
    } else {
        renderOverlayBinary(c, body);
    }
}

// ─── Overlay Image: Side by Side ────────────────────────────────────────────

function renderOverlayImage(c, body, mode) {
    var origSrc = API.downloadUrl(c.originalFile.path.replace(/^\//, ''));
    var confSrc = API.downloadUrl(c.conflictFile.path.replace(/^\//, ''));

    if (mode === 'slider') {
        renderOverlaySlider(body, origSrc, confSrc);
    } else {
        body.innerHTML =
            '<div class="cc-image-side-by-side">' +
                '<div class="cc-image-panel">' +
                    '<div class="cc-image-label">Original</div>' +
                    '<img src="' + esc(origSrc) + '">' +
                '</div>' +
                '<div class="cc-image-panel">' +
                    '<div class="cc-image-label">Conflict Copy</div>' +
                    '<img src="' + esc(confSrc) + '">' +
                '</div>' +
            '</div>';
    }
}

// ─── Overlay Slider (proper drag handle) ────────────────────────────────────

function renderOverlaySlider(body, origSrc, confSrc) {
    body.innerHTML =
        '<div class="cc-slider-container">' +
            '<img class="cc-slider-img cc-slider-img-orig" src="' + esc(origSrc) + '">' +
            '<div class="cc-slider-clip">' +
                '<img class="cc-slider-img cc-slider-img-conf" src="' + esc(confSrc) + '">' +
            '</div>' +
            '<div class="cc-slider-handle">' +
                '<div class="cc-slider-handle-line"></div>' +
                '<div class="cc-slider-handle-grip">&#8596;</div>' +
            '</div>' +
            '<div class="cc-slider-labels">' +
                '<span class="cc-slider-label-left">Original</span>' +
                '<span class="cc-slider-label-right">Conflict Copy</span>' +
            '</div>' +
        '</div>';

    var container = body.querySelector('.cc-slider-container');
    var clip = body.querySelector('.cc-slider-clip');
    var handle = body.querySelector('.cc-slider-handle');
    var confImg = body.querySelector('.cc-slider-img-conf');
    var origImg = body.querySelector('.cc-slider-img-orig');
    var dragging = false;

    function setPosition(pct) {
        pct = Math.max(0, Math.min(100, pct));
        clip.style.width = pct + '%';
        handle.style.left = pct + '%';
    }

    function getPct(e) {
        var rect = container.getBoundingClientRect();
        var x = (e.touches ? e.touches[0].clientX : e.clientX) - rect.left;
        return (x / rect.width) * 100;
    }

    // Initialize at 50%
    setPosition(50);

    // Set conflict image width to match container on load
    function syncImgWidth() {
        confImg.style.width = container.offsetWidth + 'px';
    }
    origImg.addEventListener('load', syncImgWidth);
    if (origImg.complete) syncImgWidth();

    // Mouse drag
    handle.addEventListener('mousedown', function(e) {
        e.preventDefault();
        dragging = true;
    });
    document.addEventListener('mousemove', function(e) {
        if (!dragging) return;
        setPosition(getPct(e));
    });
    document.addEventListener('mouseup', function() {
        dragging = false;
    });

    // Touch drag
    handle.addEventListener('touchstart', function(e) {
        e.preventDefault();
        dragging = true;
    });
    document.addEventListener('touchmove', function(e) {
        if (!dragging) return;
        setPosition(getPct(e));
    });
    document.addEventListener('touchend', function() {
        dragging = false;
    });

    // Click anywhere on container to jump
    container.addEventListener('click', function(e) {
        if (e.target.closest('.cc-slider-handle')) return;
        setPosition(getPct(e));
    });

    // Resize handler
    var resizeHandler = function() {
        syncImgWidth();
    };
    window.addEventListener('resize', resizeHandler);

    // Store cleanup ref on the overlay
    if (_ccOverlay) {
        _ccOverlay._sliderCleanup = function() {
            window.removeEventListener('resize', resizeHandler);
        };
    }
}

// ─── Overlay Video ──────────────────────────────────────────────────────────

function renderOverlayVideo(c, body, toolbar) {
    var origSrc = API.downloadUrl(c.originalFile.path.replace(/^\//, ''));
    var confSrc = API.downloadUrl(c.conflictFile.path.replace(/^\//, ''));

    body.innerHTML =
        '<div class="cc-video-side-by-side">' +
            '<div class="cc-video-panel">' +
                '<div class="cc-image-label">Original</div>' +
                '<video controls preload="metadata" src="' + esc(origSrc) + '"></video>' +
            '</div>' +
            '<div class="cc-video-panel">' +
                '<div class="cc-image-label">Conflict Copy</div>' +
                '<video controls preload="metadata" src="' + esc(confSrc) + '"></video>' +
            '</div>' +
        '</div>';

    var videos = body.querySelectorAll('video');
    var v1 = videos[0];
    var v2 = videos[1];
    var syncCheck = toolbar.querySelector('.cc-sync-check');
    var syncing = false;

    function syncHandler(src, dst) {
        return function() {
            if (!syncing || !syncCheck.checked) return;
            syncing = false;
            if (Math.abs(dst.currentTime - src.currentTime) > 0.3) {
                dst.currentTime = src.currentTime;
            }
            if (src.paused && !dst.paused) dst.pause();
            if (!src.paused && dst.paused) dst.play();
            syncing = true;
        };
    }

    if (syncCheck) {
        syncCheck.addEventListener('change', function() {
            syncing = syncCheck.checked;
        });
    }

    ['play', 'pause', 'seeked'].forEach(function(evt) {
        v1.addEventListener(evt, function() {
            if (!syncing) return;
            syncing = false;
            if (evt === 'seeked' && Math.abs(v2.currentTime - v1.currentTime) > 0.3) v2.currentTime = v1.currentTime;
            if (evt === 'play' && v2.paused) v2.play();
            if (evt === 'pause' && !v2.paused) v2.pause();
            syncing = true;
        });
        v2.addEventListener(evt, function() {
            if (!syncing) return;
            syncing = false;
            if (evt === 'seeked' && Math.abs(v1.currentTime - v2.currentTime) > 0.3) v1.currentTime = v2.currentTime;
            if (evt === 'play' && v1.paused) v1.play();
            if (evt === 'pause' && !v1.paused) v1.pause();
            syncing = true;
        });
    });
}

// ─── Overlay Audio ──────────────────────────────────────────────────────────

function renderOverlayAudio(c, body) {
    var origSrc = API.downloadUrl(c.originalFile.path.replace(/^\//, ''));
    var confSrc = API.downloadUrl(c.conflictFile.path.replace(/^\//, ''));

    body.innerHTML =
        '<div class="cc-audio-side-by-side">' +
            '<div class="cc-audio-panel">' +
                '<div class="cc-image-label">Original</div>' +
                '<audio controls preload="metadata" src="' + esc(origSrc) + '"></audio>' +
            '</div>' +
            '<div class="cc-audio-panel">' +
                '<div class="cc-image-label">Conflict Copy</div>' +
                '<audio controls preload="metadata" src="' + esc(confSrc) + '"></audio>' +
            '</div>' +
        '</div>';
}

// ─── Overlay Text Diff ──────────────────────────────────────────────────────

function renderOverlayText(c, body) {
    body.innerHTML = '<div class="cc-text-loading"><p class="props-muted">Loading both files...</p></div>';

    var originalUrl = '/api/v1/content/' + API.encodeURIPath(c.originalFile.path.replace(/^\//, ''));
    var conflictUrl = '/api/v1/content/' + API.encodeURIPath(c.conflictFile.path.replace(/^\//, ''));

    Promise.all([
        API.request('GET', originalUrl).then(function(r) { return r.text(); }),
        API.request('GET', conflictUrl).then(function(r) { return r.text(); })
    ]).then(function(results) {
        body.innerHTML = '<div class="cc-text-diff">' + computeDiff(results[0], results[1], 'Original') + '</div>';
    }).catch(function() {
        body.innerHTML = '<div class="cc-text-diff"><div class="alert alert-error">Failed to load file content</div></div>';
    });
}

// ─── Overlay PDF ────────────────────────────────────────────────────────────

function renderOverlayPdf(c, body) {
    var origSrc = API.downloadUrl(c.originalFile.path.replace(/^\//, ''));
    var confSrc = API.downloadUrl(c.conflictFile.path.replace(/^\//, ''));

    body.innerHTML =
        '<div class="cc-pdf-side-by-side">' +
            '<div class="cc-pdf-panel">' +
                '<div class="cc-image-label">Original</div>' +
                '<embed src="' + esc(origSrc) + '" type="application/pdf">' +
            '</div>' +
            '<div class="cc-pdf-panel">' +
                '<div class="cc-image-label">Conflict Copy</div>' +
                '<embed src="' + esc(confSrc) + '" type="application/pdf">' +
            '</div>' +
        '</div>';
}

// ─── Overlay Binary/Other ───────────────────────────────────────────────────

function renderOverlayBinary(c, body) {
    var of_ = c.originalFile;
    var cf = c.conflictFile;

    body.innerHTML =
        '<div class="cc-binary-side-by-side">' +
            '<div class="cc-binary-panel">' +
                '<div class="cc-image-label">Original</div>' +
                (of_ ? '<div class="cc-binary-meta">' +
                    '<div>' + formatBytes(of_.size) + '</div>' +
                    '<div class="props-muted">' + formatDate(of_.mtime) + '</div>' +
                    '<a class="btn btn-sm btn-outline" href="' + esc(API.downloadUrl(of_.path.replace(/^\//, ''))) + '" download>Download</a>' +
                '</div><div class="cc-hex-dump" id="cc-hex-orig">Loading hex...</div>'
                : '<div class="props-muted">Not found</div>') +
            '</div>' +
            '<div class="cc-binary-panel">' +
                '<div class="cc-image-label">Conflict Copy</div>' +
                '<div class="cc-binary-meta">' +
                    '<div>' + formatBytes(cf.size) + '</div>' +
                    '<div class="props-muted">' + formatDate(cf.mtime) + '</div>' +
                    '<a class="btn btn-sm btn-outline" href="' + esc(API.downloadUrl(cf.path.replace(/^\//, ''))) + '" download>Download</a>' +
                '</div><div class="cc-hex-dump" id="cc-hex-conf">Loading hex...</div>' +
            '</div>' +
        '</div>';

    // Fetch first 512 bytes of each file and render hex dumps
    if (of_) {
        fetchHexDump(of_.path, 'cc-hex-orig');
    }
    fetchHexDump(cf.path, 'cc-hex-conf');
}

function fetchHexDump(filePath, elementId) {
    var url = '/api/v1/content/' + API.encodeURIPath(filePath.replace(/^\//, ''));
    var headers = {};
    var token = API.getToken();
    if (token) headers['Authorization'] = 'Bearer ' + token;
    headers['Range'] = 'bytes=0-511';
    fetch(url, { method: 'GET', headers: headers })
        .then(function(resp) { return resp.arrayBuffer(); })
        .then(function(buf) {
            var el = document.getElementById(elementId);
            if (!el) return;
            var bytes = new Uint8Array(buf);
            if (bytes.length > 512) bytes = bytes.slice(0, 512);
            el.innerHTML = '<pre>' + formatHex(bytes) + '</pre>';
        }).catch(function() {
            var el = document.getElementById(elementId);
            if (el) el.innerHTML = '<span class="props-muted">Could not load preview</span>';
        });
}

function formatHex(bytes) {
    var lines = [];
    for (var i = 0; i < bytes.length; i += 16) {
        var offset = i.toString(16).padStart(8, '0');
        var hexParts = [];
        var ascii = '';
        for (var j = 0; j < 16; j++) {
            if (i + j < bytes.length) {
                hexParts.push(bytes[i + j].toString(16).padStart(2, '0'));
                var ch = bytes[i + j];
                ascii += (ch >= 32 && ch <= 126) ? String.fromCharCode(ch) : '.';
            } else {
                hexParts.push('  ');
                ascii += ' ';
            }
        }
        var hex = hexParts.slice(0, 8).join(' ') + '  ' + hexParts.slice(8).join(' ');
        lines.push(offset + '  ' + hex + '  |' + esc(ascii) + '|');
    }
    return lines.join('\n');
}

// ─── Resolve from Overlay ───────────────────────────────────────────────────

function resolveFromOverlay(action) {
    var c = _ccConflicts[_ccIndex];
    var fn = action === 'keep-original' ? resolveKeepOriginal : resolveKeepConflict;
    var label = action === 'keep-original' ? 'kept original' : 'kept conflict copy';

    fn(c).then(function() {
        Toast.success('Conflict resolved: ' + label);
        // Remove from list
        _ccConflicts.splice(_ccIndex, 1);
        if (_ccConflicts.length === 0) {
            closeConflictOverlay();
            renderConflictsTab();
            return;
        }
        if (_ccIndex >= _ccConflicts.length) _ccIndex = _ccConflicts.length - 1;
        renderOverlayContent();
    }).catch(function(err) {
        Toast.error('Failed to resolve: ' + err.message);
    });
}

// ─── Diff Utilities ─────────────────────────────────────────────────────────

// Simple line-based diff
function computeDiff(oldText, newText, oldVersion) {
    var oldLines = oldText.split('\n');
    var newLines = newText.split('\n');

    // LCS-based diff
    var lcs = lcsMatrix(oldLines, newLines);
    var ops = backtrackDiff(lcs, oldLines, newLines);

    if (ops.length === 0) {
        return '<div class="ver-diff-same">Files are identical</div>';
    }

    var addCount = 0, removeCount = 0;
    for (var k = 0; k < ops.length; k++) {
        if (ops[k].type === 'add') addCount++;
        if (ops[k].type === 'remove') removeCount++;
    }

    var html = '<div class="ver-diff-stats">' +
        '<span class="ver-diff-stat-add">+' + addCount + ' added</span>' +
        '<span class="ver-diff-stat-remove">-' + removeCount + ' removed</span>' +
        '</div>';

    html += '<div class="ver-diff-content"><table class="ver-diff-table">';
    html += '<thead><tr><th class="ver-diff-ln">v' + oldVersion + '</th><th class="ver-diff-ln">Current</th><th></th></tr></thead>';
    html += '<tbody>';

    var oldLn = 0, newLn = 0;

    for (var i = 0; i < ops.length; i++) {
        var op = ops[i];
        if (op.type === 'same') {
            oldLn++; newLn++;
            html += '<tr class="ver-diff-row-same">' +
                '<td class="ver-diff-ln">' + oldLn + '</td>' +
                '<td class="ver-diff-ln">' + newLn + '</td>' +
                '<td class="ver-diff-code">' + esc(op.line) + '</td></tr>';
        } else if (op.type === 'remove') {
            oldLn++;
            html += '<tr class="ver-diff-row-remove">' +
                '<td class="ver-diff-ln">' + oldLn + '</td>' +
                '<td class="ver-diff-ln"></td>' +
                '<td class="ver-diff-code">- ' + esc(op.line) + '</td></tr>';
        } else if (op.type === 'add') {
            newLn++;
            html += '<tr class="ver-diff-row-add">' +
                '<td class="ver-diff-ln"></td>' +
                '<td class="ver-diff-ln">' + newLn + '</td>' +
                '<td class="ver-diff-code">+ ' + esc(op.line) + '</td></tr>';
        }
    }

    html += '</tbody></table></div>';
    return html;
}

// Build LCS length matrix
function lcsMatrix(a, b) {
    var m = a.length, n = b.length;
    // For very large files, skip full LCS and use simple diff
    if (m > 2000 || n > 2000) return null;

    var dp = [];
    for (var i = 0; i <= m; i++) {
        dp[i] = [];
        for (var j = 0; j <= n; j++) {
            if (i === 0 || j === 0) {
                dp[i][j] = 0;
            } else if (a[i-1] === b[j-1]) {
                dp[i][j] = dp[i-1][j-1] + 1;
            } else {
                dp[i][j] = Math.max(dp[i-1][j], dp[i][j-1]);
            }
        }
    }
    return dp;
}

// Backtrack through LCS matrix to produce diff ops
function backtrackDiff(dp, a, b) {
    if (!dp) {
        // Fallback for large files: simple line-by-line comparison
        return simpleDiff(a, b);
    }

    var ops = [];
    var i = a.length, j = b.length;

    while (i > 0 || j > 0) {
        if (i > 0 && j > 0 && a[i-1] === b[j-1]) {
            ops.unshift({ type: 'same', line: a[i-1] });
            i--; j--;
        } else if (j > 0 && (i === 0 || dp[i][j-1] >= dp[i-1][j])) {
            ops.unshift({ type: 'add', line: b[j-1] });
            j--;
        } else {
            ops.unshift({ type: 'remove', line: a[i-1] });
            i--;
        }
    }
    return ops;
}

// Fallback diff for large files
function simpleDiff(a, b) {
    var ops = [];
    var maxLen = Math.max(a.length, b.length);
    for (var i = 0; i < maxLen; i++) {
        if (i < a.length && i < b.length) {
            if (a[i] === b[i]) {
                ops.push({ type: 'same', line: a[i] });
            } else {
                ops.push({ type: 'remove', line: a[i] });
                ops.push({ type: 'add', line: b[i] });
            }
        } else if (i < a.length) {
            ops.push({ type: 'remove', line: a[i] });
        } else {
            ops.push({ type: 'add', line: b[i] });
        }
    }
    return ops;
}
