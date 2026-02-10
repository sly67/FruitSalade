function renderVersions() {
    var hash = window.location.hash.replace('#versions', '').replace(/^\//, '');
    if (!hash) {
        document.getElementById('app').innerHTML =
            '<div class="alert alert-error">No file path specified. Navigate to a file and click History.</div>';
        return;
    }

    var filePath = '/' + decodeURIComponent(hash);
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Version History</h2>' +
            '<a href="#browser' + esc(filePath.substring(0, filePath.lastIndexOf('/'))) + '" class="btn btn-sm btn-outline">Back to folder</a>' +
        '</div>' +
        '<p style="margin-bottom:1rem;color:var(--text-muted)">' + esc(filePath) + '</p>' +
        '<div id="version-status"></div>' +
        '<div id="version-table" class="table-wrap"></div>';

    function loadVersions() {
        API.get('/api/v1/versions/' + API.encodeURIPath(filePath.replace(/^\//, '')))
            .then(function(data) {
                if (data.error) {
                    document.getElementById('version-table').innerHTML =
                        '<div class="alert alert-error">' + esc(data.error) + '</div>';
                    return;
                }
                renderVersionTable(data);
            }).catch(function() {
                document.getElementById('version-table').innerHTML =
                    '<div class="alert alert-error">Failed to load versions</div>';
            });
    }

    function renderVersionTable(data) {
        var versions = data.versions || [];
        var current = data.current_version;
        var table = document.getElementById('version-table');

        if (versions.length === 0) {
            table.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">No previous versions (current: v' + current + ')</p>';
            return;
        }

        var html = '<table><thead><tr>' +
            '<th>Version</th><th>Size</th><th>Hash</th><th>Date</th><th>Actions</th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < versions.length; i++) {
            var v = versions[i];
            html += '<tr>' +
                '<td>v' + v.version + (v.version === current ? ' (current)' : '') + '</td>' +
                '<td>' + formatBytes(v.size) + '</td>' +
                '<td style="font-family:monospace;font-size:0.8rem">' + esc((v.hash || '').substring(0, 12)) + '</td>' +
                '<td>' + formatDate(v.created_at) + '</td>' +
                '<td>' +
                    '<div class="btn-group">' +
                        '<a class="btn btn-sm btn-outline" href="/api/v1/versions/' +
                            esc(API.encodeURIPath(filePath.replace(/^\//, ''))) +
                            '?v=' + v.version + '&token=' + encodeURIComponent(API.getToken()) +
                            '" download>Download</a>' +
                        (v.version !== current ?
                            '<button class="btn btn-sm" data-action="rollback" data-version="' + v.version + '">Rollback</button>' : '') +
                    '</div>' +
                '</td>' +
                '</tr>';
        }

        html += '</tbody></table>';
        table.innerHTML = html;

        // Wire rollback buttons
        table.querySelectorAll('[data-action="rollback"]').forEach(function(btn) {
            btn.addEventListener('click', function(e) {
                var version = parseInt(e.currentTarget.getAttribute('data-version'), 10);
                if (!confirm('Rollback to version ' + version + '?')) return;

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
                        loadVersions();
                    });
            });
        });
    }

    loadVersions();
}
