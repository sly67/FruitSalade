// Version explorer — browse all versioned files or view a single file's history
function renderVersions() {
    var hash = window.location.hash.replace('#versions', '').replace(/^\//, '');

    if (!hash) {
        renderVersionExplorer();
    } else {
        var filePath = '/' + decodeURIComponent(hash);
        renderFileVersions(filePath);
    }
}

// ─── Version Explorer (all versioned files) ─────────────────────────────────

function renderVersionExplorer() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Version Explorer</h2>' +
            '<div class="toolbar-actions">' +
                '<div class="search-wrap">' +
                    '<input type="text" id="ver-search" placeholder="Filter files...">' +
                '</div>' +
            '</div>' +
        '</div>' +
        '<div id="ver-stats" class="ver-stats"></div>' +
        '<div id="ver-table" class="table-wrap">Loading...</div>';

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

