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
                    '<button class="btn btn-sm btn-outline" id="btn-clear-search" style="display:none" title="Clear" aria-label="Clear search">&times;</button>' +
                '</div>' +
                '<div class="browser-view-toggles" role="radiogroup" aria-label="View mode">' +
                    '<button class="browser-view-btn' + (viewMode === 'list' ? ' active' : '') + '" data-view="list" title="List">&#9776;</button>' +
                    '<button class="browser-view-btn' + (viewMode === 'compact' ? ' active' : '') + '" data-view="compact" title="Compact">&#9866;</button>' +
                    '<button class="browser-view-btn' + (viewMode === 'tiles' ? ' active' : '') + '" data-view="tiles" title="Tiles">&#9638;</button>' +
                '</div>' +
                '<button class="btn btn-sm" id="btn-new-folder">New Folder</button>' +
                '<button class="btn btn-sm" id="btn-new-file">New File</button>' +
                '<button class="btn btn-sm" id="btn-upload">Upload</button>' +
                '<button class="btn btn-sm btn-outline" id="btn-upload-folder">Upload Folder</button>' +
                '<input type="file" id="file-input" multiple style="display:none">' +
                '<input type="file" id="folder-input" webkitdirectory multiple style="display:none">' +
            '</div>' +
        '</div>' +
        '<div id="breadcrumb" class="breadcrumb"></div>' +
        '<div id="drop-zone" class="drop-zone hidden">Drop files here to upload</div>' +
        '<div id="batch-toolbar" class="batch-toolbar hidden"></div>' +
        '<div id="browser-status"></div>' +
        '<div id="file-table" class="table-wrap">' +
            '<div style="padding:0.75rem">' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
            '</div>' +
        '</div>';

    var searchInput = document.getElementById('search-input');
    var fileInput = document.getElementById('file-input');
    var dropZone = document.getElementById('drop-zone');
    var allItems = [];
    var userFavorites = {};

    // Sort state
    var sortField = 'name';
    var sortDir = 'asc';

    // View mode state
    var viewMode = localStorage.getItem('browser-view-mode') || 'list';
    var tileObjectURLs = [];

    // Selection state
    var selectedPaths = {};
    var lastClickedIndex = -1;

    // Build breadcrumb
    function buildBreadcrumb(path) {
        var bc = document.getElementById('breadcrumb');
        var parts = path.split('/').filter(Boolean);
        var html = '<nav aria-label="Breadcrumb"><a href="#browser">/</a>';
        var accumulated = '';
        for (var i = 0; i < parts.length; i++) {
            accumulated += '/' + parts[i];
            html += '<span class="sep" aria-hidden="true">/</span>';
            if (i === parts.length - 1) {
                html += '<a href="#browser' + esc(accumulated) + '" aria-current="page">' + esc(parts[i]) + '</a>';
            } else {
                html += '<a href="#browser' + esc(accumulated) + '">' + esc(parts[i]) + '</a>';
            }
        }
        html += '</nav>';
        bc.innerHTML = html;
    }

    // Sort items
    function sortItems(items) {
        return items.slice().sort(function(a, b) {
            // Dirs always first
            if (a.is_dir && !b.is_dir) return -1;
            if (!a.is_dir && b.is_dir) return 1;

            var cmp = 0;
            if (sortField === 'name') {
                cmp = a.name.localeCompare(b.name);
            } else if (sortField === 'size') {
                cmp = (a.size || 0) - (b.size || 0);
            } else if (sortField === 'version') {
                cmp = (a.version || 0) - (b.version || 0);
            } else if (sortField === 'modified') {
                cmp = (a.mod_time || '').localeCompare(b.mod_time || '');
            }
            return sortDir === 'desc' ? -cmp : cmp;
        });
    }

    function sortIndicator(field) {
        if (sortField !== field) return '';
        return ' <span class="sort-indicator">' + (sortDir === 'asc' ? '&#9650;' : '&#9660;') + '</span>';
    }

    // Selection helpers
    function selectionCount() {
        var n = 0;
        for (var k in selectedPaths) if (selectedPaths.hasOwnProperty(k)) n++;
        return n;
    }

    function clearSelection() {
        selectedPaths = {};
        lastClickedIndex = -1;
        updateBatchToolbar();
    }

    function updateBatchToolbar() {
        var bar = document.getElementById('batch-toolbar');
        var count = selectionCount();
        if (count === 0) {
            bar.classList.add('hidden');
            return;
        }
        bar.classList.remove('hidden');
        bar.innerHTML =
            '<span class="batch-count">' + count + ' selected</span>' +
            '<button class="btn btn-sm" id="batch-download">Download</button>' +
            '<button class="btn btn-sm" id="batch-move">Move</button>' +
            '<button class="btn btn-sm" id="batch-copy">Copy</button>' +
            '<button class="btn btn-sm" id="batch-share">Share</button>' +
            '<button class="btn btn-sm" id="batch-tag">Tag</button>' +
            '<button class="btn btn-sm btn-danger" id="batch-delete">Delete</button>' +
            '<button class="btn btn-sm btn-outline" id="batch-visibility">Visibility</button>' +
            '<button class="btn btn-sm btn-outline" id="batch-deselect">Deselect All</button>';

        document.getElementById('batch-download').addEventListener('click', batchDownload);
        document.getElementById('batch-move').addEventListener('click', function() { showFolderPicker('move'); });
        document.getElementById('batch-copy').addEventListener('click', function() { showFolderPicker('copy'); });
        document.getElementById('batch-share').addEventListener('click', batchShareModal);
        document.getElementById('batch-tag').addEventListener('click', batchTagModal);
        document.getElementById('batch-delete').addEventListener('click', batchDelete);
        document.getElementById('batch-visibility').addEventListener('click', batchVisibility);
        document.getElementById('batch-deselect').addEventListener('click', function() {
            clearSelection();
            syncCheckboxes();
        });
    }

    function syncCheckboxes() {
        var rows = document.querySelectorAll('#file-table .file-row');
        var allChecked = rows.length > 0;
        rows.forEach(function(row) {
            var cb = row.querySelector('.row-checkbox');
            var path = row.getAttribute('data-path');
            var checked = !!selectedPaths[path];
            if (cb) cb.checked = checked;
            if (checked) {
                row.classList.add('selected');
            } else {
                row.classList.remove('selected');
                allChecked = false;
            }
        });
        var headerCb = document.getElementById('select-all-cb');
        if (headerCb) headerCb.checked = allChecked && rows.length > 0;
    }

    // ── View Mode Dispatch ─────────────────────────────────────────────────

    function renderView(items) {
        cleanupTileObjectURLs();
        if (viewMode === 'compact') {
            renderCompact(items);
        } else if (viewMode === 'tiles') {
            renderTiles(items);
        } else {
            renderTable(items);
        }
    }

    function cleanupTileObjectURLs() {
        for (var i = 0; i < tileObjectURLs.length; i++) {
            URL.revokeObjectURL(tileObjectURLs[i]);
        }
        tileObjectURLs = [];
    }

    // ── Shared Row Wiring ───────────────────────────────────────────────────

    function wireFileRows(container, items) {
        // Wire kebab buttons
        container.querySelectorAll('.kebab-btn').forEach(function(btn) {
            btn.addEventListener('click', function(e) {
                e.stopPropagation();
                var row = btn.closest('.file-row');
                var path = row.getAttribute('data-path');
                var isDir = row.getAttribute('data-isdir') === '1';
                var vis = row.getAttribute('data-vis');
                var rect = btn.getBoundingClientRect();
                showKebabMenu(path, isDir, vis, rect.right, rect.bottom);
            });
        });

        // Wire favorite buttons
        container.querySelectorAll('.fav-btn').forEach(function(btn) {
            btn.addEventListener('click', function(e) {
                e.stopPropagation();
                var path = btn.getAttribute('data-fav');
                var isFav = !!userFavorites[path];
                if (isFav) {
                    API.del('/api/v1/favorites/' + API.encodeURIPath(path.replace(/^\//, ''))).then(function(resp) {
                        if (resp.ok) {
                            delete userFavorites[path];
                            btn.classList.remove('fav-active');
                            btn.innerHTML = '&#9734;';
                            btn.title = 'Star';
                        }
                    });
                } else {
                    API.put('/api/v1/favorites/' + API.encodeURIPath(path.replace(/^\//, ''))).then(function(resp) {
                        if (resp.ok) {
                            userFavorites[path] = true;
                            btn.classList.add('fav-active');
                            btn.innerHTML = '&#9733;';
                            btn.title = 'Unstar';
                        }
                    });
                }
            });
        });

        // Wire context menu on rows
        container.querySelectorAll('.file-row').forEach(function(row) {
            row.addEventListener('contextmenu', function(e) {
                e.preventDefault();
                var path = row.getAttribute('data-path');
                var isDir = row.getAttribute('data-isdir') === '1';
                var vis = row.getAttribute('data-vis');
                showKebabMenu(path, isDir, vis, e.clientX, e.clientY);
            });
        });

        // Wire select-all checkbox
        var selectAllCb = document.getElementById('select-all-cb');
        if (selectAllCb) {
            selectAllCb.addEventListener('change', function() {
                var checked = selectAllCb.checked;
                var rows = container.querySelectorAll('.file-row');
                rows.forEach(function(row) {
                    var path = row.getAttribute('data-path');
                    if (checked) {
                        selectedPaths[path] = true;
                    } else {
                        delete selectedPaths[path];
                    }
                });
                syncCheckboxes();
                updateBatchToolbar();
            });
        }

        // Wire per-row checkboxes with shift+click
        container.querySelectorAll('.row-checkbox').forEach(function(cb) {
            cb.addEventListener('click', function(e) {
                e.stopPropagation();
                var row = cb.closest('.file-row');
                var path = row.getAttribute('data-path');
                var idx = parseInt(row.getAttribute('data-idx'), 10);

                if (e.shiftKey && lastClickedIndex >= 0) {
                    var start = Math.min(lastClickedIndex, idx);
                    var end = Math.max(lastClickedIndex, idx);
                    var rows = container.querySelectorAll('.file-row');
                    rows.forEach(function(r) {
                        var rIdx = parseInt(r.getAttribute('data-idx'), 10);
                        if (rIdx >= start && rIdx <= end) {
                            selectedPaths[r.getAttribute('data-path')] = true;
                        }
                    });
                } else {
                    if (cb.checked) {
                        selectedPaths[path] = true;
                    } else {
                        delete selectedPaths[path];
                    }
                }

                lastClickedIndex = idx;
                syncCheckboxes();
                updateBatchToolbar();
            });
        });
    }

    // ── Render: List (default table) ────────────────────────────────────────

    function renderTable(items) {
        var table = document.getElementById('file-table');
        if (!items || items.length === 0) {
            table.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">Empty folder</p>';
            clearSelection();
            return;
        }

        var sorted = sortItems(items);

        var html = '<table class="responsive-table"><thead><tr>' +
            '<th scope="col" class="cb-col"><input type="checkbox" id="select-all-cb"></th>' +
            '<th scope="col" class="fav-col"></th>' +
            '<th scope="col" class="sortable" data-sort="name">Name' + sortIndicator('name') + '</th>' +
            '<th scope="col"></th>' +
            '<th scope="col" class="sortable" data-sort="size">Size' + sortIndicator('size') + '</th>' +
            '<th scope="col" class="sortable" data-sort="version">Version' + sortIndicator('version') + '</th>' +
            '<th scope="col" class="sortable" data-sort="modified">Modified' + sortIndicator('modified') + '</th>' +
            '<th scope="col"></th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < sorted.length; i++) {
            var f = sorted[i];
            var iconHtml = FileTypes.icon(f.name, f.is_dir);
            var nameLink;
            if (f.is_dir) {
                nameLink = '<a class="file-name" href="#browser' + esc(f.path) + '">' +
                    iconHtml + esc(f.name) + '</a>';
            } else {
                nameLink = '<a class="file-name" href="#viewer' + esc(f.path) + '">' +
                    iconHtml + esc(f.name) + '</a>';
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

            var isSelected = !!selectedPaths[f.path];
            var isFav = !!userFavorites[f.path];

            html += '<tr class="file-row' + (isSelected ? ' selected' : '') + '" data-path="' + esc(f.path) + '" data-isdir="' + (f.is_dir ? '1' : '0') + '" data-vis="' + esc(f.visibility || 'public') + '" data-idx="' + i + '">' +
                '<td class="cb-col"><input type="checkbox" class="row-checkbox"' + (isSelected ? ' checked' : '') + '></td>' +
                '<td class="fav-col"><button class="fav-btn' + (isFav ? ' fav-active' : '') + '" data-fav="' + esc(f.path) + '" title="' + (isFav ? 'Unstar' : 'Star') + '" aria-label="' + (isFav ? 'Unstar ' : 'Star ') + esc(f.name) + '">' + (isFav ? '&#9733;' : '&#9734;') + '</button></td>' +
                '<td data-label="Name">' + nameLink + '</td>' +
                '<td data-label="">' + visBadge + '</td>' +
                '<td data-label="Size">' + (f.is_dir ? '-' : formatBytes(f.size)) + '</td>' +
                '<td data-label="Version">' + (f.version || '-') + '</td>' +
                '<td data-label="Modified">' + formatDate(f.mod_time) + '</td>' +
                '<td data-label=""><button class="kebab-btn" data-path="' + esc(f.path) + '" aria-label="Actions for ' + esc(f.name) + '">&#8942;</button></td>' +
                '</tr>';
        }

        html += '</tbody></table>';
        table.innerHTML = html;
        updateBatchToolbar();

        // Wire sortable headers
        table.querySelectorAll('.sortable').forEach(function(th) {
            th.addEventListener('click', function() {
                var field = th.getAttribute('data-sort');
                if (sortField === field) {
                    sortDir = sortDir === 'asc' ? 'desc' : 'asc';
                } else {
                    sortField = field;
                    sortDir = 'asc';
                }
                renderTable(items);
            });
        });

        wireFileRows(table, items);
    }

    // ── Render: Compact ─────────────────────────────────────────────────────

    function renderCompact(items) {
        var container = document.getElementById('file-table');
        if (!items || items.length === 0) {
            container.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">Empty folder</p>';
            clearSelection();
            return;
        }

        var sorted = sortItems(items);

        var html = '<table class="compact-table"><thead><tr>' +
            '<th scope="col" class="cb-col"><input type="checkbox" id="select-all-cb"></th>' +
            '<th scope="col" class="fav-col"></th>' +
            '<th scope="col" class="sortable" data-sort="name">Name' + sortIndicator('name') + '</th>' +
            '<th scope="col" class="sortable" data-sort="size">Size' + sortIndicator('size') + '</th>' +
            '<th scope="col" class="sortable compact-modified" data-sort="modified">Modified' + sortIndicator('modified') + '</th>' +
            '<th scope="col" class="compact-kebab-col"></th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < sorted.length; i++) {
            var f = sorted[i];
            var iconHtml = FileTypes.icon(f.name, f.is_dir);
            var href = f.is_dir ? '#browser' + esc(f.path) : '#viewer' + esc(f.path);
            var isSelected = !!selectedPaths[f.path];
            var isFav = !!userFavorites[f.path];

            html += '<tr class="file-row' + (isSelected ? ' selected' : '') + '" data-path="' + esc(f.path) + '" data-isdir="' + (f.is_dir ? '1' : '0') + '" data-vis="' + esc(f.visibility || 'public') + '" data-idx="' + i + '">' +
                '<td class="cb-col"><input type="checkbox" class="row-checkbox"' + (isSelected ? ' checked' : '') + '></td>' +
                '<td class="fav-col"><button class="fav-btn' + (isFav ? ' fav-active' : '') + '" data-fav="' + esc(f.path) + '" title="' + (isFav ? 'Unstar' : 'Star') + '">' + (isFav ? '&#9733;' : '&#9734;') + '</button></td>' +
                '<td><a class="file-name" href="' + href + '">' + iconHtml + esc(f.name) + '</a></td>' +
                '<td>' + (f.is_dir ? '-' : formatBytes(f.size)) + '</td>' +
                '<td class="compact-modified">' + formatDate(f.mod_time) + '</td>' +
                '<td class="compact-kebab-col"><button class="kebab-btn" data-path="' + esc(f.path) + '" aria-label="Actions for ' + esc(f.name) + '">&#8942;</button></td>' +
                '</tr>';
        }

        html += '</tbody></table>';
        container.innerHTML = html;
        updateBatchToolbar();

        // Wire sortable headers
        container.querySelectorAll('.sortable').forEach(function(th) {
            th.addEventListener('click', function() {
                var field = th.getAttribute('data-sort');
                if (sortField === field) {
                    sortDir = sortDir === 'asc' ? 'desc' : 'asc';
                } else {
                    sortField = field;
                    sortDir = 'asc';
                }
                renderCompact(items);
            });
        });

        wireFileRows(container, items);
    }

    // ── Render: Tiles ───────────────────────────────────────────────────────

    function renderTiles(items) {
        var container = document.getElementById('file-table');
        if (!items || items.length === 0) {
            container.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">Empty folder</p>';
            clearSelection();
            return;
        }

        var sorted = sortItems(items);

        var html = '<div class="tiles-grid">';
        for (var i = 0; i < sorted.length; i++) {
            var f = sorted[i];
            var isSelected = !!selectedPaths[f.path];
            var isFav = !!userFavorites[f.path];
            var href = f.is_dir ? '#browser' + esc(f.path) : '#viewer' + esc(f.path);
            var isImage = !f.is_dir && FileTypes.isGalleryImage(f.name);

            html += '<div class="tile-card file-row' + (isSelected ? ' selected' : '') + '" data-path="' + esc(f.path) + '" data-isdir="' + (f.is_dir ? '1' : '0') + '" data-vis="' + esc(f.visibility || 'public') + '" data-idx="' + i + '">' +
                '<div class="tile-controls">' +
                    '<input type="checkbox" class="row-checkbox"' + (isSelected ? ' checked' : '') + '>' +
                    '<div class="tile-controls-right">' +
                        '<button class="fav-btn' + (isFav ? ' fav-active' : '') + '" data-fav="' + esc(f.path) + '" title="' + (isFav ? 'Unstar' : 'Star') + '">' + (isFav ? '&#9733;' : '&#9734;') + '</button>' +
                        '<button class="kebab-btn" data-path="' + esc(f.path) + '" aria-label="Actions for ' + esc(f.name) + '">&#8942;</button>' +
                    '</div>' +
                '</div>' +
                '<a href="' + href + '" class="tile-link">' +
                    '<div class="tile-thumb">' +
                        (isImage
                            ? '<img class="tile-thumb-img" data-thumb-path="' + esc(f.path) + '" alt="" loading="lazy">'
                            : '<div class="tile-icon">' + FileTypes.icon(f.name, f.is_dir) + '</div>') +
                    '</div>' +
                    '<div class="tile-label">' +
                        '<div class="tile-name" title="' + esc(f.name) + '">' + esc(f.name) + '</div>' +
                        '<div class="tile-meta">' + (f.is_dir ? 'Folder' : formatBytes(f.size)) + '</div>' +
                    '</div>' +
                '</a>' +
                '</div>';
        }
        html += '</div>';
        container.innerHTML = html;
        updateBatchToolbar();

        // Load thumbnails for images
        container.querySelectorAll('.tile-thumb-img[data-thumb-path]').forEach(function(img) {
            var filePath = img.getAttribute('data-thumb-path');
            var url = '/api/v1/gallery/thumb/' + API.encodeURIPath(filePath.replace(/^\//, ''));
            fetch(url, { headers: { 'Authorization': 'Bearer ' + API.getToken() } })
                .then(function(r) { return r.blob(); })
                .then(function(blob) {
                    var objURL = URL.createObjectURL(blob);
                    tileObjectURLs.push(objURL);
                    img.src = objURL;
                })
                .catch(function() {
                    // Fall back to file icon
                    var iconDiv = document.createElement('div');
                    iconDiv.className = 'tile-icon';
                    iconDiv.innerHTML = FileTypes.icon(filePath.split('/').pop(), false);
                    img.parentNode.replaceChild(iconDiv, img);
                });
        });

        wireFileRows(container, items);
    }

    // ── Kebab / Context Menu ────────────────────────────────────────────────

    var activeMenu = null;

    function closeKebabMenu() {
        if (activeMenu) {
            if (activeMenu.parentNode) activeMenu.parentNode.removeChild(activeMenu);
            activeMenu = null;
        }
    }

    document.addEventListener('click', closeKebabMenu);

    function showKebabMenu(path, isDir, vis, x, y) {
        closeKebabMenu();

        var menu = document.createElement('div');
        menu.className = 'kebab-menu';

        var items = [];
        if (isDir) {
            items.push({ label: 'Open', icon: '&#128193;', action: 'open' });
            items.push({ label: 'Properties', icon: '&#9432;', action: 'properties' });
            items.push({ label: 'Rename', icon: '&#9998;', action: 'rename' });
            items.push({ label: 'Visibility', icon: '&#128065;', action: 'visibility' });
            items.push({ sep: true });
            items.push({ label: 'Delete', icon: '&#128465;', action: 'delete', danger: true });
        } else {
            items.push({ label: 'Open', icon: '&#128196;', action: 'open' });
            items.push({ label: 'Download', icon: '&#8615;', action: 'download' });
            items.push({ label: 'Share', icon: '&#128279;', action: 'share' });
            items.push({ label: 'Properties', icon: '&#9432;', action: 'properties' });
            items.push({ label: 'Rename', icon: '&#9998;', action: 'rename' });
            items.push({ label: 'Visibility', icon: '&#128065;', action: 'visibility' });
            items.push({ sep: true });
            items.push({ label: 'Delete', icon: '&#128465;', action: 'delete', danger: true });
        }

        var html = '';
        for (var i = 0; i < items.length; i++) {
            var it = items[i];
            if (it.sep) {
                html += '<div class="kebab-sep"></div>';
            } else {
                html += '<a class="kebab-item' + (it.danger ? ' kebab-danger' : '') + '" data-action="' + it.action + '">' +
                    '<span class="kebab-icon">' + it.icon + '</span>' + esc(it.label) + '</a>';
            }
        }
        menu.innerHTML = html;

        // Position
        menu.style.position = 'fixed';
        menu.style.left = x + 'px';
        menu.style.top = y + 'px';
        document.body.appendChild(menu);

        // Adjust if off-screen
        var rect = menu.getBoundingClientRect();
        if (rect.right > window.innerWidth) {
            menu.style.left = (x - rect.width) + 'px';
        }
        if (rect.bottom > window.innerHeight) {
            menu.style.top = (y - rect.height) + 'px';
        }

        activeMenu = menu;

        // Wire items
        menu.querySelectorAll('.kebab-item').forEach(function(item) {
            item.addEventListener('click', function(e) {
                e.stopPropagation();
                var action = item.getAttribute('data-action');
                closeKebabMenu();
                handleAction(action, path, vis);
            });
        });
    }

    // ── Actions ─────────────────────────────────────────────────────────────

    function handleAction(action, path, extraData) {
        if (action === 'open') {
            var row = document.querySelector('.file-row[data-path="' + CSS.escape(path) + '"]');
            var isDir = row && row.getAttribute('data-isdir') === '1';
            if (isDir) {
                window.location.hash = '#browser' + path;
            } else {
                window.location.hash = '#viewer' + path;
            }
        } else if (action === 'download') {
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
                    Toast.info('Moved to Trash');
                    loadDir(currentPath);
                } else {
                    resp.json().then(function(d) { Toast.error(d.error || 'Delete failed'); });
                }
            });
        } else if (action === 'share') {
            showShareModal(path);
        } else if (action === 'history') {
            window.location.hash = '#versions' + path;
        } else if (action === 'visibility') {
            showVisibilityModal(path, extraData);
        } else if (action === 'properties') {
            showPropertiesPanel(path);
        } else if (action === 'rename') {
            startInlineRename(path);
        }
    }

    // ── Inline Rename ───────────────────────────────────────────────────────

    function startInlineRename(path) {
        var row = document.querySelector('.file-row[data-path="' + CSS.escape(path) + '"]');
        if (!row) return;
        var nameCell = row.querySelector('td:first-child');
        var oldName = path.split('/').pop();

        var input = document.createElement('input');
        input.type = 'text';
        input.className = 'rename-input';
        input.value = oldName;

        nameCell.innerHTML = '';
        nameCell.appendChild(input);
        input.focus();
        // Select name without extension
        var dot = oldName.lastIndexOf('.');
        if (dot > 0) {
            input.setSelectionRange(0, dot);
        } else {
            input.select();
        }

        function commit() {
            var newName = input.value.trim();
            if (!newName || newName === oldName) {
                loadDir(currentPath);
                return;
            }
            var parentDir = path.substring(0, path.lastIndexOf('/'));
            var newPath = parentDir + '/' + newName;
            API.post('/api/v1/rename/' + API.encodeURIPath(path.replace(/^\//, '')),
                { new_path: newPath })
                .then(function(resp) {
                    if (resp.ok) {
                        Toast.success('Renamed to ' + newName);
                        loadDir(currentPath);
                        TreeView.refresh();
                    } else {
                        resp.json().then(function(d) { Toast.error(d.error || 'Rename failed'); });
                        loadDir(currentPath);
                    }
                }).catch(function() {
                    Toast.error('Rename failed');
                    loadDir(currentPath);
                });
        }

        input.addEventListener('keydown', function(e) {
            if (e.key === 'Enter') { e.preventDefault(); commit(); }
            if (e.key === 'Escape') { loadDir(currentPath); }
        });
        input.addEventListener('blur', commit);
    }

    // ── Batch Actions ────────────────────────────────────────────────────────

    function getSelectedPaths() {
        var paths = [];
        for (var k in selectedPaths) if (selectedPaths.hasOwnProperty(k)) paths.push(k);
        return paths;
    }

    function batchDownload() {
        var paths = getSelectedPaths();
        if (paths.length === 0) return;
        if (paths.length === 1) {
            var a = document.createElement('a');
            a.href = API.downloadUrl(paths[0].replace(/^\//, ''));
            a.download = '';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        } else {
            Toast.info('Downloading ' + paths.length + ' files individually...');
            paths.forEach(function(p) {
                // Check it's not a directory
                var row = document.querySelector('.file-row[data-path="' + CSS.escape(p) + '"]');
                if (row && row.getAttribute('data-isdir') === '1') return;
                var a = document.createElement('a');
                a.href = API.downloadUrl(p.replace(/^\//, ''));
                a.download = '';
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
            });
        }
    }

    function batchDelete() {
        var paths = getSelectedPaths();
        if (paths.length === 0) return;
        if (!confirm('Delete ' + paths.length + ' item(s)?')) return;

        var pending = paths.length;
        var errors = [];
        paths.forEach(function(p) {
            API.del('/api/v1/tree/' + API.encodeURIPath(p.replace(/^\//, ''))).then(function(resp) {
                pending--;
                if (!resp.ok) errors.push(p.split('/').pop());
                if (pending === 0) {
                    if (errors.length > 0) {
                        Toast.error('Failed to delete: ' + errors.join(', '));
                    } else {
                        Toast.info('Moved ' + paths.length + ' item(s) to Trash');
                    }
                    clearSelection();
                    loadDir(currentPath);
                }
            }).catch(function() {
                pending--;
                errors.push(p.split('/').pop());
                if (pending === 0) {
                    Toast.error('Failed to delete: ' + errors.join(', '));
                    clearSelection();
                    loadDir(currentPath);
                }
            });
        });
    }

    function batchVisibility() {
        var paths = getSelectedPaths();
        if (paths.length === 0) return;

        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
            '<p style="margin-bottom:0.75rem">Set visibility for ' + paths.length + ' item(s):</p>' +
            '<form id="batch-vis-form">' +
                '<label class="vis-option">' +
                    '<input type="radio" name="vis" value="public" checked>' +
                    '<span class="vis-badge vis-public">&#127760;</span> Public' +
                '</label>' +
                '<label class="vis-option">' +
                    '<input type="radio" name="vis" value="group">' +
                    '<span class="vis-badge vis-group">&#128101;</span> Group' +
                '</label>' +
                '<label class="vis-option">' +
                    '<input type="radio" name="vis" value="private">' +
                    '<span class="vis-badge vis-private">&#128274;</span> Private' +
                '</label>' +
                '<button type="submit" class="btn" style="margin-top:1rem">Apply to All</button>' +
            '</form>';

        Modal.open({ title: 'Batch Visibility', content: contentDiv });

        document.getElementById('batch-vis-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var selected = contentDiv.querySelector('input[name="vis"]:checked').value;
            Modal.close();

            var pending = paths.length;
            var errors = [];
            paths.forEach(function(p) {
                API.put('/api/v1/visibility/' + API.encodeURIPath(p.replace(/^\//, '')), { visibility: selected })
                    .then(function(resp) {
                        pending--;
                        if (!resp.ok) errors.push(p.split('/').pop());
                        if (pending === 0) {
                            if (errors.length > 0) {
                                Toast.error('Failed for: ' + errors.join(', '));
                            } else {
                                Toast.success('Visibility updated for ' + paths.length + ' item(s)');
                            }
                            clearSelection();
                            loadDir(currentPath);
                        }
                    }).catch(function() {
                        pending--;
                        errors.push(p.split('/').pop());
                        if (pending === 0) {
                            Toast.error('Failed for: ' + errors.join(', '));
                            clearSelection();
                            loadDir(currentPath);
                        }
                    });
            });
        });
    }

    // ── Share Modal ─────────────────────────────────────────────────────────

    function showShareModal(path) {
        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
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
            '<div id="share-result"></div>';

        Modal.open({ title: 'Share: ' + path.split('/').pop(), content: contentDiv });

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
                    var shareUrl = window.location.origin + '/app/#share/' + data.id;
                    if (pw) shareUrl += '/' + pw;
                    document.getElementById('share-result').innerHTML =
                        '<div class="alert alert-success">Share link created!</div>' +
                        '<div class="share-url">' +
                            '<input type="text" id="share-url-input" readonly value="' + esc(shareUrl) + '">' +
                            '<button class="btn btn-sm" id="btn-copy-url">Copy</button>' +
                        '</div>';
                    document.getElementById('btn-copy-url').addEventListener('click', function() {
                        var inp = document.getElementById('share-url-input');
                        inp.select();
                        document.execCommand('copy');
                        Toast.success('Copied to clipboard');
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
                    Toast.success('Folder created');
                    loadDir(currentPath);
                } else {
                    resp.json().then(function(d) { Toast.error(d.error || 'Failed to create folder'); });
                }
            });
    });

    // New file
    document.getElementById('btn-new-file').addEventListener('click', function() {
        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
            '<form id="new-file-form">' +
                '<div class="form-group">' +
                    '<label for="new-file-name">File name</label>' +
                    '<input type="text" id="new-file-name" placeholder="e.g. notes.md" required>' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>File type</label>' +
                    '<div class="new-file-types">' +
                        '<label class="vis-option"><input type="radio" name="ftype" value="empty" checked> Empty file</label>' +
                        '<label class="vis-option"><input type="radio" name="ftype" value="text"> Text (.txt)</label>' +
                        '<label class="vis-option"><input type="radio" name="ftype" value="markdown"> Markdown (.md)</label>' +
                        '<label class="vis-option"><input type="radio" name="ftype" value="csv"> Spreadsheet (.csv)</label>' +
                        '<label class="vis-option"><input type="radio" name="ftype" value="html"> Document (.html)</label>' +
                    '</div>' +
                '</div>' +
                '<button type="submit" class="btn">Create</button>' +
            '</form>';

        Modal.open({ title: 'Create New File', content: contentDiv });

        // Auto-fill extension on type change
        var nameInput = document.getElementById('new-file-name');
        var typeRadios = contentDiv.querySelectorAll('input[name="ftype"]');
        var extMap = { empty: '', text: '.txt', markdown: '.md', csv: '.csv', html: '.html' };
        typeRadios.forEach(function(radio) {
            radio.addEventListener('change', function() {
                var cur = nameInput.value.trim();
                var dot = cur.lastIndexOf('.');
                var base = dot > 0 ? cur.substring(0, dot) : (cur || 'untitled');
                var ext = extMap[radio.value] || '';
                nameInput.value = base + ext;
            });
        });

        document.getElementById('new-file-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var name = nameInput.value.trim();
            if (!name) return;
            var ftype = contentDiv.querySelector('input[name="ftype"]:checked').value;

            var templates = {
                empty: '',
                text: '',
                markdown: '# ' + name.replace(/\.md$/i, '') + '\n\n',
                csv: 'Column A,Column B,Column C\n',
                html: '<!DOCTYPE html>\n<html lang="en">\n<head>\n    <meta charset="UTF-8">\n    <title>' +
                    name.replace(/\.html$/i, '') + '</title>\n</head>\n<body>\n    <h1>' +
                    name.replace(/\.html$/i, '') + '</h1>\n    <p></p>\n</body>\n</html>\n'
            };
            var content = templates[ftype] || '';
            var blob = new Blob([content], { type: 'text/plain' });

            var filePath = (currentPath === '/' ? '' : currentPath) + '/' + name;
            Modal.close();

            API.upload(filePath.replace(/^\//, ''), blob).then(function(resp) {
                if (resp.ok) {
                    Toast.success('File created: ' + name);
                    loadDir(currentPath);
                } else {
                    resp.json().then(function(d) { Toast.error(d.error || 'Failed to create file'); });
                }
            }).catch(function() {
                Toast.error('Failed to create file');
            });
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

    // Drag and drop — use counter to handle nested dragenter/dragleave
    var dragCounter = 0;
    var browserArea = app;

    browserArea.addEventListener('dragenter', function(e) {
        e.preventDefault();
        dragCounter++;
        if (dragCounter === 1) {
            dropZone.classList.remove('hidden');
            dropZone.classList.add('drag-over');
        }
    });
    browserArea.addEventListener('dragover', function(e) {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'copy';
    });
    browserArea.addEventListener('dragleave', function(e) {
        e.preventDefault();
        dragCounter--;
        if (dragCounter <= 0) {
            dragCounter = 0;
            dropZone.classList.remove('drag-over');
            dropZone.classList.add('hidden');
        }
    });
    browserArea.addEventListener('drop', function(e) {
        e.preventDefault();
        dragCounter = 0;
        dropZone.classList.remove('drag-over');
        dropZone.classList.add('hidden');

        // Extract files from dataTransfer.items (more reliable than .files after webkitGetAsEntry)
        var items = e.dataTransfer.items;
        if (items && items.length > 0) {
            var dirEntries = [];
            var fileList = [];

            for (var i = 0; i < items.length; i++) {
                if (items[i].kind !== 'file') continue;
                if (items[i].webkitGetAsEntry) {
                    var entry = items[i].webkitGetAsEntry();
                    if (entry && entry.isDirectory) {
                        dirEntries.push(entry);
                        continue;
                    }
                }
                var f = items[i].getAsFile();
                if (f) fileList.push(f);
            }

            if (dirEntries.length > 0) {
                collectFolderFiles(dirEntries).then(function(folderFiles) {
                    if (folderFiles.length > 0) uploadFolderFiles(folderFiles);
                });
                // Also upload any standalone files alongside the folders
                if (fileList.length > 0) uploadFiles(fileList);
                return;
            }

            if (fileList.length > 0) {
                uploadFiles(fileList);
                return;
            }
        }

        // Fallback for browsers without DataTransferItemList
        if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
            uploadFiles(e.dataTransfer.files);
        }
    });

    function uploadFiles(files) {
        if (!files || files.length === 0) return;
        // Expose currentPath for Transfers to trigger loadDir on complete
        window._currentBrowserPath = currentPath;
        for (var i = 0; i < files.length; i++) {
            var filePath = (currentPath === '/' ? '' : currentPath) + '/' + files[i].name;
            Transfers.enqueue(files[i], filePath);
        }
    }

    // Reload file list when a transfer completes
    window.addEventListener('transfer-complete', function() {
        loadDir(currentPath);
    });

    // Folder upload button
    document.getElementById('btn-upload-folder').addEventListener('click', function() {
        document.getElementById('folder-input').click();
    });

    document.getElementById('folder-input').addEventListener('change', function() {
        var folderInput = document.getElementById('folder-input');
        var files = folderInput.files;
        if (!files || files.length === 0) return;

        var fileList = [];
        for (var i = 0; i < files.length; i++) {
            fileList.push({
                file: files[i],
                relativePath: files[i].webkitRelativePath || files[i].name
            });
        }
        uploadFolderFiles(fileList);
        folderInput.value = '';
    });

    // Collect files from dropped directory entries recursively
    function collectFolderFiles(entries) {
        var allFiles = [];
        var promises = [];

        function readEntry(entry, basePath) {
            return new Promise(function(resolve) {
                if (entry.isFile) {
                    entry.file(function(file) {
                        allFiles.push({ file: file, relativePath: basePath + file.name });
                        resolve();
                    }, function() { resolve(); });
                } else if (entry.isDirectory) {
                    var reader = entry.createReader();
                    readAllEntries(reader, basePath + entry.name + '/').then(resolve);
                } else {
                    resolve();
                }
            });
        }

        function readAllEntries(reader, basePath) {
            return new Promise(function(resolve) {
                var allEntries = [];
                function readBatch() {
                    reader.readEntries(function(batch) {
                        if (batch.length === 0) {
                            var ps = allEntries.map(function(e) { return readEntry(e, basePath); });
                            Promise.all(ps).then(resolve);
                        } else {
                            allEntries = allEntries.concat(Array.from(batch));
                            readBatch();
                        }
                    }, function() { resolve(); });
                }
                readBatch();
            });
        }

        for (var i = 0; i < entries.length; i++) {
            promises.push(readEntry(entries[i], ''));
        }

        return Promise.all(promises).then(function() { return allFiles; });
    }

    // Upload folder files via transfer queue
    function uploadFolderFiles(fileList) {
        if (fileList.length === 0) return;
        window._currentBrowserPath = currentPath;

        // Collect unique directory paths
        var dirs = {};
        for (var i = 0; i < fileList.length; i++) {
            var parts = fileList[i].relativePath.split('/');
            var dirPath = '';
            for (var j = 0; j < parts.length - 1; j++) {
                dirPath += (dirPath ? '/' : '') + parts[j];
                dirs[dirPath] = true;
            }
        }

        // Sort dirs by depth (shallowest first)
        var dirList = Object.keys(dirs).sort(function(a, b) {
            return a.split('/').length - b.split('/').length;
        });

        // Create directories first, then enqueue files
        var dirPromise = Promise.resolve();
        for (var d = 0; d < dirList.length; d++) {
            (function(dp) {
                dirPromise = dirPromise.then(function() {
                    var fullDirPath = (currentPath === '/' ? '' : currentPath) + '/' + dp;
                    return API.put('/api/v1/tree/' + API.encodeURIPath(fullDirPath.replace(/^\//, '')) + '?type=dir');
                });
            })(dirList[d]);
        }

        dirPromise.then(function() {
            for (var fi = 0; fi < fileList.length; fi++) {
                var filePath = (currentPath === '/' ? '' : currentPath) + '/' + fileList[fi].relativePath;
                Transfers.enqueue(fileList[fi].file, filePath);
            }
        }).catch(function() {
            Toast.error('Failed to create directories');
        });
    }

    // View toggle wiring
    document.querySelectorAll('.browser-view-btn').forEach(function(btn) {
        btn.addEventListener('click', function() {
            var mode = btn.getAttribute('data-view');
            if (mode === viewMode) return;
            viewMode = mode;
            localStorage.setItem('browser-view-mode', mode);
            document.querySelectorAll('.browser-view-btn').forEach(function(b) {
                b.classList.toggle('active', b.getAttribute('data-view') === mode);
            });
            if (!isSearching) {
                renderView(allItems);
            }
        });
    });

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
        renderView(allItems);
    }

    function globalSearch(query) {
        isSearching = true;
        clearSelection();
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

        var html = '<table class="responsive-table"><thead><tr>' +
            '<th>Name</th><th>Path</th><th>Size</th><th>Modified</th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < shown.length; i++) {
            var f = shown[i];
            var iconHtml = FileTypes.icon(f.name, f.is_dir);
            var href = f.is_dir ? '#browser' + f.path : '#viewer' + f.path;

            // Highlight match in name
            var displayName = highlightMatch(f.name, query);
            var displayPath = highlightMatch(f.path, query);

            html += '<tr class="file-row">' +
                '<td data-label="Name"><a class="file-name" href="' + esc(href) + '">' +
                    iconHtml + displayName + '</a></td>' +
                '<td data-label="Path" class="search-path">' + displayPath + '</td>' +
                '<td data-label="Size">' + (f.is_dir ? '-' : formatBytes(f.size)) + '</td>' +
                '<td data-label="Modified">' + formatDate(f.mod_time) + '</td>' +
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

    // ── Folder Picker Modal (Move / Copy) ──────────────────────────────────

    function showFolderPicker(mode) {
        var paths = getSelectedPaths();
        if (paths.length === 0) return;

        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
            '<p style="margin-bottom:0.75rem">' + (mode === 'move' ? 'Move' : 'Copy') + ' ' + paths.length + ' item(s) to:</p>' +
            '<div class="folder-picker-tree" id="folder-picker-tree">Loading...</div>' +
            '<div style="margin-top:0.75rem">' +
                '<button class="btn" id="folder-picker-confirm" disabled>' + (mode === 'move' ? 'Move Here' : 'Copy Here') + '</button>' +
            '</div>';

        Modal.open({ title: (mode === 'move' ? 'Move' : 'Copy') + ' to Folder', content: contentDiv });

        var selectedDest = null;

        API.get('/api/v1/tree').then(function(data) {
            if (!data.root) return;
            var tree = document.getElementById('folder-picker-tree');
            tree.innerHTML = '';
            renderPickerNode(data.root, tree, 0);
        });

        function renderPickerNode(node, container, depth) {
            if (!node.is_dir && node.path !== '/') return;

            var item = document.createElement('div');
            item.className = 'folder-picker-item';
            item.style.paddingLeft = (depth * 16 + 8) + 'px';
            item.textContent = node.path === '/' ? '/ (root)' : node.name;
            item.setAttribute('data-path', node.path);

            item.addEventListener('click', function() {
                container.querySelectorAll('.folder-picker-item').forEach(function(el) {
                    el.classList.remove('selected');
                });
                item.classList.add('selected');
                selectedDest = node.path;
                document.getElementById('folder-picker-confirm').disabled = false;
            });

            container.appendChild(item);

            if (node.children) {
                for (var i = 0; i < node.children.length; i++) {
                    if (node.children[i].is_dir) {
                        renderPickerNode(node.children[i], container, depth + 1);
                    }
                }
            }
        }

        document.getElementById('folder-picker-confirm').addEventListener('click', function() {
            if (!selectedDest) return;
            Modal.close();

            var endpoint = mode === 'move' ? '/api/v1/bulk/move' : '/api/v1/bulk/copy';
            API.post(endpoint, { paths: paths, destination: selectedDest }).then(function(resp) {
                return resp.json();
            }).then(function(data) {
                if (data.failed > 0) {
                    Toast.error((data.errors || []).join(', ') || 'Some items failed');
                } else {
                    Toast.success((mode === 'move' ? 'Moved' : 'Copied') + ' ' + data.succeeded + ' item(s)');
                }
                clearSelection();
                loadDir(currentPath);
            }).catch(function() {
                Toast.error(mode + ' failed');
            });
        });
    }

    // ── Batch Share Modal ──────────────────────────────────────────────────

    function batchShareModal() {
        var paths = getSelectedPaths();
        if (paths.length === 0) return;

        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
            '<p style="margin-bottom:0.75rem">Create share links for ' + paths.length + ' item(s):</p>' +
            '<form id="batch-share-form">' +
                '<div class="form-group">' +
                    '<label>Password (optional)</label>' +
                    '<input type="text" id="batch-share-password" placeholder="Leave empty for no password">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Expires in (seconds, optional)</label>' +
                    '<input type="number" id="batch-share-expiry" placeholder="e.g. 86400 for 1 day">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Max downloads (optional)</label>' +
                    '<input type="number" id="batch-share-maxdl" placeholder="0 = unlimited">' +
                '</div>' +
                '<button type="submit" class="btn">Create Share Links</button>' +
            '</form>';

        Modal.open({ title: 'Batch Share', content: contentDiv });

        document.getElementById('batch-share-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var body = { paths: paths };
            var pw = document.getElementById('batch-share-password').value;
            var exp = document.getElementById('batch-share-expiry').value;
            var maxDl = document.getElementById('batch-share-maxdl').value;
            if (pw) body.password = pw;
            if (exp) body.expires_in_sec = parseInt(exp, 10);
            if (maxDl) body.max_downloads = parseInt(maxDl, 10);

            Modal.close();

            API.post('/api/v1/bulk/share', body).then(function(resp) {
                return resp.json();
            }).then(function(data) {
                if (data.failed > 0) {
                    Toast.error('Failed for some items: ' + (data.errors || []).join(', '));
                } else {
                    Toast.success('Created ' + data.succeeded + ' share link(s)');
                }
                clearSelection();
            }).catch(function() {
                Toast.error('Batch share failed');
            });
        });
    }

    // ── Batch Tag Modal ─────────────────────────────────────────────────────

    function batchTagModal() {
        var paths = getSelectedPaths();
        if (paths.length === 0) return;

        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
            '<p style="margin-bottom:0.75rem">Add tags to ' + paths.length + ' item(s):</p>' +
            '<form id="batch-tag-form">' +
                '<div class="form-group">' +
                    '<label>Tags (comma-separated)</label>' +
                    '<input type="text" id="batch-tag-input" placeholder="e.g. vacation, family, 2025">' +
                '</div>' +
                '<div id="tag-suggestions" class="tag-suggestions"></div>' +
                '<button type="submit" class="btn" style="margin-top:0.5rem">Apply Tags</button>' +
            '</form>';

        Modal.open({ title: 'Batch Tag', content: contentDiv });

        // Load existing tags for suggestions
        API.get('/api/v1/gallery/tags').then(function(tags) {
            if (!tags || tags.length === 0) return;
            var sugDiv = document.getElementById('tag-suggestions');
            var html = '';
            for (var i = 0; i < Math.min(tags.length, 20); i++) {
                html += '<button type="button" class="tag-suggestion-pill" data-tag="' + esc(tags[i].tag) + '">' +
                    esc(tags[i].tag) + '</button>';
            }
            sugDiv.innerHTML = html;
            sugDiv.querySelectorAll('.tag-suggestion-pill').forEach(function(pill) {
                pill.addEventListener('click', function() {
                    var tagInput = document.getElementById('batch-tag-input');
                    var current = tagInput.value.trim();
                    var tag = pill.getAttribute('data-tag');
                    if (current) {
                        tagInput.value = current + ', ' + tag;
                    } else {
                        tagInput.value = tag;
                    }
                });
            });
        });

        document.getElementById('batch-tag-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var tagStr = document.getElementById('batch-tag-input').value.trim();
            if (!tagStr) {
                Toast.error('Enter at least one tag');
                return;
            }
            var tags = tagStr.split(',').map(function(t) { return t.trim(); }).filter(Boolean);
            if (tags.length === 0) return;

            Modal.close();

            API.post('/api/v1/bulk/tag', { paths: paths, tags: tags }).then(function(resp) {
                return resp.json();
            }).then(function(data) {
                if (data.failed > 0) {
                    Toast.error('Some tags failed: ' + (data.errors || []).join(', '));
                } else {
                    Toast.success('Applied ' + tags.length + ' tag(s) to ' + paths.length + ' item(s)');
                }
                clearSelection();
            }).catch(function() {
                Toast.error('Batch tag failed');
            });
        });
    }

    // Visibility modal
    function showVisibilityModal(path, currentVis) {
        currentVis = currentVis || 'public';
        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
            '<form id="vis-form" style="padding:0.5rem 0">' +
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
            '</form>';

        Modal.open({ title: 'Visibility: ' + path.split('/').pop(), content: contentDiv });

        document.getElementById('vis-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var selected = contentDiv.querySelector('input[name="vis"]:checked').value;
            API.put('/api/v1/visibility/' + API.encodeURIPath(path.replace(/^\//, '')), { visibility: selected })
                .then(function(resp) {
                    if (resp.ok) {
                        Modal.close();
                        Toast.success('Visibility updated');
                        loadDir(currentPath);
                    } else {
                        resp.json().then(function(d) { Toast.error(d.error || 'Failed to set visibility'); });
                    }
                });
        });
    }

    // Properties modal
    function showPropertiesPanel(path) {
        var panel = document.getElementById('detail-panel');
        panel.innerHTML =
            '<div class="detail-panel-header">' +
                '<h3>Properties</h3>' +
                '<button class="detail-panel-close" id="detail-panel-close">&times;</button>' +
            '</div>' +
            '<div class="detail-panel-body"><div class="props-loading">Loading...</div></div>';
        panel.classList.remove('hidden');
        panel.classList.add('open');

        document.getElementById('detail-panel-close').addEventListener('click', closeDetailPanel);

        var body = panel.querySelector('.detail-panel-body');

        API.get('/api/v1/properties/' + API.encodeURIPath(path.replace(/^\//, ''))).then(function(data) {
            if (data.error) {
                body.innerHTML = '<div class="alert alert-error">' + esc(data.error) + '</div>';
                return;
            }
            renderProperties(data, path, body);
        }).catch(function() {
            body.innerHTML = '<div class="alert alert-error">Failed to load properties</div>';
        });
    }

    function closeDetailPanel() {
        var panel = document.getElementById('detail-panel');
        panel.classList.remove('open');
        panel.classList.add('hidden');
        panel.innerHTML = '';
    }

    function renderProperties(data, path, container) {
        var iconHtml = FileTypes.icon(data.name, data.is_dir);
        var visIcon = data.visibility === 'private' ? '&#128274;' : (data.visibility === 'group' ? '&#128101;' : '&#127760;');
        var visClass = 'vis-' + (data.visibility || 'public');
        var visLabel = (data.visibility || 'public').charAt(0).toUpperCase() + (data.visibility || 'public').slice(1);

        var html = '';

        // Header with name and icon
        html += '<div class="props-header">' +
            '<span class="props-icon">' + iconHtml + '</span>' +
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
                closeDetailPanel();
                showVisibilityModal(path, data.visibility || 'public');
            });
        }

        // Wire create share link
        var newShareBtn = document.getElementById('props-new-share');
        if (newShareBtn) {
            newShareBtn.addEventListener('click', function() {
                closeDetailPanel();
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
                        showPropertiesPanel(path);
                    } else {
                        resp.json().then(function(d) { Toast.error(d.error || 'Failed to revoke'); });
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
                        showPropertiesPanel(path);
                    } else {
                        resp.json().then(function(d) { Toast.error(d.error || 'Failed to remove'); });
                    }
                });
            });
        });
    }

    // Load directory contents
    function loadDir(path) {
        buildBreadcrumb(path);
        var apiPath = path === '/' ? '/api/v1/tree' : '/api/v1/tree/' + API.encodeURIPath(path.replace(/^\//, ''));

        // Fetch favorites and directory in parallel
        Promise.all([
            API.get(apiPath),
            API.get('/api/v1/favorites/paths')
        ]).then(function(results) {
            var data = results[0];
            var favPaths = results[1];

            // Build favorites map
            userFavorites = {};
            if (Array.isArray(favPaths)) {
                for (var i = 0; i < favPaths.length; i++) {
                    userFavorites[favPaths[i]] = true;
                }
            }

            if (data.error) {
                document.getElementById('file-table').innerHTML =
                    '<div class="alert alert-error">' + esc(data.error) + '</div>';
                return;
            }
            var root = data.root;
            allItems = root.children || [];
            renderView(allItems);
        }).catch(function() {
            document.getElementById('file-table').innerHTML =
                '<div class="alert alert-error">Failed to load files</div>';
        });
    }

    loadDir(currentPath);
}

function renderFavorites() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Favorites</h2>' +
        '</div>' +
        '<div id="favorites-table" class="table-wrap">' +
            '<div style="padding:0.75rem">' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
                '<div class="skeleton skeleton-row"></div>' +
            '</div>' +
        '</div>';

    API.get('/api/v1/favorites').then(function(items) {
        var table = document.getElementById('favorites-table');

        if (!items || items.length === 0) {
            table.innerHTML =
                '<div class="empty-state">' +
                    '<span class="empty-icon">&#11088;</span>' +
                    '<p>No favorites yet</p>' +
                    '<p class="empty-hint">Star files in the file browser to add them here</p>' +
                '</div>';
            return;
        }

        var html = '<table class="responsive-table"><thead><tr>' +
            '<th>Name</th>' +
            '<th>Path</th>' +
            '<th>Size</th>' +
            '<th>Modified</th>' +
            '<th></th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < items.length; i++) {
            var f = items[i];
            var iconHtml = FileTypes.icon(f.file_name || f.file_path.split('/').pop(), f.is_dir);
            var href = f.is_dir ? '#browser' + f.file_path : '#viewer' + f.file_path;
            var displayName = f.file_name || f.file_path.split('/').pop();

            html += '<tr class="file-row">' +
                '<td data-label="Name"><a class="file-name" href="' + esc(href) + '">' +
                    iconHtml + esc(displayName) + '</a></td>' +
                '<td data-label="Path" class="search-path"><code>' + esc(f.file_path) + '</code></td>' +
                '<td data-label="Size">' + (f.is_dir ? '-' : formatBytes(f.size)) + '</td>' +
                '<td data-label="Modified">' + formatDate(f.mod_time) + '</td>' +
                '<td data-label="">' +
                    '<button class="btn btn-sm btn-outline" data-unfav="' + esc(f.file_path) + '" title="Remove favorite">&#9733;</button>' +
                '</td>' +
                '</tr>';
        }

        html += '</tbody></table>';
        table.innerHTML = html;

        // Wire unstar buttons
        table.querySelectorAll('[data-unfav]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var path = btn.getAttribute('data-unfav');
                API.del('/api/v1/favorites/' + API.encodeURIPath(path.replace(/^\//, ''))).then(function(resp) {
                    if (resp.ok) {
                        Toast.info('Removed from favorites');
                        renderFavorites();
                    }
                });
            });
        });
    }).catch(function() {
        document.getElementById('favorites-table').innerHTML =
            '<div class="alert alert-error">Failed to load favorites</div>';
    });
}
