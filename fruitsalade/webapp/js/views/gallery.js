// Gallery view — image browsing with grid/list views, lightbox, EXIF, tags
function renderGallery() {
    var app = document.getElementById('app');

    // ── State ────────────────────────────────────────────────────────────────
    var items = [];
    var total = 0;
    var offset = 0;
    var limit = 50;
    var hasMore = false;
    var loading = false;
    var currentIndex = -1;
    var objectURLs = [];
    var lightboxObjectURL = null;
    var observer = null;

    // View mode from localStorage
    var viewMode = localStorage.getItem('gallery-view-mode') || 'grid';

    // Sort state
    var sortBy = 'date';
    var sortOrder = 'desc';

    // Filter state
    var filterQuery = '';
    var filterDateFrom = '';
    var filterDateTo = '';
    var filterTags = [];
    var filterCameraMake = '';
    var filterCountry = '';

    // Page state: 'images' | 'albums' | 'tags'
    var currentPage = 'images';
    var albumTab = 'my-albums';
    var albumData = { date: null, location: null, camera: null };

    // Custom albums state
    var userAlbums = null;
    var activeAlbumId = null;
    var activeAlbumName = '';
    var allTags = null;

    // ── Render Shell ─────────────────────────────────────────────────────────
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Gallery</h2>' +
            '<div class="toolbar-actions">' +
                '<input type="text" id="gallery-search-input" placeholder="Search images..." class="gallery-toolbar-search">' +
                '<div class="gallery-view-toggles">' +
                    '<button class="gallery-view-btn' + (viewMode === 'list' ? ' active' : '') + '" data-mode="list" title="List">&#9776;</button>' +
                    '<button class="gallery-view-btn' + (viewMode === 'list-thumb' ? ' active' : '') + '" data-mode="list-thumb" title="List + Thumbnails">&#9783;</button>' +
                    '<button class="gallery-view-btn' + (viewMode === 'grid' ? ' active' : '') + '" data-mode="grid" title="Grid">&#9638;</button>' +
                    '<button class="gallery-view-btn' + (viewMode === 'grid-ext' ? ' active' : '') + '" data-mode="grid-ext" title="Grid + Details">&#9641;</button>' +
                '</div>' +
                '<select id="gallery-sort">' +
                    '<option value="date"' + (sortBy === 'date' ? ' selected' : '') + '>Sort: Date</option>' +
                    '<option value="name"' + (sortBy === 'name' ? ' selected' : '') + '>Sort: Name</option>' +
                    '<option value="size"' + (sortBy === 'size' ? ' selected' : '') + '>Sort: Size</option>' +
                '</select>' +
            '</div>' +
        '</div>' +
        '<div class="gallery-page-nav">' +
            '<button class="gallery-page-tab active" data-page="images">Images</button>' +
            '<button class="gallery-page-tab" data-page="albums">Albums</button>' +
            '<button class="gallery-page-tab" data-page="tags">Tags</button>' +
            '<button class="gallery-page-tab" data-page="map">Map</button>' +
            '<button class="gallery-page-tab" data-page="settings">Settings</button>' +
        '</div>' +
        '<div id="gallery-album-breadcrumb" class="gallery-album-breadcrumb hidden"></div>' +
        '<div id="gallery-status"></div>' +
        '<div id="gallery-container"></div>' +
        '<div id="gallery-sentinel" style="height:1px"></div>';

    // ── Thumbnail Loading ────────────────────────────────────────────────────

    function loadThumb(imgEl, filePath) {
        var url = '/api/v1/gallery/thumb/' + API.encodeURIPath(filePath.replace(/^\//, ''));
        fetch(url, { headers: { 'Authorization': 'Bearer ' + API.getToken() } })
            .then(function(r) { return r.blob(); })
            .then(function(blob) {
                var objURL = URL.createObjectURL(blob);
                objectURLs.push(objURL);
                imgEl.src = objURL;
            })
            .catch(function() { imgEl.alt = 'No thumbnail'; });
    }

    function cleanupObjectURLs() {
        for (var i = 0; i < objectURLs.length; i++) {
            URL.revokeObjectURL(objectURLs[i]);
        }
        objectURLs = [];
    }

    // ── Page Navigation ─────────────────────────────────────────────────────

    var pageTabs = app.querySelectorAll('.gallery-page-tab');
    for (var pi = 0; pi < pageTabs.length; pi++) {
        (function(btn) {
            btn.addEventListener('click', function() {
                var page = btn.getAttribute('data-page');
                if (page === currentPage) return;
                currentPage = page;

                // Update tab active state
                var tabs = app.querySelectorAll('.gallery-page-tab');
                for (var j = 0; j < tabs.length; j++) tabs[j].classList.remove('active');
                btn.classList.add('active');

                // Show/hide toolbar controls based on page
                var viewToggles = app.querySelector('.gallery-view-toggles');
                var sortSelect = document.getElementById('gallery-sort');
                var searchInput = document.getElementById('gallery-search-input');
                var sentinel = document.getElementById('gallery-sentinel');
                var status = document.getElementById('gallery-status');

                if (page === 'images') {
                    viewToggles.style.display = '';
                    sortSelect.style.display = '';
                    searchInput.style.display = '';
                    sentinel.style.display = '';
                    if (status) status.style.display = '';
                    loadGallery(false);
                } else {
                    viewToggles.style.display = 'none';
                    sortSelect.style.display = 'none';
                    searchInput.style.display = 'none';
                    sentinel.style.display = 'none';
                    if (status) status.style.display = 'none';
                    // Exit album view if active
                    if (activeAlbumId) {
                        activeAlbumId = null;
                        activeAlbumName = '';
                        updateAlbumBreadcrumb();
                    }
                    if (page === 'albums') {
                        renderAlbumsPage();
                    } else if (page === 'tags') {
                        renderTagsPage();
                    } else if (page === 'map') {
                        renderMapPage();
                    } else if (page === 'settings') {
                        renderSettingsPage();
                    }
                }
            });
        })(pageTabs[pi]);
    }

    // ── Search / Fetch ───────────────────────────────────────────────────────

    function buildSearchURL(off) {
        var params = [];
        if (filterQuery) params.push('query=' + encodeURIComponent(filterQuery));
        if (filterDateFrom) params.push('date_from=' + encodeURIComponent(filterDateFrom));
        if (filterDateTo) params.push('date_to=' + encodeURIComponent(filterDateTo));
        if (filterTags.length > 0) params.push('tags=' + encodeURIComponent(filterTags.join(',')));
        if (filterCameraMake) params.push('camera_make=' + encodeURIComponent(filterCameraMake));
        if (filterCountry) params.push('country=' + encodeURIComponent(filterCountry));
        params.push('sort_by=' + encodeURIComponent(sortBy));
        params.push('sort_order=' + encodeURIComponent(sortOrder));
        params.push('limit=' + limit);
        params.push('offset=' + off);
        return '/api/v1/gallery/search?' + params.join('&');
    }

    function loadGallery(append) {
        if (loading) return;
        loading = true;

        // If inside an album view, load album images instead
        if (activeAlbumId) {
            loadAlbumGallery(append);
            return;
        }

        var url = buildSearchURL(append ? offset : 0);

        API.get(url).then(function(data) {
            loading = false;
            if (data.error) {
                Toast.error(data.error);
                return;
            }

            if (append) {
                items = items.concat(data.items || []);
            } else {
                cleanupObjectURLs();
                items = data.items || [];
            }
            total = data.total || 0;
            offset = data.offset + data.limit;
            hasMore = !!data.has_more;

            renderGalleryItems(append);
            updateStatus();
        }).catch(function() {
            loading = false;
            Toast.error('Failed to load gallery');
        });
    }

    function loadAlbumGallery(append) {
        API.get('/api/v1/gallery/albums/' + activeAlbumId + '/images').then(function(paths) {
            loading = false;
            if (!paths || paths.length === 0) {
                cleanupObjectURLs();
                items = [];
                total = 0;
                hasMore = false;
                renderGalleryItems(false);
                updateStatus();
                return;
            }
            // Search for these specific images
            var url = '/api/v1/gallery/search?limit=200&offset=0';
            API.get(url).then(function(data) {
                loading = false;
                var allItems = data.items || [];
                // Filter to only album images
                var pathSet = {};
                for (var i = 0; i < paths.length; i++) pathSet[paths[i]] = true;
                var filtered = [];
                for (var j = 0; j < allItems.length; j++) {
                    if (pathSet[allItems[j].file_path]) filtered.push(allItems[j]);
                }
                cleanupObjectURLs();
                items = filtered;
                total = filtered.length;
                hasMore = false;
                renderGalleryItems(false);
                updateStatus();
                updateAlbumBreadcrumb();
            });
        }).catch(function() {
            loading = false;
            Toast.error('Failed to load album images');
        });
    }

    function updateAlbumBreadcrumb() {
        var bc = document.getElementById('gallery-album-breadcrumb');
        if (!bc) return;
        if (activeAlbumId) {
            bc.classList.remove('hidden');
            bc.innerHTML =
                '<a id="album-bc-back" class="gallery-album-bc-link">Albums</a>' +
                '<span class="gallery-album-bc-sep">&rsaquo;</span>' +
                '<span class="gallery-album-bc-current">' + esc(activeAlbumName) + '</span>' +
                '<button class="btn btn-sm btn-outline gallery-album-bc-exit" id="album-bc-exit">Exit Album</button>';
            document.getElementById('album-bc-back').addEventListener('click', exitAlbumView);
            document.getElementById('album-bc-exit').addEventListener('click', exitAlbumView);
        } else {
            bc.classList.add('hidden');
            bc.innerHTML = '';
        }
    }

    function enterAlbumView(albumId, albumName) {
        activeAlbumId = albumId;
        activeAlbumName = albumName;
        offset = 0;
        // Switch to images page
        currentPage = 'images';
        var tabs = app.querySelectorAll('.gallery-page-tab');
        for (var j = 0; j < tabs.length; j++) {
            tabs[j].classList.remove('active');
            if (tabs[j].getAttribute('data-page') === 'images') tabs[j].classList.add('active');
        }
        // Show toolbar controls
        app.querySelector('.gallery-view-toggles').style.display = '';
        document.getElementById('gallery-sort').style.display = '';
        document.getElementById('gallery-search-input').style.display = '';
        document.getElementById('gallery-sentinel').style.display = '';
        var status = document.getElementById('gallery-status');
        if (status) status.style.display = '';

        updateAlbumBreadcrumb();
        loadGallery(false);
    }

    function exitAlbumView() {
        activeAlbumId = null;
        activeAlbumName = '';
        offset = 0;
        updateAlbumBreadcrumb();
        loadGallery(false);
    }

    function enterTagView(tag) {
        filterTags = [tag];
        activeAlbumId = null;
        activeAlbumName = '';
        offset = 0;
        // Switch to images page
        currentPage = 'images';
        var tabs = app.querySelectorAll('.gallery-page-tab');
        for (var j = 0; j < tabs.length; j++) {
            tabs[j].classList.remove('active');
            if (tabs[j].getAttribute('data-page') === 'images') tabs[j].classList.add('active');
        }
        // Show toolbar controls
        app.querySelector('.gallery-view-toggles').style.display = '';
        document.getElementById('gallery-sort').style.display = '';
        document.getElementById('gallery-search-input').style.display = '';
        document.getElementById('gallery-sentinel').style.display = '';
        var status = document.getElementById('gallery-status');
        if (status) status.style.display = '';

        updateAlbumBreadcrumb();
        loadGallery(false);
    }

    function updateStatus() {
        var el = document.getElementById('gallery-status');
        if (el) {
            var tagInfo = '';
            if (filterTags.length > 0) {
                tagInfo = ' &mdash; filtered by: ';
                for (var t = 0; t < filterTags.length; t++) {
                    tagInfo += '<span class="gallery-filter-chip">' + esc(filterTags[t]) +
                        '<button class="gallery-filter-chip-x" data-tag="' + esc(filterTags[t]) + '">&times;</button></span> ';
                }
            }
            el.innerHTML = '<div class="gallery-status-bar">' +
                '<span>' + esc(String(total)) + ' image' + (total !== 1 ? 's' : '') + tagInfo + '</span>' +
            '</div>';

            // Wire chip remove
            var chipBtns = el.querySelectorAll('.gallery-filter-chip-x');
            for (var ci = 0; ci < chipBtns.length; ci++) {
                (function(btn) {
                    btn.addEventListener('click', function() {
                        var tag = btn.getAttribute('data-tag');
                        var idx = filterTags.indexOf(tag);
                        if (idx !== -1) filterTags.splice(idx, 1);
                        offset = 0;
                        loadGallery(false);
                    });
                })(chipBtns[ci]);
            }
        }
    }

    // ── Render Items ─────────────────────────────────────────────────────────

    function renderGalleryItems(append) {
        var container = document.getElementById('gallery-container');
        if (!container) return;

        if (viewMode === 'grid' || viewMode === 'grid-ext') {
            renderGridItems(container, append);
        } else {
            renderListItems(container, append);
        }
    }

    function renderGridItems(container, append) {
        if (!append) {
            container.innerHTML = '';
            container.className = 'gallery-grid gallery-grid-' + viewMode;
        }

        var startIdx = append ? (items.length - (offset - (offset - limit))) : 0;
        if (append) {
            // Only render newly appended items
            var prevCount = container.querySelectorAll('.gallery-grid-item').length;
            startIdx = prevCount;
        } else {
            startIdx = 0;
        }

        for (var i = startIdx; i < items.length; i++) {
            var item = items[i];
            var el = document.createElement('div');
            el.className = 'gallery-grid-item';
            el.setAttribute('data-path', item.file_path);
            el.setAttribute('data-index', String(i));

            var thumbWrap = document.createElement('div');
            thumbWrap.className = 'gallery-grid-thumb';

            var img = document.createElement('img');
            img.alt = esc(item.file_name);
            img.setAttribute('loading', 'lazy');
            thumbWrap.appendChild(img);
            el.appendChild(thumbWrap);

            if (viewMode === 'grid-ext') {
                var caption = document.createElement('div');
                caption.className = 'gallery-grid-caption';

                var nameDiv = document.createElement('div');
                nameDiv.className = 'gallery-grid-name';
                nameDiv.textContent = item.file_name;
                caption.appendChild(nameDiv);

                var dateDiv = document.createElement('div');
                dateDiv.className = 'gallery-grid-date';
                dateDiv.textContent = formatGalleryDate(item.date_taken || item.mod_time);
                if (item.camera_make || item.camera_model) {
                    var camSpan = document.createElement('span');
                    camSpan.className = 'gallery-grid-camera';
                    camSpan.textContent = ' - ' + [item.camera_make, item.camera_model].filter(Boolean).join(' ');
                    dateDiv.appendChild(camSpan);
                }
                caption.appendChild(dateDiv);
                el.appendChild(caption);
            }

            container.appendChild(el);

            // Load thumbnail
            if (item.has_thumbnail) {
                loadThumb(img, item.file_path);
            } else {
                img.alt = item.file_name;
                img.className = 'gallery-no-thumb';
            }

            // Click handler
            (function(idx) {
                el.addEventListener('click', function() {
                    openLightbox(idx);
                });
            })(i);
        }

        if (items.length === 0 && !append) {
            container.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">No images found</p>';
        }
    }

    function renderListItems(container, append) {
        var isThumbMode = viewMode === 'list-thumb';
        var thumbSize = isThumbMode ? 100 : 48;
        var thumbHeight = isThumbMode ? 75 : 48;

        if (!append) {
            container.className = 'gallery-list';
            container.innerHTML =
                '<table class="responsive-table gallery-table"><thead><tr>' +
                    '<th class="gallery-thumb-col">Thumb</th>' +
                    '<th>Name</th>' +
                    '<th>Date</th>' +
                    '<th>Size</th>' +
                    '<th>Camera</th>' +
                '</tr></thead><tbody id="gallery-tbody"></tbody></table>';
        }

        var tbody = document.getElementById('gallery-tbody');
        if (!tbody) return;

        var startIdx = append ? tbody.querySelectorAll('tr').length : 0;

        for (var i = startIdx; i < items.length; i++) {
            var item = items[i];
            var tr = document.createElement('tr');
            tr.className = 'gallery-list-row';
            tr.setAttribute('data-path', item.file_path);
            tr.setAttribute('data-index', String(i));

            // Thumbnail cell
            var tdThumb = document.createElement('td');
            tdThumb.className = 'gallery-thumb-col';
            tdThumb.setAttribute('data-label', 'Thumb');
            var img = document.createElement('img');
            img.className = 'gallery-list-thumb';
            img.style.width = thumbSize + 'px';
            img.style.height = thumbHeight + 'px';
            img.alt = '';
            img.setAttribute('loading', 'lazy');
            tdThumb.appendChild(img);
            tr.appendChild(tdThumb);

            // Name cell
            var tdName = document.createElement('td');
            tdName.setAttribute('data-label', 'Name');
            tdName.textContent = item.file_name;
            tr.appendChild(tdName);

            // Date cell
            var tdDate = document.createElement('td');
            tdDate.setAttribute('data-label', 'Date');
            tdDate.textContent = formatGalleryDate(item.date_taken || item.mod_time);
            tr.appendChild(tdDate);

            // Size cell
            var tdSize = document.createElement('td');
            tdSize.setAttribute('data-label', 'Size');
            tdSize.textContent = formatBytes(item.size);
            tr.appendChild(tdSize);

            // Camera cell
            var tdCamera = document.createElement('td');
            tdCamera.setAttribute('data-label', 'Camera');
            tdCamera.textContent = [item.camera_make, item.camera_model].filter(Boolean).join(' ') || '-';
            tr.appendChild(tdCamera);

            tbody.appendChild(tr);

            // Load thumbnail
            if (item.has_thumbnail) {
                loadThumb(img, item.file_path);
            }

            // Click handler
            (function(idx) {
                tr.addEventListener('click', function() {
                    openLightbox(idx);
                });
            })(i);
        }

        if (items.length === 0 && !append) {
            container.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">No images found</p>';
        }
    }

    // ── View Mode Toggle ─────────────────────────────────────────────────────

    var viewBtns = app.querySelectorAll('.gallery-view-btn');
    for (var vi = 0; vi < viewBtns.length; vi++) {
        (function(btn) {
            btn.addEventListener('click', function() {
                var mode = btn.getAttribute('data-mode');
                if (mode === viewMode) return;
                viewMode = mode;
                localStorage.setItem('gallery-view-mode', mode);

                // Update active class
                var allBtns = app.querySelectorAll('.gallery-view-btn');
                for (var j = 0; j < allBtns.length; j++) {
                    allBtns[j].classList.remove('active');
                }
                btn.classList.add('active');

                // Re-render items
                cleanupObjectURLs();
                renderGalleryItems(false);
            });
        })(viewBtns[vi]);
    }

    // ── Sort ─────────────────────────────────────────────────────────────────

    var sortSelect = document.getElementById('gallery-sort');
    sortSelect.addEventListener('change', function() {
        sortBy = sortSelect.value;
        offset = 0;
        loadGallery(false);
    });

    // ── Search Input ─────────────────────────────────────────────────────────

    var searchInput = document.getElementById('gallery-search-input');
    var searchTimer = null;
    searchInput.addEventListener('input', function() {
        clearTimeout(searchTimer);
        searchTimer = setTimeout(function() {
            filterQuery = searchInput.value.trim();
            offset = 0;
            loadGallery(false);
        }, 350);
    });
    searchInput.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
            clearTimeout(searchTimer);
            filterQuery = searchInput.value.trim();
            offset = 0;
            loadGallery(false);
        }
    });

    // ── Albums Page ──────────────────────────────────────────────────────────

    function renderAlbumsPage() {
        var container = document.getElementById('gallery-container');
        if (!container) return;
        cleanupObjectURLs();

        container.className = '';
        container.innerHTML =
            '<div class="gallery-albums-tabs">' +
                '<button class="gallery-albums-tab' + (albumTab === 'my-albums' ? ' active' : '') + '" data-tab="my-albums">My Albums</button>' +
                '<button class="gallery-albums-tab' + (albumTab === 'date' ? ' active' : '') + '" data-tab="date">Date</button>' +
                '<button class="gallery-albums-tab' + (albumTab === 'location' ? ' active' : '') + '" data-tab="location">Location</button>' +
                '<button class="gallery-albums-tab' + (albumTab === 'camera' ? ' active' : '') + '" data-tab="camera">Camera</button>' +
            '</div>' +
            '<div id="gallery-albums-content"></div>';

        // Wire tab buttons
        var tabBtns = container.querySelectorAll('.gallery-albums-tab');
        for (var ti = 0; ti < tabBtns.length; ti++) {
            (function(btn) {
                btn.addEventListener('click', function() {
                    albumTab = btn.getAttribute('data-tab');
                    renderAlbumsPage();
                });
            })(tabBtns[ti]);
        }

        loadAlbumTab(albumTab);
    }

    function loadAlbumTab(tab) {
        var content = document.getElementById('gallery-albums-content');
        if (!content) return;

        if (tab === 'my-albums') {
            loadMyAlbumsTab(content);
            return;
        }

        if (albumData[tab]) {
            renderAlbumContent(tab, albumData[tab], content);
            return;
        }

        content.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">Loading...</p>';

        var endpoint = '/api/v1/gallery/albums/' + tab;
        API.get(endpoint).then(function(data) {
            albumData[tab] = data;
            renderAlbumContent(tab, data, content);
        }).catch(function() {
            content.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">Failed to load albums</p>';
        });
    }

    // ── My Albums Tab ────────────────────────────────────────────────────────

    function loadMyAlbumsTab(content) {
        content.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">Loading...</p>';

        API.get('/api/v1/gallery/albums').then(function(albums) {
            userAlbums = albums || [];
            renderMyAlbums(content);
        }).catch(function() {
            content.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">Failed to load albums</p>';
        });
    }

    function renderMyAlbums(content) {
        var html = '<div class="gallery-myalbums-header">' +
            '<button class="btn btn-sm" id="btn-new-album">New Album</button>' +
        '</div>';

        if (!userAlbums || userAlbums.length === 0) {
            html += '<p class="gallery-album-empty">No albums yet. Create one!</p>';
        } else {
            html += '<div class="gallery-album-cards">';
            for (var i = 0; i < userAlbums.length; i++) {
                var album = userAlbums[i];
                html += '<div class="gallery-album-card" data-album-id="' + album.id + '" data-album-name="' + esc(album.name) + '">' +
                    '<div class="gallery-album-card-thumb">';
                if (album.cover_path) {
                    html += '<img class="gallery-album-card-img" data-cover-path="' + esc(album.cover_path) + '" alt="">';
                } else {
                    html += '<div class="gallery-album-card-placeholder">&#128247;</div>';
                }
                html += '</div>' +
                    '<div class="gallery-album-card-info">' +
                        '<div class="gallery-album-card-name">' + esc(album.name) + '</div>' +
                        '<div class="gallery-album-card-meta">' + album.image_count + ' image' + (album.image_count !== 1 ? 's' : '') + '</div>' +
                    '</div>' +
                    '<div class="gallery-album-card-actions">' +
                        '<button class="btn btn-xs btn-outline" data-action="edit-album" data-id="' + album.id + '" title="Edit">&#9998;</button>' +
                        '<button class="btn btn-xs btn-danger" data-action="delete-album" data-id="' + album.id + '" title="Delete">&times;</button>' +
                    '</div>' +
                '</div>';
            }
            html += '</div>';
        }

        content.innerHTML = html;

        // Load cover thumbnails
        var coverImgs = content.querySelectorAll('.gallery-album-card-img');
        for (var ci = 0; ci < coverImgs.length; ci++) {
            var coverPath = coverImgs[ci].getAttribute('data-cover-path');
            if (coverPath) loadThumb(coverImgs[ci], coverPath);
        }

        // Wire new album button
        var newBtn = document.getElementById('btn-new-album');
        if (newBtn) {
            newBtn.addEventListener('click', function() {
                showAlbumModal(null);
            });
        }

        // Wire card clicks (enter album view)
        var cards = content.querySelectorAll('.gallery-album-card');
        for (var cc = 0; cc < cards.length; cc++) {
            (function(card) {
                card.addEventListener('click', function(e) {
                    // Ignore if clicking an action button
                    if (e.target.closest('[data-action]')) return;
                    var albumId = parseInt(card.getAttribute('data-album-id'), 10);
                    var albumName = card.getAttribute('data-album-name');
                    enterAlbumView(albumId, albumName);
                });
            })(cards[cc]);
        }

        // Wire edit/delete action buttons
        var actionBtns = content.querySelectorAll('[data-action]');
        for (var ab = 0; ab < actionBtns.length; ab++) {
            (function(btn) {
                btn.addEventListener('click', function(e) {
                    e.stopPropagation();
                    var action = btn.getAttribute('data-action');
                    var id = parseInt(btn.getAttribute('data-id'), 10);
                    if (action === 'edit-album') {
                        var album = null;
                        for (var k = 0; k < userAlbums.length; k++) {
                            if (userAlbums[k].id === id) { album = userAlbums[k]; break; }
                        }
                        showAlbumModal(album);
                    } else if (action === 'delete-album') {
                        if (!confirm('Delete this album? Images will not be deleted.')) return;
                        API.del('/api/v1/gallery/albums/' + id).then(function() {
                            Toast.success('Album deleted');
                            userAlbums = null;
                            loadMyAlbumsTab(content);
                        }).catch(function() { Toast.error('Failed to delete album'); });
                    }
                });
            })(actionBtns[ab]);
        }
    }

    function showAlbumModal(existing) {
        var isEdit = !!existing;
        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
            '<form id="album-form">' +
                '<div class="form-group">' +
                    '<label for="album-name">Name</label>' +
                    '<input type="text" id="album-name" value="' + (isEdit ? esc(existing.name) : '') + '" required>' +
                '</div>' +
                '<div class="form-group">' +
                    '<label for="album-desc">Description</label>' +
                    '<textarea id="album-desc" rows="3">' + (isEdit ? esc(existing.description || '') : '') + '</textarea>' +
                '</div>' +
                '<button type="submit" class="btn">' + (isEdit ? 'Update' : 'Create') + '</button>' +
            '</form>';

        Modal.open({
            title: isEdit ? 'Edit Album' : 'New Album',
            content: contentDiv
        });

        document.getElementById('album-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var name = document.getElementById('album-name').value.trim();
            if (!name) { Toast.error('Name is required'); return; }
            var desc = document.getElementById('album-desc').value.trim();
            var body = { name: name, description: desc };
            var promise;
            if (isEdit) {
                promise = API.put('/api/v1/gallery/albums/' + existing.id, body);
            } else {
                promise = API.post('/api/v1/gallery/albums', body);
            }
            promise.then(function() {
                Toast.success(isEdit ? 'Album updated' : 'Album created');
                Modal.close();
                userAlbums = null;
                var content = document.getElementById('gallery-albums-content');
                if (content) loadMyAlbumsTab(content);
            }).catch(function() { Toast.error('Operation failed'); });
        });
    }

    // ── Auto Albums Content (Date, Location, Camera) ────────────────────────

    function renderAlbumContent(tab, data, content) {
        if (tab === 'date') {
            renderDateAlbumCards(data, content);
        } else if (tab === 'location') {
            renderLocationAlbumCards(data, content);
        } else if (tab === 'camera') {
            renderCameraAlbumCards(data, content);
        }
    }

    function renderDateAlbumCards(data, content) {
        if (!data || data.length === 0) {
            content.innerHTML = '<p class="gallery-album-empty">No date albums</p>';
            return;
        }

        var html = '<div class="gallery-album-cards">';
        for (var y = 0; y < data.length; y++) {
            var year = data[y];
            if (!year.months) continue;
            for (var m = 0; m < year.months.length; m++) {
                var mo = year.months[m];
                var monthName = getMonthName(mo.month);
                html += '<div class="gallery-album-card gallery-auto-album-card" data-filter-type="date" data-year="' + year.year + '" data-month="' + mo.month + '">' +
                    '<div class="gallery-album-card-thumb">' +
                        '<div class="gallery-album-card-placeholder">&#128197;</div>' +
                    '</div>' +
                    '<div class="gallery-album-card-info">' +
                        '<div class="gallery-album-card-name">' + esc(monthName + ' ' + year.year) + '</div>' +
                        '<div class="gallery-album-card-meta">' + mo.count + ' image' + (mo.count !== 1 ? 's' : '') + '</div>' +
                    '</div>' +
                '</div>';
            }
        }
        html += '</div>';
        content.innerHTML = html;

        wireAutoAlbumCards(content);
    }

    function renderLocationAlbumCards(data, content) {
        if (!data || data.length === 0) {
            content.innerHTML = '<p class="gallery-album-empty">No location albums</p>';
            return;
        }

        var html = '<div class="gallery-album-cards">';
        for (var c = 0; c < data.length; c++) {
            var country = data[c];
            if (!country.cities) continue;
            for (var ci = 0; ci < country.cities.length; ci++) {
                var city = country.cities[ci];
                var label = [city.city, country.country].filter(Boolean).join(', ') || 'Unknown';
                html += '<div class="gallery-album-card gallery-auto-album-card" data-filter-type="location" data-country="' + esc(country.country) + '" data-city="' + esc(city.city) + '">' +
                    '<div class="gallery-album-card-thumb">' +
                        '<div class="gallery-album-card-placeholder">&#127758;</div>' +
                    '</div>' +
                    '<div class="gallery-album-card-info">' +
                        '<div class="gallery-album-card-name">' + esc(label) + '</div>' +
                        '<div class="gallery-album-card-meta">' + city.count + ' image' + (city.count !== 1 ? 's' : '') + '</div>' +
                    '</div>' +
                '</div>';
            }
        }
        html += '</div>';
        content.innerHTML = html;

        wireAutoAlbumCards(content);
    }

    function renderCameraAlbumCards(data, content) {
        if (!data || data.length === 0) {
            content.innerHTML = '<p class="gallery-album-empty">No camera albums</p>';
            return;
        }

        var html = '<div class="gallery-album-cards">';
        for (var mk = 0; mk < data.length; mk++) {
            var make = data[mk];
            if (!make.models) continue;
            for (var mi = 0; mi < make.models.length; mi++) {
                var model = make.models[mi];
                html += '<div class="gallery-album-card gallery-auto-album-card" data-filter-type="camera" data-make="' + esc(make.make) + '" data-model="' + esc(model.model) + '">' +
                    '<div class="gallery-album-card-thumb">' +
                        '<div class="gallery-album-card-placeholder">&#128247;</div>' +
                    '</div>' +
                    '<div class="gallery-album-card-info">' +
                        '<div class="gallery-album-card-name">' + esc(model.model || 'Unknown') + '</div>' +
                        '<div class="gallery-album-card-meta">' + esc(make.make) + ' &middot; ' + model.count + ' image' + (model.count !== 1 ? 's' : '') + '</div>' +
                    '</div>' +
                '</div>';
            }
        }
        html += '</div>';
        content.innerHTML = html;

        wireAutoAlbumCards(content);
    }

    function wireAutoAlbumCards(content) {
        var cards = content.querySelectorAll('.gallery-auto-album-card');
        for (var i = 0; i < cards.length; i++) {
            (function(card) {
                card.addEventListener('click', function() {
                    var filterType = card.getAttribute('data-filter-type');

                    // Reset filters
                    filterQuery = '';
                    filterCameraMake = '';
                    filterCountry = '';
                    filterTags = [];
                    filterDateFrom = '';
                    filterDateTo = '';

                    if (filterType === 'date') {
                        var year = card.getAttribute('data-year');
                        var month = card.getAttribute('data-month');
                        var paddedMonth = month.length === 1 ? '0' + month : month;
                        var daysInMonth = new Date(parseInt(year, 10), parseInt(month, 10), 0).getDate();
                        filterDateFrom = year + '-' + paddedMonth + '-01';
                        filterDateTo = year + '-' + paddedMonth + '-' + (daysInMonth < 10 ? '0' : '') + daysInMonth;
                    } else if (filterType === 'location') {
                        filterCountry = card.getAttribute('data-country') || '';
                    } else if (filterType === 'camera') {
                        filterCameraMake = card.getAttribute('data-make') || '';
                    }

                    offset = 0;

                    // Switch to images page
                    currentPage = 'images';
                    var tabs = app.querySelectorAll('.gallery-page-tab');
                    for (var j = 0; j < tabs.length; j++) {
                        tabs[j].classList.remove('active');
                        if (tabs[j].getAttribute('data-page') === 'images') tabs[j].classList.add('active');
                    }
                    app.querySelector('.gallery-view-toggles').style.display = '';
                    document.getElementById('gallery-sort').style.display = '';
                    document.getElementById('gallery-search-input').style.display = '';
                    document.getElementById('gallery-sentinel').style.display = '';
                    var status = document.getElementById('gallery-status');
                    if (status) status.style.display = '';

                    loadGallery(false);
                });
            })(cards[i]);
        }
    }

    // ── Map Page ─────────────────────────────────────────────────────────────

    function renderMapPage() {
        var container = document.getElementById('gallery-container');
        if (!container) return;
        cleanupObjectURLs();

        container.className = 'gallery-map-container';
        container.innerHTML = '<div id="photo-map"></div>' +
            '<div class="map-overlay map-loading"><div class="spinner"></div> Loading map data...</div>';

        API.get('/api/v1/gallery/map/points').then(function(points) {
            var overlay = container.querySelector('.map-loading');
            if (overlay) overlay.remove();

            if (!points || points.length === 0) {
                container.innerHTML = '<div id="photo-map"></div>' +
                    '<div class="map-overlay map-empty">' +
                    '<span class="map-empty-icon">&#127758;</span>' +
                    '<p>No geolocated photos found</p>' +
                    '<p class="map-empty-hint">Photos with GPS data in their EXIF metadata will appear here.</p>' +
                    '</div>';
                initGalleryMap(container.querySelector('#photo-map'), []);
                return;
            }

            initGalleryMap(container.querySelector('#photo-map'), points);
        }).catch(function() {
            var overlay = container.querySelector('.map-loading');
            if (overlay) {
                overlay.className = 'map-overlay map-error';
                overlay.innerHTML = 'Failed to load map data';
            }
        });
    }

    function initGalleryMap(mapEl, points) {
        if (!mapEl) return;

        var isDark = document.documentElement.getAttribute('data-theme') === 'dark';

        var map = L.map(mapEl, {
            zoomControl: true,
            attributionControl: true
        });

        var tileUrl = isDark
            ? 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
            : 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
        var tileAttr = isDark
            ? '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/">CARTO</a>'
            : '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>';

        L.tileLayer(tileUrl, {
            attribution: tileAttr,
            maxZoom: 19
        }).addTo(map);

        if (points.length === 0) {
            map.setView([20, 0], 2);
            return;
        }

        var cluster = L.markerClusterGroup({
            maxClusterRadius: 60,
            spiderfyOnMaxZoom: true,
            showCoverageOnHover: false,
            zoomToBoundsOnClick: true
        });

        var bounds = [];

        for (var i = 0; i < points.length; i++) {
            var p = points[i];
            var latlng = [p.latitude, p.longitude];
            bounds.push(latlng);

            var marker = L.marker(latlng);
            marker._photoData = p;
            marker.on('click', function(e) {
                var d = e.target._photoData;
                var thumbHtml = '';
                if (d.has_thumbnail) {
                    var thumbUrl = '/api/v1/gallery/thumb/' + API.encodeURIPath(d.file_path.replace(/^\//, ''));
                    thumbHtml = '<img class="map-popup-thumb" src="' + thumbUrl +
                        '?token=' + API.getToken() + '" alt="' + esc(d.file_name) + '" loading="lazy">';
                }

                var dateStr = '';
                if (d.date_taken) {
                    var dt = new Date(d.date_taken);
                    dateStr = '<div class="map-popup-date">' + dt.toLocaleDateString() + '</div>';
                }

                var html = '<div class="map-popup">' +
                    (thumbHtml ? '<div class="map-popup-img">' + thumbHtml + '</div>' : '') +
                    '<div class="map-popup-info">' +
                    '<div class="map-popup-name" title="' + esc(d.file_name) + '">' + esc(d.file_name) + '</div>' +
                    dateStr +
                    '<a class="map-popup-link" href="#" data-map-view-path="' + esc(d.file_path) + '">View image</a>' +
                    '</div></div>';

                e.target.bindPopup(html, { maxWidth: 280, className: 'map-popup-container' }).openPopup();
            });

            // When popup opens, wire the "View image" link
            marker.on('popupopen', function(e) {
                var link = e.popup.getElement().querySelector('[data-map-view-path]');
                if (link) {
                    link.addEventListener('click', function(ev) {
                        ev.preventDefault();
                        var filePath = link.getAttribute('data-map-view-path');
                        viewImageFromMap(filePath);
                    });
                }
            });
            cluster.addLayer(marker);
        }

        map.addLayer(cluster);
        map.fitBounds(bounds, { padding: [40, 40], maxZoom: 15 });
    }

    function viewImageFromMap(filePath) {
        // Search for this specific image, load it into items, then open lightbox
        var fileName = filePath.replace(/^.*\//, '');
        filterQuery = fileName;
        filterDateFrom = '';
        filterDateTo = '';
        filterTags = [];
        filterCameraMake = '';
        filterCountry = '';
        offset = 0;
        activeAlbumId = null;
        activeAlbumName = '';

        // Switch to images tab
        currentPage = 'images';
        var tabs = app.querySelectorAll('.gallery-page-tab');
        for (var j = 0; j < tabs.length; j++) {
            tabs[j].classList.remove('active');
            if (tabs[j].getAttribute('data-page') === 'images') tabs[j].classList.add('active');
        }
        app.querySelector('.gallery-view-toggles').style.display = '';
        document.getElementById('gallery-sort').style.display = '';
        var si = document.getElementById('gallery-search-input');
        si.style.display = '';
        si.value = fileName;
        document.getElementById('gallery-sentinel').style.display = '';
        var status = document.getElementById('gallery-status');
        if (status) status.style.display = '';

        // Fetch then open lightbox on the matching item
        var url = buildSearchURL(0);
        loading = true;
        API.get(url).then(function(data) {
            loading = false;
            cleanupObjectURLs();
            items = data.items || [];
            total = data.total || 0;
            offset = data.offset + data.limit;
            hasMore = !!data.has_more;
            renderGalleryItems(false);
            updateStatus();

            // Find matching item and open lightbox
            for (var k = 0; k < items.length; k++) {
                if (items[k].file_path === filePath) {
                    openLightbox(k);
                    return;
                }
            }
            // Fallback: open first result
            if (items.length > 0) openLightbox(0);
        }).catch(function() {
            loading = false;
        });
    }

    // ── Tags Page ────────────────────────────────────────────────────────────

    function renderTagsPage() {
        var container = document.getElementById('gallery-container');
        if (!container) return;
        cleanupObjectURLs();

        container.className = '';
        container.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">Loading tags...</p>';

        API.get('/api/v1/gallery/tags').then(function(tags) {
            allTags = tags || [];
            if (allTags.length === 0) {
                container.innerHTML = '<p class="gallery-album-empty">No tags yet</p>';
                return;
            }

            // For each tag, we need a cover image. Fetch first image per tag.
            var tagCards = [];
            var loaded = 0;
            var totalTags = allTags.length;

            for (var i = 0; i < totalTags; i++) {
                (function(tagObj, idx) {
                    // Search for one image with this tag to use as cover
                    API.get('/api/v1/gallery/search?tags=' + encodeURIComponent(tagObj.tag) + '&limit=1&offset=0').then(function(data) {
                        var coverPath = null;
                        if (data && data.items && data.items.length > 0 && data.items[0].has_thumbnail) {
                            coverPath = data.items[0].file_path;
                        }
                        tagCards[idx] = { tag: tagObj.tag, count: tagObj.count, coverPath: coverPath };
                        loaded++;
                        if (loaded === totalTags) {
                            renderTagCards(container, tagCards);
                        }
                    }).catch(function() {
                        tagCards[idx] = { tag: tagObj.tag, count: tagObj.count, coverPath: null };
                        loaded++;
                        if (loaded === totalTags) {
                            renderTagCards(container, tagCards);
                        }
                    });
                })(allTags[i], i);
            }
        }).catch(function() {
            container.innerHTML = '<p style="padding:1.5rem;color:var(--text-muted)">Failed to load tags</p>';
        });
    }

    function renderTagCards(container, tagCards) {
        var html = '<div class="gallery-album-cards">';
        for (var i = 0; i < tagCards.length; i++) {
            var tc = tagCards[i];
            html += '<div class="gallery-album-card gallery-tag-card" data-tag="' + esc(tc.tag) + '">' +
                '<div class="gallery-album-card-thumb">';
            if (tc.coverPath) {
                html += '<img class="gallery-album-card-img" data-cover-path="' + esc(tc.coverPath) + '" alt="">';
            } else {
                html += '<div class="gallery-album-card-placeholder">&#127991;</div>';
            }
            html += '</div>' +
                '<div class="gallery-album-card-info">' +
                    '<div class="gallery-album-card-name">' + esc(tc.tag) + '</div>' +
                    '<div class="gallery-album-card-meta">' + tc.count + ' image' + (tc.count !== 1 ? 's' : '') + '</div>' +
                '</div>' +
                '<div class="gallery-album-card-actions">' +
                    '<button class="btn btn-xs btn-outline" data-action="rename-tag" data-tag="' + esc(tc.tag) + '" title="Rename">&#9998;</button>' +
                    '<button class="btn btn-xs btn-danger" data-action="delete-tag" data-tag="' + esc(tc.tag) + '" title="Delete">&times;</button>' +
                '</div>' +
            '</div>';
        }
        html += '</div>';
        container.innerHTML = html;

        // Load cover thumbnails
        var coverImgs = container.querySelectorAll('.gallery-album-card-img');
        for (var ci = 0; ci < coverImgs.length; ci++) {
            var coverPath = coverImgs[ci].getAttribute('data-cover-path');
            if (coverPath) loadThumb(coverImgs[ci], coverPath);
        }

        // Wire tag card clicks (ignore if clicking action buttons)
        var cards = container.querySelectorAll('.gallery-tag-card');
        for (var cc = 0; cc < cards.length; cc++) {
            (function(card) {
                card.addEventListener('click', function(e) {
                    if (e.target.closest('[data-action]')) return;
                    var tag = card.getAttribute('data-tag');
                    enterTagView(tag);
                });
            })(cards[cc]);
        }

        // Wire rename/delete action buttons
        var actionBtns = container.querySelectorAll('[data-action]');
        for (var ab = 0; ab < actionBtns.length; ab++) {
            (function(btn) {
                btn.addEventListener('click', function(e) {
                    e.stopPropagation();
                    var action = btn.getAttribute('data-action');
                    var tagName = btn.getAttribute('data-tag');
                    if (action === 'rename-tag') {
                        showSettingsRenameModal(tagName, 'user');
                    } else if (action === 'delete-tag') {
                        settingsDeleteTag(tagName, 'user');
                    }
                });
            })(actionBtns[ab]);
        }
    }

    // ── Settings Page ─────────────────────────────────────────────────────────

    var isAdmin = sessionStorage.getItem('is_admin') === 'true';

    function renderSettingsPage() {
        var container = document.getElementById('gallery-container');
        if (!container) return;
        cleanupObjectURLs();

        container.className = '';
        var html = '<div class="gallery-settings-page">';

        // My Albums section — all users
        html += '<div class="settings-section">' +
            '<div class="settings-section-title"><span>My Albums</span>' +
                '<button class="btn btn-sm" id="settings-new-album">New Album</button>' +
            '</div>' +
            '<div class="settings-section-desc">Manage your custom photo albums.</div>' +
            '<div id="settings-albums-content">Loading...</div>' +
        '</div>';

        // My Tags section — all users
        html += '<div class="settings-section">' +
            '<div class="settings-section-title"><span>My Tags</span>' +
                '<button class="btn btn-sm" id="settings-add-tag">Add Tag</button>' +
            '</div>' +
            '<div class="settings-section-desc">Rename or delete your manual tags. Only affects tags you added on files you can access.</div>' +
            '<div id="settings-user-tags">Loading...</div>' +
        '</div>';

        // Global Tag Management — admin only
        if (isAdmin) {
            html += '<div class="settings-section">' +
                '<div class="settings-section-title">Global Tag Management</div>' +
                '<div class="settings-section-desc">Rename or delete tags globally across all images and all sources (admin only).</div>' +
                '<div id="settings-global-tags">Loading...</div>' +
            '</div>';
        }

        // Plugins — admin only
        if (isAdmin) {
            html += '<div class="settings-section">' +
                '<div class="settings-section-title">Auto-Tagging Plugins</div>' +
                '<div class="settings-section-desc">Manage external tagging plugins (admin only).</div>' +
                '<div id="settings-plugins-link">' +
                    '<button class="btn btn-sm btn-outline" id="settings-open-plugins">Manage Plugins</button>' +
                '</div>' +
            '</div>';
        }

        html += '</div>';
        container.innerHTML = html;

        // Load sections
        loadSettingsAlbums();
        loadSettingsUserTags();
        if (isAdmin) loadSettingsGlobalTags();

        // Wire new album button
        var newAlbumBtn = document.getElementById('settings-new-album');
        if (newAlbumBtn) {
            newAlbumBtn.addEventListener('click', function() {
                showAlbumModal(null);
                // After modal closes, refresh the album list
                var origClose = Modal.close;
                Modal.close = function() {
                    origClose();
                    Modal.close = origClose;
                    loadSettingsAlbums();
                };
            });
        }

        // Wire add tag button
        var addTagBtn = document.getElementById('settings-add-tag');
        if (addTagBtn) {
            addTagBtn.addEventListener('click', function() {
                showAddTagModal();
            });
        }

        // Wire plugins link (just switch to plugins page if it exists, else alert)
        var pluginsBtn = document.getElementById('settings-open-plugins');
        if (pluginsBtn) {
            pluginsBtn.addEventListener('click', function() {
                window.location.hash = '#gallery-plugins';
            });
        }
    }

    // ── Settings: My Albums ──────────────────────────────────────────────────

    function loadSettingsAlbums() {
        var el = document.getElementById('settings-albums-content');
        if (!el) return;

        API.get('/api/v1/gallery/albums').then(function(albums) {
            renderSettingsAlbumsTable(el, albums || []);
        }).catch(function() {
            el.innerHTML = '<p style="color:var(--text-muted)">Failed to load albums</p>';
        });
    }

    function renderSettingsAlbumsTable(el, albums) {
        if (!albums || albums.length === 0) {
            el.innerHTML = '<p style="color:var(--text-muted)">No albums yet. Click "New Album" to create one.</p>';
            return;
        }

        var html = '<div class="table-container"><table class="data-table">' +
            '<thead><tr>' +
                '<th>Name</th>' +
                '<th>Description</th>' +
                '<th>Images</th>' +
                '<th>Created</th>' +
                '<th>Actions</th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < albums.length; i++) {
            var a = albums[i];
            var desc = a.description || '';
            if (desc.length > 60) desc = desc.substring(0, 60) + '...';
            var created = a.created_at ? formatGalleryDate(a.created_at) : '-';
            html += '<tr>' +
                '<td>' + esc(a.name) + '</td>' +
                '<td>' + esc(desc) + '</td>' +
                '<td>' + esc(String(a.image_count)) + '</td>' +
                '<td>' + esc(created) + '</td>' +
                '<td>' +
                    '<div class="btn-group">' +
                        '<button class="btn btn-sm btn-outline" data-action="edit-settings-album" data-id="' + a.id + '">Edit</button>' +
                        '<button class="btn btn-sm btn-danger" data-action="delete-settings-album" data-id="' + a.id + '">Delete</button>' +
                    '</div>' +
                '</td>' +
            '</tr>';
        }

        html += '</tbody></table></div>';
        el.innerHTML = html;

        // Wire actions
        var btns = el.querySelectorAll('[data-action]');
        for (var b = 0; b < btns.length; b++) {
            (function(btn) {
                btn.addEventListener('click', function() {
                    var action = btn.getAttribute('data-action');
                    var id = parseInt(btn.getAttribute('data-id'), 10);
                    if (action === 'edit-settings-album') {
                        var album = null;
                        for (var k = 0; k < albums.length; k++) {
                            if (albums[k].id === id) { album = albums[k]; break; }
                        }
                        showAlbumModal(album);
                        var origClose = Modal.close;
                        Modal.close = function() {
                            origClose();
                            Modal.close = origClose;
                            loadSettingsAlbums();
                        };
                    } else if (action === 'delete-settings-album') {
                        if (!confirm('Delete this album? Images will not be deleted.')) return;
                        API.del('/api/v1/gallery/albums/' + id).then(function() {
                            Toast.success('Album deleted');
                            loadSettingsAlbums();
                        }).catch(function() { Toast.error('Failed to delete album'); });
                    }
                });
            })(btns[b]);
        }
    }

    // ── Settings: My Tags ────────────────────────────────────────────────────

    function loadSettingsUserTags() {
        var el = document.getElementById('settings-user-tags');
        if (!el) return;

        API.get('/api/v1/gallery/tags').then(function(tags) {
            renderSettingsTagTable(el, tags || [], 'user');
        }).catch(function() {
            el.innerHTML = '<p style="color:var(--text-muted)">Failed to load tags</p>';
        });
    }

    // ── Settings: Global Tags (admin) ────────────────────────────────────────

    function loadSettingsGlobalTags() {
        var el = document.getElementById('settings-global-tags');
        if (!el) return;

        API.get('/api/v1/gallery/tags').then(function(tags) {
            renderSettingsTagTable(el, tags || [], 'global');
        }).catch(function() {
            el.innerHTML = '<p style="color:var(--text-muted)">Failed to load tags</p>';
        });
    }

    // ── Settings: Add Tag Modal ─────────────────────────────────────────────

    function showAddTagModal() {
        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
            '<form id="add-tag-form">' +
                '<div class="form-group">' +
                    '<label for="add-tag-name">Tag Name</label>' +
                    '<input type="text" id="add-tag-name" required placeholder="e.g. vacation, portrait...">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Select Images</label>' +
                    '<input type="text" id="add-tag-img-search" placeholder="Search images...">' +
                    '<div id="add-tag-img-list" class="add-tag-img-list">Loading...</div>' +
                '</div>' +
                '<button type="submit" class="btn">Add Tag</button>' +
            '</form>';

        Modal.open({
            title: 'Add Tag to Images',
            content: contentDiv
        });

        var selectedPaths = {};
        var allImages = [];

        // Load gallery images
        API.get('/api/v1/gallery/search?limit=200&offset=0&sort_by=date&sort_order=desc').then(function(data) {
            allImages = (data && data.items) ? data.items : [];
            renderAddTagImageList(allImages, selectedPaths);
        }).catch(function() {
            var list = document.getElementById('add-tag-img-list');
            if (list) list.innerHTML = '<p style="color:var(--text-muted)">Failed to load images</p>';
        });

        // Search filter
        var searchInput = document.getElementById('add-tag-img-search');
        var searchTimer = null;
        if (searchInput) {
            searchInput.addEventListener('input', function() {
                clearTimeout(searchTimer);
                searchTimer = setTimeout(function() {
                    var q = searchInput.value.trim().toLowerCase();
                    if (!q) {
                        renderAddTagImageList(allImages, selectedPaths);
                        return;
                    }
                    var filtered = [];
                    for (var i = 0; i < allImages.length; i++) {
                        if (allImages[i].file_name.toLowerCase().indexOf(q) !== -1 ||
                            allImages[i].file_path.toLowerCase().indexOf(q) !== -1) {
                            filtered.push(allImages[i]);
                        }
                    }
                    renderAddTagImageList(filtered, selectedPaths);
                }, 250);
            });
        }

        // Submit
        document.getElementById('add-tag-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var tagName = document.getElementById('add-tag-name').value.trim();
            if (!tagName) { Toast.error('Tag name is required'); return; }

            var paths = [];
            for (var p in selectedPaths) {
                if (selectedPaths[p]) paths.push(p);
            }
            if (paths.length === 0) { Toast.error('Select at least one image'); return; }

            var done = 0;
            var errors = 0;
            for (var i = 0; i < paths.length; i++) {
                (function(filePath) {
                    API.post('/api/v1/gallery/tags/' + API.encodeURIPath(filePath.replace(/^\//, '')), { tag: tagName })
                        .then(function(resp) {
                            if (!resp.ok) errors++;
                            done++;
                            if (done === paths.length) {
                                if (errors > 0) {
                                    Toast.error(errors + ' image(s) failed');
                                } else {
                                    Toast.success('Tag "' + tagName + '" added to ' + paths.length + ' image(s)');
                                }
                                Modal.close();
                                loadSettingsUserTags();
                            }
                        })
                        .catch(function() {
                            errors++;
                            done++;
                            if (done === paths.length) {
                                Toast.error(errors + ' image(s) failed');
                                Modal.close();
                                loadSettingsUserTags();
                            }
                        });
                })(paths[i]);
            }
        });
    }

    function renderAddTagImageList(images, selectedPaths) {
        var list = document.getElementById('add-tag-img-list');
        if (!list) return;

        if (!images || images.length === 0) {
            list.innerHTML = '<p style="color:var(--text-muted)">No images found</p>';
            return;
        }

        var html = '';
        for (var i = 0; i < images.length; i++) {
            var img = images[i];
            var checked = selectedPaths[img.file_path] ? ' checked' : '';
            html += '<label class="add-tag-img-row">' +
                '<input type="checkbox" data-path="' + esc(img.file_path) + '"' + checked + '>' +
                '<span class="add-tag-img-name">' + esc(img.file_name) + '</span>' +
            '</label>';
        }
        list.innerHTML = html;

        // Wire checkboxes
        var boxes = list.querySelectorAll('input[type="checkbox"]');
        for (var b = 0; b < boxes.length; b++) {
            (function(box) {
                box.addEventListener('change', function() {
                    var path = box.getAttribute('data-path');
                    selectedPaths[path] = box.checked;
                });
            })(boxes[b]);
        }
    }

    // ── Shared tag table renderer ────────────────────────────────────────────

    function renderSettingsTagTable(el, tags, scope) {
        if (!tags || tags.length === 0) {
            el.innerHTML = '<p style="color:var(--text-muted)">No tags found.</p>';
            return;
        }

        var actionPrefix = scope === 'global' ? 'global-' : 'user-';

        var html = '<div class="table-container"><table class="data-table">' +
            '<thead><tr>' +
                '<th>Tag</th>' +
                '<th>Count</th>' +
                '<th>Actions</th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < tags.length; i++) {
            var tag = tags[i];
            html += '<tr>' +
                '<td>' + esc(tag.tag) + '</td>' +
                '<td>' + esc(String(tag.count)) + '</td>' +
                '<td>' +
                    '<div class="btn-group">' +
                        '<button class="btn btn-sm btn-outline" data-action="' + actionPrefix + 'rename-tag" data-tag="' + esc(tag.tag) + '">Rename</button>' +
                        '<button class="btn btn-sm btn-danger" data-action="' + actionPrefix + 'delete-tag" data-tag="' + esc(tag.tag) + '">Delete</button>' +
                    '</div>' +
                '</td>' +
            '</tr>';
        }

        html += '</tbody></table></div>';
        el.innerHTML = html;

        // Wire actions
        var btns = el.querySelectorAll('[data-action]');
        for (var b = 0; b < btns.length; b++) {
            (function(btn) {
                btn.addEventListener('click', function() {
                    var action = btn.getAttribute('data-action');
                    var tagName = btn.getAttribute('data-tag');
                    if (action === 'user-rename-tag') {
                        showSettingsRenameModal(tagName, 'user');
                    } else if (action === 'user-delete-tag') {
                        settingsDeleteTag(tagName, 'user');
                    } else if (action === 'global-rename-tag') {
                        showSettingsRenameModal(tagName, 'global');
                    } else if (action === 'global-delete-tag') {
                        settingsDeleteTag(tagName, 'global');
                    }
                });
            })(btns[b]);
        }
    }

    function showSettingsRenameModal(tag, scope) {
        var endpoint = scope === 'global'
            ? '/api/v1/admin/gallery/tags/' + encodeURIComponent(tag)
            : '/api/v1/gallery/user-tags/' + encodeURIComponent(tag);
        var scopeLabel = scope === 'global' ? 'globally across all images' : 'on your accessible files (manual tags only)';

        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
            '<form id="rename-tag-form">' +
                '<div class="form-group">' +
                    '<label>Current tag: <strong>' + esc(tag) + '</strong></label>' +
                    '<p class="settings-section-desc" style="margin-top:0.25rem">This will rename the tag ' + scopeLabel + '.</p>' +
                '</div>' +
                '<div class="form-group">' +
                    '<label for="new-tag-name">New Name</label>' +
                    '<input type="text" id="new-tag-name" required placeholder="New tag name">' +
                '</div>' +
                '<button type="submit" class="btn">Rename</button>' +
            '</form>';

        Modal.open({
            title: 'Rename Tag',
            content: contentDiv
        });

        document.getElementById('rename-tag-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var newTag = document.getElementById('new-tag-name').value.trim();
            if (!newTag) { Toast.error('New tag name is required'); return; }

            API.put(endpoint, { new_tag: newTag })
                .then(function(resp) {
                    if (resp.ok) {
                        return resp.json().then(function(data) {
                            Toast.success('Tag renamed (' + (data.affected || 0) + ' images updated)');
                            Modal.close();
                            if (scope === 'global') loadSettingsGlobalTags();
                            else loadSettingsUserTags();
                            if (currentPage === 'tags') renderTagsPage();
                        });
                    } else {
                        return resp.json().then(function(d) { Toast.error(d.error || 'Failed'); });
                    }
                })
                .catch(function() { Toast.error('Failed to rename tag'); });
        });
    }

    function settingsDeleteTag(tag, scope) {
        var endpoint = scope === 'global'
            ? '/api/v1/admin/gallery/tags/' + encodeURIComponent(tag)
            : '/api/v1/gallery/user-tags/' + encodeURIComponent(tag);
        var scopeLabel = scope === 'global'
            ? 'Delete tag "' + tag + '" from ALL images (all sources)? This cannot be undone.'
            : 'Delete tag "' + tag + '" from your accessible files (manual tags only)? This cannot be undone.';

        if (!confirm(scopeLabel)) return;

        API.del(endpoint)
            .then(function(resp) {
                if (resp.ok) {
                    return resp.json().then(function(data) {
                        Toast.success('Tag deleted (' + (data.affected || 0) + ' images affected)');
                        if (scope === 'global') loadSettingsGlobalTags();
                        else loadSettingsUserTags();
                        if (currentPage === 'tags') renderTagsPage();
                    });
                } else {
                    return resp.json().then(function(d) { Toast.error(d.error || 'Failed'); });
                }
            })
            .catch(function() { Toast.error('Failed to delete tag'); });
    }

    // ── Infinite Scroll ──────────────────────────────────────────────────────

    function setupInfiniteScroll() {
        var sentinel = document.getElementById('gallery-sentinel');
        if (!sentinel) return;

        if (observer) {
            observer.disconnect();
        }

        observer = new IntersectionObserver(function(entries) {
            for (var i = 0; i < entries.length; i++) {
                if (entries[i].isIntersecting && hasMore && !loading) {
                    loadGallery(true);
                }
            }
        }, { rootMargin: '200px' });

        observer.observe(sentinel);
    }

    // ── Gallery Share Modal ──────────────────────────────────────────────────

    function showGalleryShareModal(path) {
        var contentDiv = document.createElement('div');
        contentDiv.innerHTML =
            '<form id="gallery-share-form">' +
                '<div class="form-group">' +
                    '<label>Password (optional)</label>' +
                    '<input type="text" id="gallery-share-password" placeholder="Leave empty for no password">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Expires in (seconds, optional)</label>' +
                    '<input type="number" id="gallery-share-expiry" placeholder="e.g. 86400 for 1 day">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Max downloads (optional)</label>' +
                    '<input type="number" id="gallery-share-max-dl" placeholder="0 = unlimited">' +
                '</div>' +
                '<button type="submit" class="btn">Create Share Link</button>' +
            '</form>' +
            '<div id="gallery-share-result"></div>';

        Modal.open({ title: 'Share: ' + path.split('/').pop(), content: contentDiv });

        document.getElementById('gallery-share-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var body = {};
            var pw = document.getElementById('gallery-share-password').value;
            var exp = document.getElementById('gallery-share-expiry').value;
            var maxDl = document.getElementById('gallery-share-max-dl').value;
            if (pw) body.password = pw;
            if (exp) body.expires_in_sec = parseInt(exp, 10);
            if (maxDl) body.max_downloads = parseInt(maxDl, 10);

            API.post('/api/v1/share/' + API.encodeURIPath(path.replace(/^\//, '')), body)
                .then(function(resp) { return resp.json(); })
                .then(function(data) {
                    if (data.error) {
                        document.getElementById('gallery-share-result').innerHTML =
                            '<div class="alert alert-error">' + esc(data.error) + '</div>';
                        return;
                    }
                    var shareUrl = window.location.origin + '/app/#share/' + data.id;
                    if (pw) shareUrl += '/' + pw;
                    document.getElementById('gallery-share-result').innerHTML =
                        '<div class="alert alert-success">Share link created!</div>' +
                        '<div class="share-url">' +
                            '<input type="text" id="gallery-share-url-input" readonly value="' + esc(shareUrl) + '">' +
                            '<button class="btn btn-sm" id="gallery-btn-copy-url">Copy</button>' +
                        '</div>';
                    document.getElementById('gallery-btn-copy-url').addEventListener('click', function() {
                        var inp = document.getElementById('gallery-share-url-input');
                        inp.select();
                        document.execCommand('copy');
                        Toast.success('Copied to clipboard');
                    });
                });
        });
    }

    // ── Lightbox ─────────────────────────────────────────────────────────────

    function openLightbox(index) {
        currentIndex = index;
        if (currentIndex < 0 || currentIndex >= items.length) return;

        var item = items[currentIndex];

        // Create overlay
        var overlay = document.createElement('div');
        overlay.className = 'lightbox-overlay';
        overlay.id = 'lightbox-overlay';

        overlay.innerHTML =
            '<div class="lightbox-content">' +
                '<button class="lightbox-close">&times;</button>' +
                '<button class="lightbox-prev">&#10094;</button>' +
                '<div class="lightbox-image-wrap">' +
                    '<img class="lightbox-image" src="">' +
                '</div>' +
                '<button class="lightbox-next">&#10095;</button>' +
            '</div>' +
            '<div class="lightbox-sidebar hidden" id="lightbox-sidebar">' +
                '<div class="lightbox-sidebar-header">' +
                    '<h3>Details</h3>' +
                    '<button class="lightbox-sidebar-close">&times;</button>' +
                '</div>' +
                '<div class="lightbox-sidebar-body" id="lightbox-sidebar-body"></div>' +
            '</div>' +
            '<div class="lightbox-toolbar">' +
                '<button class="lightbox-btn" id="lb-info-btn" title="Info">&#9432;</button>' +
                '<button class="lightbox-btn" id="lb-download-btn" title="Download">&#11015;</button>' +
                '<button class="lightbox-btn" id="lb-share-btn" title="Share">&#128279;</button>' +
                (activeAlbumId ? '<button class="lightbox-btn" id="lb-set-cover-btn" title="Set as Album Cover">&#9733;</button>' : '') +
                '<span class="lightbox-counter" id="lb-counter"></span>' +
            '</div>';

        document.body.appendChild(overlay);

        // Load image
        loadLightboxImage(item);
        updateLightboxCounter();

        // Wire close
        overlay.querySelector('.lightbox-close').addEventListener('click', closeLightbox);

        // Wire navigation
        overlay.querySelector('.lightbox-prev').addEventListener('click', function() {
            lightboxNav(-1);
        });
        overlay.querySelector('.lightbox-next').addEventListener('click', function() {
            lightboxNav(1);
        });

        // Wire info toggle
        document.getElementById('lb-info-btn').addEventListener('click', function() {
            var sidebar = document.getElementById('lightbox-sidebar');
            if (sidebar.classList.contains('hidden')) {
                sidebar.classList.remove('hidden');
                loadLightboxMetadata(items[currentIndex]);
            } else {
                sidebar.classList.add('hidden');
            }
        });

        // Wire sidebar close
        overlay.querySelector('.lightbox-sidebar-close').addEventListener('click', function() {
            document.getElementById('lightbox-sidebar').classList.add('hidden');
        });

        // Wire set cover (album view only)
        var setCoverBtn = document.getElementById('lb-set-cover-btn');
        if (setCoverBtn && activeAlbumId) {
            setCoverBtn.addEventListener('click', function() {
                var curItem = items[currentIndex];
                API.put('/api/v1/gallery/albums/' + activeAlbumId + '/cover', { cover_path: curItem.file_path })
                    .then(function() {
                        Toast.success('Album cover set');
                    })
                    .catch(function() { Toast.error('Failed to set cover'); });
            });
        }

        // Wire download
        document.getElementById('lb-download-btn').addEventListener('click', function() {
            var curItem = items[currentIndex];
            var a = document.createElement('a');
            a.href = API.downloadUrl(curItem.file_path.replace(/^\//, ''));
            a.download = curItem.file_name;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        });

        // Wire share
        document.getElementById('lb-share-btn').addEventListener('click', function() {
            var curItem = items[currentIndex];
            showGalleryShareModal(curItem.file_path);
        });

        // Click outside to close
        overlay.addEventListener('click', function(e) {
            if (e.target === overlay) {
                closeLightbox();
            }
        });

        // Keyboard navigation
        document.addEventListener('keydown', lightboxKeyHandler);

        // Mobile swipe
        setupLightboxSwipe(overlay);
    }

    function loadLightboxImage(item) {
        var imgEl = document.querySelector('.lightbox-image');
        if (!imgEl) return;

        imgEl.src = '';
        imgEl.alt = 'Loading...';

        // Clean up previous lightbox URL
        if (lightboxObjectURL) {
            URL.revokeObjectURL(lightboxObjectURL);
            lightboxObjectURL = null;
        }

        var url = '/api/v1/content/' + API.encodeURIPath(item.file_path.replace(/^\//, ''));
        fetch(url, { headers: { 'Authorization': 'Bearer ' + API.getToken() } })
            .then(function(r) { return r.blob(); })
            .then(function(blob) {
                lightboxObjectURL = URL.createObjectURL(blob);
                imgEl.src = lightboxObjectURL;
                imgEl.alt = item.file_name;
            })
            .catch(function() {
                imgEl.alt = 'Failed to load image';
            });
    }

    function updateLightboxCounter() {
        var counter = document.getElementById('lb-counter');
        if (counter) {
            counter.textContent = (currentIndex + 1) + ' / ' + items.length;
        }
    }

    function lightboxNav(direction) {
        var newIndex = currentIndex + direction;
        if (newIndex < 0) newIndex = items.length - 1;
        if (newIndex >= items.length) newIndex = 0;
        currentIndex = newIndex;

        loadLightboxImage(items[currentIndex]);
        updateLightboxCounter();

        // Refresh sidebar if visible
        var sidebar = document.getElementById('lightbox-sidebar');
        if (sidebar && !sidebar.classList.contains('hidden')) {
            loadLightboxMetadata(items[currentIndex]);
        }
    }

    function lightboxKeyHandler(e) {
        if (e.key === 'Escape') {
            closeLightbox();
        } else if (e.key === 'ArrowLeft') {
            lightboxNav(-1);
        } else if (e.key === 'ArrowRight') {
            lightboxNav(1);
        }
    }

    function closeLightbox() {
        var overlay = document.getElementById('lightbox-overlay');
        if (overlay && overlay.parentNode) {
            overlay.parentNode.removeChild(overlay);
        }
        document.removeEventListener('keydown', lightboxKeyHandler);

        // Clean up lightbox object URL
        if (lightboxObjectURL) {
            URL.revokeObjectURL(lightboxObjectURL);
            lightboxObjectURL = null;
        }

        currentIndex = -1;
    }

    // ── Lightbox Swipe (Mobile) ──────────────────────────────────────────────

    function setupLightboxSwipe(overlay) {
        var touchStartX = 0;
        var touchStartY = 0;

        var imageWrap = overlay.querySelector('.lightbox-image-wrap');
        if (!imageWrap) return;

        imageWrap.addEventListener('touchstart', function(e) {
            touchStartX = e.touches[0].clientX;
            touchStartY = e.touches[0].clientY;
        }, { passive: true });

        imageWrap.addEventListener('touchend', function(e) {
            var dx = e.changedTouches[0].clientX - touchStartX;
            var dy = Math.abs(e.changedTouches[0].clientY - touchStartY);

            // Horizontal swipe with minimal vertical drift
            if (dy > 50) return;

            if (dx > 60) {
                lightboxNav(-1); // Swipe right = previous
            } else if (dx < -60) {
                lightboxNav(1);  // Swipe left = next
            }
        }, { passive: true });
    }

    // ── Lightbox Metadata / EXIF ─────────────────────────────────────────────

    function loadLightboxMetadata(item) {
        var body = document.getElementById('lightbox-sidebar-body');
        if (!body) return;

        body.innerHTML = '<p style="color:var(--text-muted)">Loading...</p>';

        var url = '/api/v1/gallery/metadata/' + API.encodeURIPath(item.file_path.replace(/^\//, ''));
        API.get(url).then(function(meta) {
            renderLightboxSidebar(meta, item);
        }).catch(function() {
            body.innerHTML = '<p style="color:var(--text-muted)">Failed to load metadata</p>';
        });
    }

    function renderLightboxSidebar(meta, item) {
        var body = document.getElementById('lightbox-sidebar-body');
        if (!body) return;

        var html = '';

        // File info
        html += '<div class="lightbox-section">';
        html += '<div class="lightbox-section-title">File</div>';
        html += '<div class="lightbox-meta-grid">';
        html += '<div class="lightbox-meta-key">Name</div><div class="lightbox-meta-val">' + esc(item.file_name) + '</div>';
        html += '<div class="lightbox-meta-key">Size</div><div class="lightbox-meta-val">' + formatBytes(item.size) + '</div>';
        if (meta.width && meta.height) {
            html += '<div class="lightbox-meta-key">Dimensions</div><div class="lightbox-meta-val">' + esc(String(meta.width)) + ' x ' + esc(String(meta.height)) + '</div>';
        }
        html += '<div class="lightbox-meta-key">Modified</div><div class="lightbox-meta-val">' + formatDate(item.mod_time) + '</div>';
        if (meta.date_taken) {
            html += '<div class="lightbox-meta-key">Date Taken</div><div class="lightbox-meta-val">' + formatDate(meta.date_taken) + '</div>';
        }
        html += '</div></div>';

        // Camera info
        if (meta.camera_make || meta.camera_model || meta.focal_length || meta.aperture || meta.shutter_speed || meta.iso) {
            html += '<div class="lightbox-section">';
            html += '<div class="lightbox-section-title">Camera</div>';
            html += '<div class="lightbox-meta-grid">';
            if (meta.camera_make) {
                html += '<div class="lightbox-meta-key">Make</div><div class="lightbox-meta-val">' + esc(meta.camera_make) + '</div>';
            }
            if (meta.camera_model) {
                html += '<div class="lightbox-meta-key">Model</div><div class="lightbox-meta-val">' + esc(meta.camera_model) + '</div>';
            }
            if (meta.focal_length) {
                html += '<div class="lightbox-meta-key">Focal Length</div><div class="lightbox-meta-val">' + esc(String(meta.focal_length)) + '</div>';
            }
            if (meta.aperture) {
                html += '<div class="lightbox-meta-key">Aperture</div><div class="lightbox-meta-val">' + esc(String(meta.aperture)) + '</div>';
            }
            if (meta.shutter_speed) {
                html += '<div class="lightbox-meta-key">Shutter Speed</div><div class="lightbox-meta-val">' + esc(String(meta.shutter_speed)) + '</div>';
            }
            if (meta.iso) {
                html += '<div class="lightbox-meta-key">ISO</div><div class="lightbox-meta-val">' + esc(String(meta.iso)) + '</div>';
            }
            html += '</div></div>';
        }

        // Location
        if (meta.latitude || meta.longitude || meta.location_city || meta.location_country) {
            html += '<div class="lightbox-section">';
            html += '<div class="lightbox-section-title">Location</div>';
            html += '<div class="lightbox-meta-grid">';
            if (meta.location_city) {
                html += '<div class="lightbox-meta-key">City</div><div class="lightbox-meta-val">' + esc(meta.location_city) + '</div>';
            }
            if (meta.location_country) {
                html += '<div class="lightbox-meta-key">Country</div><div class="lightbox-meta-val">' + esc(meta.location_country) + '</div>';
            }
            if (meta.latitude && meta.longitude) {
                html += '<div class="lightbox-meta-key">Coordinates</div><div class="lightbox-meta-val">' +
                    esc(String(meta.latitude.toFixed(6))) + ', ' + esc(String(meta.longitude.toFixed(6))) + '</div>';
            }
            html += '</div>';
            if (meta.latitude && meta.longitude) {
                html += '<div id="lightbox-minimap" class="lightbox-minimap"></div>';
            }
            html += '</div>';
        }

        // Tags
        html += '<div class="lightbox-section">';
        html += '<div class="lightbox-section-title">Tags</div>';
        html += '<div class="lightbox-tags" id="lb-tags-container">';
        html += renderLightboxTags(meta.tags || []);
        html += '</div>';
        html += '<div class="lightbox-tag-input">' +
            '<input type="text" id="lb-tag-input" placeholder="Add tag...">' +
        '</div>';
        html += '</div>';

        // Albums membership
        html += '<div class="lightbox-section">';
        html += '<div class="lightbox-section-title">Albums</div>';
        html += '<div class="lightbox-albums" id="lb-albums-container"><span class="lightbox-tags-empty">Loading...</span></div>';
        html += '</div>';

        body.innerHTML = html;

        // Initialize mini map if coordinates exist
        if (meta.latitude && meta.longitude) {
            initLightboxMinimap(meta.latitude, meta.longitude);
        }

        // Load album membership
        loadLightboxAlbums(item);

        // Wire tag add
        var tagInput = document.getElementById('lb-tag-input');
        if (tagInput) {
            tagInput.addEventListener('keydown', function(e) {
                if (e.key === 'Enter') {
                    e.preventDefault();
                    var val = tagInput.value.trim();
                    if (!val) return;
                    var filePath = item.file_path;
                    API.post('/api/v1/gallery/tags/' + API.encodeURIPath(filePath.replace(/^\//, '')), { tag: val })
                        .then(function(resp) {
                            if (resp.ok) {
                                tagInput.value = '';
                                Toast.success('Tag added');
                                // Refresh sidebar
                                loadLightboxMetadata(item);
                            } else {
                                resp.json().then(function(d) { Toast.error(d.error || 'Failed to add tag'); });
                            }
                        })
                        .catch(function() { Toast.error('Failed to add tag'); });
                }
            });
        }

        // Wire tag click and delete buttons
        wireTagClickHandlers();
        wireTagDeleteButtons(item);
    }

    function renderLightboxTags(tags) {
        if (!tags || tags.length === 0) {
            return '<span class="lightbox-tags-empty">No tags</span>';
        }
        var html = '';
        for (var i = 0; i < tags.length; i++) {
            var t = tags[i];
            var tagName = typeof t === 'string' ? t : t.tag;
            var source = (typeof t === 'object' && t.source) ? t.source : '';
            html += '<span class="lightbox-tag lightbox-tag-clickable" data-tag="' + esc(tagName) + '">' +
                '<span class="lightbox-tag-label">' + esc(tagName) + '</span>' +
                (source ? '<span class="lightbox-tag-source">' + esc(source) + '</span>' : '') +
                '<button class="lightbox-tag-x" data-tag="' + esc(tagName) + '">&times;</button>' +
            '</span>';
        }
        return html;
    }

    function wireTagClickHandlers() {
        var chips = document.querySelectorAll('.lightbox-tag-clickable .lightbox-tag-label');
        for (var i = 0; i < chips.length; i++) {
            (function(label) {
                label.addEventListener('click', function() {
                    var tag = label.parentElement.getAttribute('data-tag');
                    if (tag) {
                        closeLightbox();
                        enterTagView(tag);
                    }
                });
            })(chips[i]);
        }
    }

    function wireTagDeleteButtons(item) {
        var btns = document.querySelectorAll('.lightbox-tag-x');
        for (var i = 0; i < btns.length; i++) {
            (function(btn) {
                btn.addEventListener('click', function(e) {
                    e.stopPropagation();
                    var tag = btn.getAttribute('data-tag');
                    var filePath = item.file_path;
                    API.del('/api/v1/gallery/tags/' + API.encodeURIPath(filePath.replace(/^\//, '')) + '?tag=' + encodeURIComponent(tag))
                        .then(function(resp) {
                            if (resp.ok) {
                                Toast.success('Tag removed');
                                loadLightboxMetadata(item);
                            } else {
                                resp.json().then(function(d) { Toast.error(d.error || 'Failed to remove tag'); });
                            }
                        })
                        .catch(function() { Toast.error('Failed to remove tag'); });
                });
            })(btns[i]);
        }
    }

    // ── Lightbox Album Membership ───────────────────────────────────────────

    function loadLightboxAlbums(item) {
        var container = document.getElementById('lb-albums-container');
        if (!container) return;

        API.get('/api/v1/gallery/image-albums/' + API.encodeURIPath(item.file_path.replace(/^\//, ''))).then(function(data) {
            var albums = Array.isArray(data) ? data : [];
            renderLightboxAlbums(container, albums, item);
        }).catch(function() {
            container.innerHTML = '<span class="lightbox-tags-empty">Failed to load albums</span>';
        });
    }

    function renderLightboxAlbums(container, albums, item) {
        var html = '';
        for (var i = 0; i < albums.length; i++) {
            html += '<span class="lightbox-album-pill" data-album-id="' + albums[i].id + '" data-album-name="' + esc(albums[i].name) + '">' +
                '<span class="lightbox-album-label">' + esc(albums[i].name) + '</span>' +
                '<button class="lightbox-album-x" data-album-id="' + albums[i].id + '" title="Remove from album">&times;</button>' +
            '</span>';
        }
        html += '<div class="lightbox-album-add">' +
            '<select id="lb-album-select"><option value="">Add to album...</option></select>' +
        '</div>';

        container.innerHTML = html;

        // Populate album dropdown with user's albums
        var sel = document.getElementById('lb-album-select');
        API.get('/api/v1/gallery/albums').then(function(allAlbums) {
            if (!Array.isArray(allAlbums)) return;
            for (var j = 0; j < allAlbums.length; j++) {
                var opt = document.createElement('option');
                opt.value = String(allAlbums[j].id);
                opt.textContent = allAlbums[j].name;
                sel.appendChild(opt);
            }
        });

        // Wire add to album
        sel.addEventListener('change', function() {
            var albumId = parseInt(sel.value, 10);
            if (!albumId) return;
            API.post('/api/v1/gallery/albums/' + albumId + '/images', { file_path: item.file_path })
                .then(function() {
                    Toast.success('Added to album');
                    loadLightboxAlbums(item);
                })
                .catch(function() { Toast.error('Failed to add'); });
        });

        // Wire album pill clicks (enter album view)
        var albumPills = container.querySelectorAll('.lightbox-album-pill');
        for (var al = 0; al < albumPills.length; al++) {
            (function(pill) {
                pill.addEventListener('click', function(e) {
                    if (e.target.classList.contains('lightbox-album-x')) return;
                    var albumId = parseInt(pill.getAttribute('data-album-id'), 10);
                    var albumName = pill.getAttribute('data-album-name');
                    closeLightbox();
                    enterAlbumView(albumId, albumName);
                });
            })(albumPills[al]);
        }

        // Wire remove buttons
        var removeBtns = container.querySelectorAll('.lightbox-album-x');
        for (var rb = 0; rb < removeBtns.length; rb++) {
            (function(btn) {
                btn.addEventListener('click', function(e) {
                    e.stopPropagation();
                    var albumId = parseInt(btn.getAttribute('data-album-id'), 10);
                    API.del('/api/v1/gallery/albums/' + albumId + '/images', { file_path: item.file_path })
                        .then(function() {
                            Toast.success('Removed from album');
                            loadLightboxAlbums(item);
                        })
                        .catch(function() { Toast.error('Failed to remove'); });
                });
            })(removeBtns[rb]);
        }
    }

    function initLightboxMinimap(lat, lng) {
        var mapEl = document.getElementById('lightbox-minimap');
        if (!mapEl || typeof L === 'undefined') return;

        var isDark = document.documentElement.getAttribute('data-theme') === 'dark';
        var tileUrl = isDark
            ? 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
            : 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';

        var minimap = L.map(mapEl, {
            zoomControl: false,
            attributionControl: false,
            dragging: false,
            scrollWheelZoom: false,
            doubleClickZoom: false,
            touchZoom: false,
            boxZoom: false,
            keyboard: false
        }).setView([lat, lng], 13);

        L.tileLayer(tileUrl, { maxZoom: 19 }).addTo(minimap);
        L.marker([lat, lng]).addTo(minimap);
    }

    // ── Helpers ───────────────────────────────────────────────────────────────

    function formatGalleryDate(dateStr) {
        if (!dateStr) return '-';
        var d = new Date(dateStr);
        var months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
        return months[d.getMonth()] + ' ' + d.getDate() + ', ' + d.getFullYear();
    }

    function getMonthName(monthNum) {
        var months = ['', 'January', 'February', 'March', 'April', 'May', 'June',
            'July', 'August', 'September', 'October', 'November', 'December'];
        return months[parseInt(monthNum, 10)] || 'Month ' + monthNum;
    }

    // ── Init ─────────────────────────────────────────────────────────────────

    loadGallery(false);
    setupInfiniteScroll();
}
