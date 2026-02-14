function renderShares() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar"><h2>Share Links</h2></div>' +
        '<div id="shares-alert"></div>' +
        '<div id="shares-table">Loading...</div>';

    loadShares();
}

function loadShares() {
    API.get('/api/v1/admin/sharelinks').then(function(links) {
        if (!links || links.length === 0) {
            document.getElementById('shares-table').innerHTML = '<p>No share links found.</p>';
            return;
        }

        var rows = '';
        for (var i = 0; i < links.length; i++) {
            var l = links[i];
            var statusBadge = l.is_active
                ? '<span class="badge badge-green">Active</span>'
                : '<span class="badge badge-red">Revoked</span>';

            var expiresAt = l.expires_at ? formatDate(l.expires_at) : 'Never';
            var downloads = l.download_count + (l.max_downloads > 0 ? '/' + l.max_downloads : '');

            rows += '<tr>' +
                '<td title="' + esc(l.id) + '">' + esc(l.id.substring(0, 12)) + '...</td>' +
                '<td>' + esc(l.path) + '</td>' +
                '<td>' + esc(l.created_by_username) + '</td>' +
                '<td>' + statusBadge + '</td>' +
                '<td>' + downloads + '</td>' +
                '<td>' + expiresAt + '</td>' +
                '<td>' + formatDate(l.created_at) + '</td>' +
                '<td>' +
                    (l.is_active
                        ? '<button class="btn btn-sm btn-danger" onclick="revokeShareLink(\'' + esc(l.id) + '\')">Revoke</button>'
                        : '') +
                '</td>' +
            '</tr>';
        }

        document.getElementById('shares-table').innerHTML =
            '<div class="table-wrap"><table>' +
                '<thead><tr><th>ID</th><th>Path</th><th>Created By</th><th>Status</th><th>Downloads</th><th>Expires</th><th>Created</th><th>Actions</th></tr></thead>' +
                '<tbody>' + rows + '</tbody>' +
            '</table></div>';
    }).catch(function() {
        document.getElementById('shares-table').innerHTML =
            '<div class="alert alert-error">Failed to load share links</div>';
    });
}

function revokeShareLink(id) {
    if (!confirm('Revoke this share link?')) return;

    API.del('/api/v1/share/' + id).then(function(resp) {
        return resp.json().then(function(data) {
            if (resp.ok) {
                showAlert('shares-alert', 'Share link revoked', 'success');
                loadShares();
            } else {
                showAlert('shares-alert', data.error || 'Failed to revoke', 'error');
            }
        });
    }).catch(function() {
        showAlert('shares-alert', 'Failed to revoke share link', 'error');
    });
}
