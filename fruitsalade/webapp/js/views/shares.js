function renderShares() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>My Share Links</h2>' +
        '</div>' +
        '<div id="shares-content">' +
            '<div style="padding:0.75rem">' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
            '</div>' +
        '</div>';

    loadShares();
}

function loadShares() {
    var container = document.getElementById('shares-content');

    API.get('/api/v1/shares').then(function(data) {
        if (!data || data.length === 0) {
            container.innerHTML =
                '<div class="dashboard-section">' +
                    '<p class="dashboard-empty">You haven\'t created any share links yet.</p>' +
                    '<a href="#browser" class="btn btn-outline" style="margin-top:0.75rem">Go to Files</a>' +
                '</div>';
            return;
        }

        var html = '<div class="table-wrap"><table class="responsive-table"><thead><tr>' +
            '<th>File</th>' +
            '<th>Downloads</th>' +
            '<th>Expires</th>' +
            '<th>Created</th>' +
            '<th></th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < data.length; i++) {
            var link = data[i];
            var fileName = link.path.split('/').pop();
            var iconHtml = FileTypes.icon(fileName, false);
            var dlInfo = link.download_count + (link.max_downloads > 0 ? '/' + link.max_downloads : '');
            var expInfo = link.expires_at ? formatDate(link.expires_at) : 'Never';
            var active = link.is_active !== false;

            html += '<tr class="file-row">' +
                '<td data-label="File">' +
                    '<a class="file-name" href="#viewer' + esc(link.path) + '">' + iconHtml + esc(fileName) + '</a>' +
                    '<div class="search-path">' + esc(link.path) + '</div>' +
                '</td>' +
                '<td data-label="Downloads">' + dlInfo + '</td>' +
                '<td data-label="Expires">' + expInfo + '</td>' +
                '<td data-label="Created">' + formatDate(link.created_at) + '</td>' +
                '<td data-label="">' +
                    '<div class="btn-group">' +
                        '<button class="btn btn-sm btn-outline" data-copy-link="' + esc(link.id) + '">Copy Link</button>' +
                        (active ? '<button class="btn btn-sm btn-danger" data-revoke-link="' + esc(link.id) + '">Revoke</button>' : '<span class="badge badge-red">Revoked</span>') +
                    '</div>' +
                '</td>' +
                '</tr>';
        }

        html += '</tbody></table></div>';
        container.innerHTML = html;

        // Wire copy link buttons
        container.querySelectorAll('[data-copy-link]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var linkId = btn.getAttribute('data-copy-link');
                var url = window.location.origin + '/app/#share/' + linkId;
                if (navigator.clipboard) {
                    navigator.clipboard.writeText(url).then(function() {
                        Toast.success('Link copied to clipboard');
                    });
                } else {
                    // Fallback
                    var input = document.createElement('input');
                    input.value = url;
                    document.body.appendChild(input);
                    input.select();
                    document.execCommand('copy');
                    document.body.removeChild(input);
                    Toast.success('Link copied to clipboard');
                }
            });
        });

        // Wire revoke buttons
        container.querySelectorAll('[data-revoke-link]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var linkId = btn.getAttribute('data-revoke-link');
                if (!confirm('Revoke this share link?')) return;
                API.del('/api/v1/share/' + encodeURIComponent(linkId)).then(function(resp) {
                    if (resp.ok) {
                        Toast.success('Share link revoked');
                        loadShares();
                    } else {
                        resp.json().then(function(d) { Toast.error(d.error || 'Failed to revoke'); });
                    }
                });
            });
        });
    }).catch(function() {
        container.innerHTML = '<div class="alert alert-error">Failed to load share links</div>';
    });
}
