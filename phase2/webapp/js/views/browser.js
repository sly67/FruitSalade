function renderBrowser() {
    var hash = window.location.hash.replace('#browser', '').replace(/^\//, '');
    var currentPath = hash ? '/' + decodeURIComponent(hash) : '/';

    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Files</h2>' +
            '<div class="toolbar-actions">' +
                '<input type="text" id="search-input" placeholder="Filter by name...">' +
                '<button class="btn btn-sm" id="btn-new-folder">New Folder</button>' +
                '<button class="btn btn-sm" id="btn-upload">Upload</button>' +
                '<input type="file" id="file-input" multiple style="display:none">' +
            '</div>' +
        '</div>' +
        '<div id="breadcrumb" class="breadcrumb"></div>' +
        '<div id="drop-zone" class="drop-zone hidden">Drop files here to upload</div>' +
        '<div id="browser-status"></div>' +
        '<div id="file-table" class="table-wrap"></div>' +
        '<div id="share-modal"></div>';

    var searchInput = document.getElementById('search-input');
    var fileInput = document.getElementById('file-input');
    var dropZone = document.getElementById('drop-zone');
    var allItems = [];

    // Build breadcrumb
    function buildBreadcrumb(path) {
        var bc = document.getElementById('breadcrumb');
        var parts = path.split('/').filter(Boolean);
        var html = '<a href="#browser">/</a>';
        var accumulated = '';
        for (var i = 0; i < parts.length; i++) {
            accumulated += '/' + parts[i];
            html += '<span class="sep">/</span>';
            html += '<a href="#browser' + esc(accumulated) + '">' + esc(parts[i]) + '</a>';
        }
        bc.innerHTML = html;
    }

    // Render file table
    function renderTable(items) {
        var table = document.getElementById('file-table');
        if (!items || items.length === 0) {
            table.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">Empty folder</p>';
            return;
        }

        // Sort: dirs first, then alphabetical
        items.sort(function(a, b) {
            if (a.is_dir && !b.is_dir) return -1;
            if (!a.is_dir && b.is_dir) return 1;
            return a.name.localeCompare(b.name);
        });

        var html = '<table><thead><tr>' +
            '<th>Name</th><th>Size</th><th>Version</th><th>Modified</th><th>Actions</th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < items.length; i++) {
            var f = items[i];
            var icon = f.is_dir ? '&#128193;' : '&#128196;';
            var nameLink;
            if (f.is_dir) {
                nameLink = '<a class="file-name" href="#browser' + esc(f.path) + '">' +
                    '<span class="file-icon">' + icon + '</span>' + esc(f.name) + '</a>';
            } else {
                nameLink = '<a class="file-name" href="#viewer' + esc(f.path) + '">' +
                    '<span class="file-icon">' + icon + '</span>' + esc(f.name) + '</a>';
            }

            var actions = '';
            if (!f.is_dir) {
                actions =
                    '<div class="btn-group">' +
                        '<button class="btn btn-sm btn-outline" data-action="download" data-path="' + esc(f.path) + '">Download</button>' +
                        '<button class="btn btn-sm btn-outline" data-action="share" data-path="' + esc(f.path) + '">Share</button>' +
                        '<button class="btn btn-sm btn-outline" data-action="history" data-path="' + esc(f.path) + '">History</button>' +
                        '<button class="btn btn-sm btn-danger" data-action="delete" data-path="' + esc(f.path) + '">Delete</button>' +
                    '</div>';
            } else {
                actions =
                    '<div class="btn-group">' +
                        '<button class="btn btn-sm btn-danger" data-action="delete" data-path="' + esc(f.path) + '">Delete</button>' +
                    '</div>';
            }

            html += '<tr class="file-row">' +
                '<td>' + nameLink + '</td>' +
                '<td>' + (f.is_dir ? '-' : formatBytes(f.size)) + '</td>' +
                '<td>' + (f.version || '-') + '</td>' +
                '<td>' + formatDate(f.mod_time) + '</td>' +
                '<td>' + actions + '</td>' +
                '</tr>';
        }

        html += '</tbody></table>';
        table.innerHTML = html;

        // Wire action buttons
        table.querySelectorAll('[data-action]').forEach(function(btn) {
            btn.addEventListener('click', function(e) {
                var action = e.currentTarget.getAttribute('data-action');
                var path = e.currentTarget.getAttribute('data-path');
                handleAction(action, path);
            });
        });
    }

    function handleAction(action, path) {
        if (action === 'download') {
            var a = document.createElement('a');
            a.href = API.downloadUrl(path.replace(/^\//, ''));
            a.download = '';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        } else if (action === 'delete') {
            if (!confirm('Delete ' + path + '?')) return;
            API.del('/api/v1/tree/' + API.encodeURIPath(path.replace(/^\//, ''))).then(function(resp) {
                if (resp.ok) {
                    loadDir(currentPath);
                } else {
                    resp.json().then(function(d) { alert(d.error || 'Delete failed'); });
                }
            });
        } else if (action === 'share') {
            showShareModal(path);
        } else if (action === 'history') {
            window.location.hash = '#versions' + path;
        }
    }

    // Share modal
    function showShareModal(path) {
        var modal = document.getElementById('share-modal');
        modal.innerHTML =
            '<div class="modal-overlay" id="modal-overlay">' +
                '<div class="modal">' +
                    '<button class="modal-close" id="modal-close-btn">&times;</button>' +
                    '<h3>Share: ' + esc(path) + '</h3>' +
                    '<form id="share-form">' +
                        '<div class="form-group">' +
                            '<label>Password (optional)</label>' +
                            '<input type="text" id="share-password" placeholder="Leave empty for no password">' +
                        '</div>' +
                        '<div class="form-group">' +
                            '<label>Expires in (seconds, optional)</label>' +
                            '<input type="number" id="share-expiry" placeholder="e.g. 86400 for 1 day">' +
                        '</div>' +
                        '<div class="form-group">' +
                            '<label>Max downloads (optional)</label>' +
                            '<input type="number" id="share-max-dl" placeholder="0 = unlimited">' +
                        '</div>' +
                        '<button type="submit" class="btn">Create Share Link</button>' +
                    '</form>' +
                    '<div id="share-result"></div>' +
                '</div>' +
            '</div>';

        document.getElementById('modal-close-btn').addEventListener('click', function() {
            modal.innerHTML = '';
        });
        document.getElementById('modal-overlay').addEventListener('click', function(e) {
            if (e.target === e.currentTarget) modal.innerHTML = '';
        });

        document.getElementById('share-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var body = {};
            var pw = document.getElementById('share-password').value;
            var exp = document.getElementById('share-expiry').value;
            var maxDl = document.getElementById('share-max-dl').value;
            if (pw) body.password = pw;
            if (exp) body.expires_in_sec = parseInt(exp, 10);
            if (maxDl) body.max_downloads = parseInt(maxDl, 10);

            API.post('/api/v1/share/' + API.encodeURIPath(path.replace(/^\//, '')), body)
                .then(function(resp) { return resp.json(); })
                .then(function(data) {
                    if (data.error) {
                        document.getElementById('share-result').innerHTML =
                            '<div class="alert alert-error">' + esc(data.error) + '</div>';
                        return;
                    }
                    document.getElementById('share-result').innerHTML =
                        '<div class="alert alert-success">Share link created!</div>' +
                        '<div class="share-url">' +
                            '<input type="text" id="share-url-input" readonly value="' + esc(data.url) + '">' +
                            '<button class="btn btn-sm" id="btn-copy-url">Copy</button>' +
                        '</div>';
                    document.getElementById('btn-copy-url').addEventListener('click', function() {
                        var inp = document.getElementById('share-url-input');
                        inp.select();
                        document.execCommand('copy');
                    });
                });
        });
    }

    // New folder
    document.getElementById('btn-new-folder').addEventListener('click', function() {
        var name = prompt('Folder name:');
        if (!name) return;
        var newPath = (currentPath === '/' ? '' : currentPath) + '/' + name;
        API.put('/api/v1/tree/' + API.encodeURIPath(newPath.replace(/^\//, '')) + '?type=dir')
            .then(function(resp) {
                if (resp.ok) {
                    loadDir(currentPath);
                } else {
                    resp.json().then(function(d) { alert(d.error || 'Failed to create folder'); });
                }
            });
    });

    // Upload button
    document.getElementById('btn-upload').addEventListener('click', function() {
        fileInput.click();
    });

    fileInput.addEventListener('change', function() {
        uploadFiles(fileInput.files);
        fileInput.value = '';
    });

    // Drag and drop
    var tableEl = document.getElementById('file-table');
    tableEl.addEventListener('dragenter', function(e) {
        e.preventDefault();
        dropZone.classList.remove('hidden');
        dropZone.classList.add('drag-over');
    });
    dropZone.addEventListener('dragover', function(e) {
        e.preventDefault();
    });
    dropZone.addEventListener('dragleave', function(e) {
        dropZone.classList.remove('drag-over');
        dropZone.classList.add('hidden');
    });
    dropZone.addEventListener('drop', function(e) {
        e.preventDefault();
        dropZone.classList.remove('drag-over');
        dropZone.classList.add('hidden');
        if (e.dataTransfer.files.length > 0) {
            uploadFiles(e.dataTransfer.files);
        }
    });

    function uploadFiles(files) {
        var status = document.getElementById('browser-status');
        var pending = files.length;
        var errors = [];
        status.innerHTML = '<div class="alert alert-success">Uploading ' + pending + ' file(s)...</div>';

        for (var i = 0; i < files.length; i++) {
            (function(file) {
                var filePath = (currentPath === '/' ? '' : currentPath) + '/' + file.name;
                API.upload(filePath.replace(/^\//, ''), file).then(function(resp) {
                    pending--;
                    if (!resp.ok) {
                        errors.push(file.name);
                    }
                    if (pending === 0) {
                        if (errors.length > 0) {
                            status.innerHTML = '<div class="alert alert-error">Failed to upload: ' + esc(errors.join(', ')) + '</div>';
                        } else {
                            status.innerHTML = '<div class="alert alert-success">Upload complete!</div>';
                            setTimeout(function() { status.innerHTML = ''; }, 2000);
                        }
                        loadDir(currentPath);
                    }
                }).catch(function() {
                    pending--;
                    errors.push(file.name);
                    if (pending === 0) {
                        status.innerHTML = '<div class="alert alert-error">Failed to upload: ' + esc(errors.join(', ')) + '</div>';
                        loadDir(currentPath);
                    }
                });
            })(files[i]);
        }
    }

    // Search/filter
    searchInput.addEventListener('input', function() {
        var q = searchInput.value.toLowerCase();
        if (!q) {
            renderTable(allItems);
            return;
        }
        var filtered = allItems.filter(function(f) {
            return f.name.toLowerCase().indexOf(q) !== -1;
        });
        renderTable(filtered);
    });

    // Load directory contents
    function loadDir(path) {
        buildBreadcrumb(path);
        var apiPath = path === '/' ? '/api/v1/tree' : '/api/v1/tree/' + API.encodeURIPath(path.replace(/^\//, ''));
        API.get(apiPath).then(function(data) {
            if (data.error) {
                document.getElementById('file-table').innerHTML =
                    '<div class="alert alert-error">' + esc(data.error) + '</div>';
                return;
            }
            var root = data.root;
            allItems = root.children || [];
            renderTable(allItems);
        }).catch(function() {
            document.getElementById('file-table').innerHTML =
                '<div class="alert alert-error">Failed to load files</div>';
        });
    }

    loadDir(currentPath);
}
