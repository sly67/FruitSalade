function renderBrowser() {
    var hash = window.location.hash.replace('#browser', '').replace(/^\//, '');
    var currentPath = hash ? '/' + decodeURIComponent(hash) : '/';

    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Files</h2>' +
            '<div class="toolbar-actions">' +
                '<div class="search-wrap">' +
                    '<input type="text" id="search-input" placeholder="Search all files...">' +
                    '<button class="btn btn-sm btn-outline" id="btn-clear-search" style="display:none" title="Clear">&times;</button>' +
                '</div>' +
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
            '<th>Name</th><th></th><th>Size</th><th>Version</th><th>Modified</th><th>Actions</th>' +
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

            // Visibility badge
            var visBadge = '';
            if (f.visibility === 'private') {
                visBadge = '<span class="vis-badge vis-private" title="Private">&#128274;</span>';
            } else if (f.visibility === 'group') {
                visBadge = '<span class="vis-badge vis-group" title="Group">&#128101;</span>';
            } else if (f.visibility && f.visibility !== 'public') {
                visBadge = '<span class="vis-badge vis-public" title="Public">&#127760;</span>';
            }

            var actions = '';
            if (!f.is_dir) {
                actions =
                    '<div class="btn-group">' +
                        '<button class="btn btn-sm btn-outline" data-action="properties" data-path="' + esc(f.path) + '">Properties</button>' +
                        '<button class="btn btn-sm btn-outline" data-action="download" data-path="' + esc(f.path) + '">Download</button>' +
                        '<button class="btn btn-sm btn-outline" data-action="share" data-path="' + esc(f.path) + '">Share</button>' +
                        '<button class="btn btn-sm btn-danger" data-action="delete" data-path="' + esc(f.path) + '">Delete</button>' +
                    '</div>';
            } else {
                actions =
                    '<div class="btn-group">' +
                        '<button class="btn btn-sm btn-outline" data-action="properties" data-path="' + esc(f.path) + '">Properties</button>' +
                        '<button class="btn btn-sm btn-danger" data-action="delete" data-path="' + esc(f.path) + '">Delete</button>' +
                    '</div>';
            }

            html += '<tr class="file-row">' +
                '<td>' + nameLink + '</td>' +
                '<td>' + visBadge + '</td>' +
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
                var vis = e.currentTarget.getAttribute('data-vis');
                handleAction(action, path, vis);
            });
        });
    }

    function handleAction(action, path, extraData) {
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
        } else if (action === 'visibility') {
            showVisibilityModal(path, extraData);
        } else if (action === 'properties') {
            showPropertiesModal(path);
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

    // Global search
    var searchTimer = null;
    var isSearching = false;
    var clearBtn = document.getElementById('btn-clear-search');

    searchInput.addEventListener('input', function() {
        var q = searchInput.value.trim();
        clearTimeout(searchTimer);

        if (!q) {
            clearSearch();
            return;
        }

        clearBtn.style.display = 'inline-flex';

        // Debounce: local filter immediately, global search after 300ms
        var localQ = q.toLowerCase();
        var localFiltered = allItems.filter(function(f) {
            return f.name.toLowerCase().indexOf(localQ) !== -1;
        });
        if (!isSearching) {
            renderTable(localFiltered);
        }

        searchTimer = setTimeout(function() {
            globalSearch(q);
        }, 300);
    });

    clearBtn.addEventListener('click', function() {
        searchInput.value = '';
        clearSearch();
        searchInput.focus();
    });

    function clearSearch() {
        isSearching = false;
        clearBtn.style.display = 'none';
        document.getElementById('breadcrumb').classList.remove('hidden');
        renderTable(allItems);
    }

    function globalSearch(query) {
        isSearching = true;
        var q = query.toLowerCase();
        var bc = document.getElementById('breadcrumb');
        bc.classList.add('hidden');

        API.get('/api/v1/tree').then(function(data) {
            if (data.error) return;
            var flat = [];
            flattenTree(data.root, flat);
            var results = flat.filter(function(f) {
                return f.name.toLowerCase().indexOf(q) !== -1 ||
                       f.path.toLowerCase().indexOf(q) !== -1;
            });
            renderSearchResults(results, query);
        });
    }

    function flattenTree(node, out) {
        if (!node) return;
        if (node.path && node.path !== '/') {
            out.push(node);
        }
        if (node.children) {
            for (var i = 0; i < node.children.length; i++) {
                flattenTree(node.children[i], out);
            }
        }
    }

    function renderSearchResults(items, query) {
        // Check search is still active
        if (!isSearching) return;

        var table = document.getElementById('file-table');
        if (!items || items.length === 0) {
            table.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">No results for "' + esc(query) + '"</p>';
            return;
        }

        // Sort: dirs first, then by path
        items.sort(function(a, b) {
            if (a.is_dir && !b.is_dir) return -1;
            if (!a.is_dir && b.is_dir) return 1;
            return a.path.localeCompare(b.path);
        });

        // Limit results
        var total = items.length;
        var shown = items.slice(0, 200);

        var html = '<table><thead><tr>' +
            '<th>Name</th><th>Path</th><th>Size</th><th>Modified</th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < shown.length; i++) {
            var f = shown[i];
            var icon = f.is_dir ? '&#128193;' : '&#128196;';
            var href = f.is_dir ? '#browser' + f.path : '#viewer' + f.path;

            // Highlight match in name
            var displayName = highlightMatch(f.name, query);
            var displayPath = highlightMatch(f.path, query);

            html += '<tr class="file-row">' +
                '<td><a class="file-name" href="' + esc(href) + '">' +
                    '<span class="file-icon">' + icon + '</span>' + displayName + '</a></td>' +
                '<td class="search-path">' + displayPath + '</td>' +
                '<td>' + (f.is_dir ? '-' : formatBytes(f.size)) + '</td>' +
                '<td>' + formatDate(f.mod_time) + '</td>' +
                '</tr>';
        }

        html += '</tbody></table>';
        if (total > 200) {
            html += '<p style="padding:0.75rem 1rem;color:var(--text-muted);font-size:0.85rem">Showing 200 of ' + total + ' results</p>';
        }
        table.innerHTML = '<div style="padding:0.5rem 1rem;font-size:0.85rem;color:var(--text-muted);border-bottom:1px solid var(--border)">' +
            total + ' result' + (total !== 1 ? 's' : '') + ' for "' + esc(query) + '"</div>' + html;
    }

    function highlightMatch(text, query) {
        var lower = text.toLowerCase();
        var q = query.toLowerCase();
        var idx = lower.indexOf(q);
        if (idx === -1) return esc(text);
        return esc(text.substring(0, idx)) +
            '<mark>' + esc(text.substring(idx, idx + query.length)) + '</mark>' +
            esc(text.substring(idx + query.length));
    }

    // Visibility modal
    function showVisibilityModal(path, currentVis) {
        var modal = document.getElementById('share-modal');
        currentVis = currentVis || 'public';
        modal.innerHTML =
            '<div class="modal-overlay" id="modal-overlay">' +
                '<div class="modal">' +
                    '<button class="modal-close" id="modal-close-btn">&times;</button>' +
                    '<h3>Visibility: ' + esc(path) + '</h3>' +
                    '<form id="vis-form" style="padding:1rem 0">' +
                        '<label class="vis-option">' +
                            '<input type="radio" name="vis" value="public"' + (currentVis === 'public' ? ' checked' : '') + '>' +
                            '<span class="vis-badge vis-public">&#127760;</span> Public - visible to anyone with permission' +
                        '</label>' +
                        '<label class="vis-option">' +
                            '<input type="radio" name="vis" value="group"' + (currentVis === 'group' ? ' checked' : '') + '>' +
                            '<span class="vis-badge vis-group">&#128101;</span> Group - visible to group members only' +
                        '</label>' +
                        '<label class="vis-option">' +
                            '<input type="radio" name="vis" value="private"' + (currentVis === 'private' ? ' checked' : '') + '>' +
                            '<span class="vis-badge vis-private">&#128274;</span> Private - visible to owner only' +
                        '</label>' +
                        '<button type="submit" class="btn" style="margin-top:1rem">Save</button>' +
                    '</form>' +
                '</div>' +
            '</div>';

        document.getElementById('modal-close-btn').addEventListener('click', function() {
            modal.innerHTML = '';
        });
        document.getElementById('modal-overlay').addEventListener('click', function(e) {
            if (e.target === e.currentTarget) modal.innerHTML = '';
        });

        document.getElementById('vis-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var selected = modal.querySelector('input[name="vis"]:checked').value;
            API.put('/api/v1/visibility/' + API.encodeURIPath(path.replace(/^\//, '')), { visibility: selected })
                .then(function(resp) {
                    if (resp.ok) {
                        modal.innerHTML = '';
                        loadDir(currentPath);
                    } else {
                        resp.json().then(function(d) { alert(d.error || 'Failed to set visibility'); });
                    }
                });
        });
    }

    // Properties modal
    function showPropertiesModal(path) {
        var modal = document.getElementById('share-modal');
        modal.innerHTML =
            '<div class="modal-overlay" id="modal-overlay">' +
                '<div class="modal props-modal">' +
                    '<button class="modal-close" id="modal-close-btn">&times;</button>' +
                    '<h3>Properties</h3>' +
                    '<div id="props-content" class="props-loading">Loading...</div>' +
                '</div>' +
            '</div>';

        document.getElementById('modal-close-btn').addEventListener('click', function() {
            modal.innerHTML = '';
        });
        document.getElementById('modal-overlay').addEventListener('click', function(e) {
            if (e.target === e.currentTarget) modal.innerHTML = '';
        });

        API.get('/api/v1/properties/' + API.encodeURIPath(path.replace(/^\//, ''))).then(function(data) {
            if (data.error) {
                document.getElementById('props-content').innerHTML =
                    '<div class="alert alert-error">' + esc(data.error) + '</div>';
                return;
            }
            renderProperties(data, path, modal);
        }).catch(function() {
            document.getElementById('props-content').innerHTML =
                '<div class="alert alert-error">Failed to load properties</div>';
        });
    }

    function renderProperties(data, path, modal) {
        var container = document.getElementById('props-content');
        if (!container) return;

        var icon = data.is_dir ? '&#128193;' : '&#128196;';
        var visIcon = data.visibility === 'private' ? '&#128274;' : (data.visibility === 'group' ? '&#128101;' : '&#127760;');
        var visClass = 'vis-' + (data.visibility || 'public');
        var visLabel = (data.visibility || 'public').charAt(0).toUpperCase() + (data.visibility || 'public').slice(1);

        var html = '';

        // Header with name and icon
        html += '<div class="props-header">' +
            '<span class="props-icon">' + icon + '</span>' +
            '<div class="props-name">' + esc(data.name) + '</div>' +
            '</div>';

        // Metadata grid
        html += '<div class="props-section"><div class="props-label">Details</div><div class="props-grid">';
        html += '<div class="props-key">Path</div><div class="props-val"><code>' + esc(data.path) + '</code></div>';
        if (!data.is_dir) {
            html += '<div class="props-key">Size</div><div class="props-val">' + formatBytes(data.size) + '</div>';
            html += '<div class="props-key">Hash</div><div class="props-val"><code class="props-hash">' + esc(data.hash || '-') + '</code></div>';
            html += '<div class="props-key">Version</div><div class="props-val">v' + (data.version || 1);
            if (data.version_count > 0) {
                html += ' <span class="props-muted">(' + data.version_count + ' previous)</span>';
            }
            html += '</div>';
        }
        html += '<div class="props-key">Modified</div><div class="props-val">' + formatDate(data.mod_time) + '</div>';
        html += '<div class="props-key">Type</div><div class="props-val">' + (data.is_dir ? 'Folder' : 'File') + '</div>';
        html += '</div></div>';

        // Ownership & Visibility
        html += '<div class="props-section"><div class="props-label">Ownership &amp; Visibility</div><div class="props-grid">';
        html += '<div class="props-key">Owner</div><div class="props-val">' + (data.owner_name ? esc(data.owner_name) : '<span class="props-muted">none</span>') + '</div>';
        html += '<div class="props-key">Group</div><div class="props-val">' + (data.group_name ? esc(data.group_name) : '<span class="props-muted">none</span>') + '</div>';
        html += '<div class="props-key">Visibility</div><div class="props-val">' +
            '<span class="vis-badge ' + visClass + '">' + visIcon + '</span> ' + visLabel +
            ' <button class="btn btn-sm btn-outline props-inline-btn" id="props-change-vis">Change</button></div>';
        html += '</div></div>';

        // Permissions
        html += '<div class="props-section"><div class="props-label">Permissions</div>';
        if (data.permissions && data.permissions.length > 0) {
            html += '<table class="props-table"><thead><tr><th>User</th><th>Permission</th><th></th></tr></thead><tbody>';
            for (var i = 0; i < data.permissions.length; i++) {
                var p = data.permissions[i];
                html += '<tr><td>' + esc(p.username || 'User #' + p.user_id) + '</td>' +
                    '<td><span class="badge badge-blue">' + esc(p.permission) + '</span></td>' +
                    '<td><button class="btn btn-sm btn-danger" data-revoke-perm="' + p.user_id + '">Remove</button></td></tr>';
            }
            html += '</tbody></table>';
        } else {
            html += '<div class="props-empty">No explicit permissions set</div>';
        }
        html += '</div>';

        // Share links
        if (!data.is_dir) {
            html += '<div class="props-section"><div class="props-label">Share Links</div>';
            if (data.share_links && data.share_links.length > 0) {
                html += '<table class="props-table"><thead><tr><th>Created by</th><th>Downloads</th><th>Expires</th><th></th></tr></thead><tbody>';
                for (var j = 0; j < data.share_links.length; j++) {
                    var sl = data.share_links[j];
                    var dlInfo = sl.download_count + (sl.max_downloads > 0 ? '/' + sl.max_downloads : '');
                    var expInfo = sl.expires_at ? formatDate(sl.expires_at) : 'Never';
                    html += '<tr><td>' + esc(sl.created_by) + '</td>' +
                        '<td>' + dlInfo + '</td>' +
                        '<td>' + expInfo + '</td>' +
                        '<td><button class="btn btn-sm btn-danger" data-revoke-link="' + esc(sl.id) + '">Revoke</button></td></tr>';
                }
                html += '</tbody></table>';
            } else {
                html += '<div class="props-empty">No active share links</div>';
            }
            html += '<button class="btn btn-sm" id="props-new-share" style="margin-top:0.5rem">Create Share Link</button>';
            html += '</div>';
        }

        // Quick actions
        html += '<div class="props-actions">';
        if (!data.is_dir) {
            html += '<a class="btn btn-sm btn-outline" href="' + esc(API.downloadUrl(path.replace(/^\//, ''))) + '" download>Download</a>';
            html += '<a class="btn btn-sm btn-outline" href="#versions' + esc(path) + '">Version History</a>';
        }
        html += '</div>';

        container.innerHTML = html;
        container.className = '';

        // Wire change visibility button
        var changeVisBtn = document.getElementById('props-change-vis');
        if (changeVisBtn) {
            changeVisBtn.addEventListener('click', function() {
                modal.innerHTML = '';
                showVisibilityModal(path, data.visibility || 'public');
            });
        }

        // Wire create share link
        var newShareBtn = document.getElementById('props-new-share');
        if (newShareBtn) {
            newShareBtn.addEventListener('click', function() {
                modal.innerHTML = '';
                showShareModal(path);
            });
        }

        // Wire revoke share link buttons
        container.querySelectorAll('[data-revoke-link]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var linkId = btn.getAttribute('data-revoke-link');
                if (!confirm('Revoke this share link?')) return;
                API.del('/api/v1/share/' + encodeURIComponent(linkId)).then(function(resp) {
                    if (resp.ok) {
                        showPropertiesModal(path);
                    } else {
                        resp.json().then(function(d) { alert(d.error || 'Failed to revoke'); });
                    }
                });
            });
        });

        // Wire revoke permission buttons
        container.querySelectorAll('[data-revoke-perm]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var uid = btn.getAttribute('data-revoke-perm');
                if (!confirm('Remove this permission?')) return;
                API.del('/api/v1/permissions/' + API.encodeURIPath(path.replace(/^\//, '')) + '?user_id=' + uid).then(function(resp) {
                    if (resp.ok) {
                        showPropertiesModal(path);
                    } else {
                        resp.json().then(function(d) { alert(d.error || 'Failed to remove'); });
                    }
                });
            });
        });
    }

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
