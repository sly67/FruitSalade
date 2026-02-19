function renderShareDownload() {
    var hash = window.location.hash.replace('#share', '').replace(/^\//, '');
    var parts = hash.split('/');
    var token = parts[0] || '';
    var password = parts[1] || '';

    var app = document.getElementById('app');

    if (!token) {
        app.innerHTML =
            '<div class="share-page"><div class="share-card">' +
                '<div class="share-error">Invalid share link</div>' +
            '</div></div>';
        return;
    }

    app.innerHTML =
        '<div class="share-page"><div class="share-card">' +
            '<div class="share-loading">Loading...</div>' +
        '</div></div>';

    fetch('/api/v1/share/' + encodeURIComponent(token) + '/info')
        .then(function(resp) { return resp.json(); })
        .then(function(info) {
            var card = app.querySelector('.share-card');

            if (!info.valid) {
                card.innerHTML =
                    '<div class="share-brand">FruitSalade</div>' +
                    '<div class="share-error">' +
                        '<div class="share-error-icon">&#9888;</div>' +
                        '<div class="share-error-msg">' + esc(info.error || 'This share link is no longer available') + '</div>' +
                    '</div>';
                return;
            }

            if (info.has_password && !password) {
                // Show password form
                card.innerHTML =
                    '<div class="share-brand">FruitSalade</div>' +
                    '<div class="share-file-info">' +
                        '<div class="share-file-icon">' + FileTypes.icon(info.file_name, false) + '</div>' +
                        '<div class="share-file-name">' + esc(info.file_name) + '</div>' +
                        (info.file_size ? '<div class="share-file-size">' + formatBytes(info.file_size) + '</div>' : '') +
                    '</div>' +
                    '<div class="share-password-section">' +
                        '<p class="share-password-label">This file is password-protected</p>' +
                        '<form id="share-pw-form" class="share-password-form">' +
                            '<input type="password" id="share-pw-input" placeholder="Enter password" required>' +
                            '<button type="submit" class="btn">Unlock</button>' +
                        '</form>' +
                        '<div id="share-pw-error" class="share-pw-error hidden"></div>' +
                    '</div>';

                document.getElementById('share-pw-form').addEventListener('submit', function(e) {
                    e.preventDefault();
                    var pw = document.getElementById('share-pw-input').value;
                    if (!pw) return;
                    startDownload(token, pw);
                });
                return;
            }

            // Ready to download (no password needed, or password in URL)
            card.innerHTML =
                '<div class="share-brand">FruitSalade</div>' +
                '<div class="share-file-info">' +
                    '<div class="share-file-icon">' + FileTypes.icon(info.file_name, false) + '</div>' +
                    '<div class="share-file-name">' + esc(info.file_name) + '</div>' +
                    (info.file_size ? '<div class="share-file-size">' + formatBytes(info.file_size) + '</div>' : '') +
                '</div>' +
                (info.expires_at ? '<div class="share-meta">Expires: ' + formatDate(info.expires_at) + '</div>' : '') +
                '<button class="btn share-download-btn" id="share-dl-btn">Download</button>';

            document.getElementById('share-dl-btn').addEventListener('click', function() {
                startDownload(token, password);
            });

            // Auto-download if password was in URL
            if (password) {
                startDownload(token, password);
            }
        })
        .catch(function() {
            var card = app.querySelector('.share-card');
            card.innerHTML =
                '<div class="share-brand">FruitSalade</div>' +
                '<div class="share-error">' +
                    '<div class="share-error-icon">&#9888;</div>' +
                    '<div class="share-error-msg">Failed to load share link information</div>' +
                '</div>';
        });

    function startDownload(token, pw) {
        var url = '/api/v1/share/' + encodeURIComponent(token);
        if (pw) {
            url += '?password=' + encodeURIComponent(pw);
        }

        // Validate via fetch first, then trigger browser download
        fetch(url).then(function(resp) {
            if (resp.ok) {
                // Valid — use a hidden link for proper download behavior
                resp.blob().then(function(blob) {
                    var a = document.createElement('a');
                    var cd = resp.headers.get('Content-Disposition') || '';
                    var match = cd.match(/filename="?([^"]+)"?/);
                    a.download = match ? match[1] : 'download';
                    a.href = URL.createObjectURL(blob);
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);
                    URL.revokeObjectURL(a.href);
                });
            } else {
                resp.json().then(function(data) {
                    showDownloadError(data.error || 'Download failed');
                }).catch(function() {
                    showDownloadError('Download failed');
                });
            }
        }).catch(function() {
            showDownloadError('Download failed — network error');
        });
    }

    function showDownloadError(msg) {
        var errEl = document.getElementById('share-pw-error');
        if (errEl) {
            errEl.textContent = msg;
            errEl.classList.remove('hidden');
        } else {
            // Show error on the card
            var card = app.querySelector('.share-card');
            if (card) {
                var existing = card.querySelector('.share-dl-error');
                if (existing) existing.remove();
                var div = document.createElement('div');
                div.className = 'share-dl-error';
                div.textContent = msg;
                card.appendChild(div);
            }
        }
    }
}
