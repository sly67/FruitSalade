function renderTrash() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Trash</h2>' +
            '<div class="toolbar-actions">' +
                '<button class="btn btn-sm btn-danger" id="btn-empty-trash">Empty Trash</button>' +
            '</div>' +
        '</div>' +
        '<div id="trash-table" class="table-wrap">' +
            '<div style="padding:0.75rem">' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
            '</div>' +
        '</div>';

    document.getElementById('btn-empty-trash').addEventListener('click', function() {
        if (!confirm('Permanently delete all items in trash? This cannot be undone.')) return;
        API.del('/api/v1/trash').then(function(resp) {
            if (resp.ok) {
                Toast.success('Trash emptied');
                loadTrash();
            } else {
                resp.json().then(function(d) { Toast.error(d.error || 'Failed to empty trash'); });
            }
        });
    });

    function loadTrash() {
        API.get('/api/v1/trash').then(function(items) {
            renderTrashTable(items);
        }).catch(function() {
            document.getElementById('trash-table').innerHTML =
                '<div class="alert alert-error">Failed to load trash</div>';
        });
    }

    function renderTrashTable(items) {
        var table = document.getElementById('trash-table');

        if (!items || items.length === 0) {
            table.innerHTML =
                '<div class="empty-state">' +
                    '<span class="empty-icon">&#128465;</span>' +
                    '<p>Trash is empty</p>' +
                '</div>';
            return;
        }

        var html = '<table class="responsive-table"><thead><tr>' +
            '<th>Name</th>' +
            '<th>Original Path</th>' +
            '<th>Size</th>' +
            '<th>Deleted</th>' +
            '<th>Deleted By</th>' +
            '<th>Actions</th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < items.length; i++) {
            var item = items[i];
            var iconHtml = FileTypes.icon(item.name, item.is_dir);

            html += '<tr class="file-row">' +
                '<td data-label="Name">' + iconHtml + esc(item.name) + '</td>' +
                '<td data-label="Path" class="trash-path"><code>' + esc(item.original_path) + '</code></td>' +
                '<td data-label="Size">' + (item.is_dir ? '-' : formatBytes(item.size)) + '</td>' +
                '<td data-label="Deleted">' + formatDate(item.deleted_at) + '</td>' +
                '<td data-label="Deleted By">' + esc(item.deleted_by_name || '-') + '</td>' +
                '<td data-label="Actions" class="trash-actions">' +
                    '<button class="btn btn-sm" data-restore="' + esc(item.original_path) + '">Restore</button>' +
                    '<button class="btn btn-sm btn-danger" data-purge="' + esc(item.original_path) + '">Delete</button>' +
                '</td>' +
                '</tr>';
        }

        html += '</tbody></table>';
        table.innerHTML = html;

        // Wire restore buttons
        table.querySelectorAll('[data-restore]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var path = btn.getAttribute('data-restore');
                API.post('/api/v1/trash/restore', { path: path }).then(function(resp) {
                    if (resp.ok) {
                        Toast.success('Restored ' + path.split('/').pop());
                        loadTrash();
                    } else {
                        resp.json().then(function(d) { Toast.error(d.error || 'Restore failed'); });
                    }
                });
            });
        });

        // Wire purge buttons
        table.querySelectorAll('[data-purge]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var path = btn.getAttribute('data-purge');
                if (!confirm('Permanently delete ' + path.split('/').pop() + '? This cannot be undone.')) return;
                API.del('/api/v1/trash/' + API.encodeURIPath(path.replace(/^\//, ''))).then(function(resp) {
                    if (resp.ok) {
                        Toast.success('Permanently deleted');
                        loadTrash();
                    } else {
                        resp.json().then(function(d) { Toast.error(d.error || 'Purge failed'); });
                    }
                });
            });
        });
    }

    loadTrash();
}
