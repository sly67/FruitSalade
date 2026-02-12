// Admin share link management view (ported from admin UI)
function renderAdminShares() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar"><h2>Share Links (Admin)</h2></div>' +
        '<div id="admin-shares-table">Loading...</div>';

    loadAdminShares();
}

function loadAdminShares() {
    API.get('/api/v1/admin/sharelinks').then(function(links) {
        if (!links || links.length === 0) {
            document.getElementById('admin-shares-table').innerHTML =
                '<p style="color:var(--text-muted)">No share links found.</p>';
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
                '<td data-label="ID" title="' + esc(l.id) + '">' + esc(l.id.substring(0, 12)) + '...</td>' +
                '<td data-label="Path">' + esc(l.path) + '</td>' +
                '<td data-label="Created By">' + esc(l.created_by_username || '-') + '</td>' +
                '<td data-label="Status">' + statusBadge + '</td>' +
                '<td data-label="Downloads">' + downloads + '</td>' +
                '<td data-label="Expires">' + expiresAt + '</td>' +
                '<td data-label="Created">' + formatDate(l.created_at) + '</td>' +
                '<td data-label="">' +
                    (l.is_active
                        ? '<button class="btn btn-sm btn-danger" data-action="revoke-share" data-id="' + esc(l.id) + '">Revoke</button>'
                        : '') +
                '</td>' +
            '</tr>';
        }

        document.getElementById('admin-shares-table').innerHTML =
            '<div class="table-wrap"><table class="responsive-table">' +
                '<thead><tr><th>ID</th><th>Path</th><th>Created By</th><th>Status</th><th>Downloads</th><th>Expires</th><th>Created</th><th>Actions</th></tr></thead>' +
                '<tbody>' + rows + '</tbody>' +
            '</table></div>';

        // Wire revoke buttons
        document.getElementById('admin-shares-table').querySelectorAll('[data-action="revoke-share"]').forEach(function(btn) {
            btn.addEventListener('click', function(e) {
                var id = e.currentTarget.getAttribute('data-id');
                revokeAdminShareLink(id);
            });
        });
    }).catch(function() {
        document.getElementById('admin-shares-table').innerHTML =
            '<div class="alert alert-error">Failed to load share links</div>';
    });
}

function revokeAdminShareLink(id) {
    if (!confirm('Revoke this share link?')) return;

    API.del('/api/v1/share/' + id).then(function(resp) {
        return resp.json().then(function(data) {
            if (resp.ok) {
                Toast.show('Share link revoked', 'success');
                loadAdminShares();
            } else {
                Toast.show(data.error || 'Failed to revoke', 'error');
            }
        });
    }).catch(function() {
        Toast.show('Failed to revoke share link', 'error');
    });
}

