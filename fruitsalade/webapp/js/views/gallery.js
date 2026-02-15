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

    // Panel state
    var filtersOpen = false;
    var albumsOpen = false;
    var albumsLoaded = false;
    var albumTab = 'date';
    var albumData = { date: null, location: null, camera: null };

    // ── Render Shell ─────────────────────────────────────────────────────────
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Gallery</h2>' +
            '<div class="toolbar-actions">' +
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
                '<button class="btn btn-outline btn-sm" id="gallery-filters-toggle">Filters</button>' +
                '<button class="btn btn-outline btn-sm" id="gallery-albums-toggle">Albums</button>' +
            '</div>' +
        '</div>' +
        '<div id="gallery-filters-panel" class="gallery-filters-panel hidden"></div>' +
        '<div id="gallery-albums-panel" class="gallery-albums-panel hidden"></div>' +
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

    function updateStatus() {
        var el = document.getElementById('gallery-status');
        if (el) {
            el.innerHTML = '<div class="gallery-status-bar">' +
                '<span>' + esc(String(total)) + ' image' + (total !== 1 ? 's' : '') + '</span>' +
            '</div>';
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

    // ── Filters Panel ────────────────────────────────────────────────────────

    var filtersPanel = document.getElementById('gallery-filters-panel');

    function renderFiltersPanel() {
        var tagsHtml = '';
        for (var t = 0; t < filterTags.length; t++) {
            tagsHtml += '<span class="gallery-filter-chip">' +
                esc(filterTags[t]) +
                '<button class="gallery-filter-chip-x" data-tag="' + esc(filterTags[t]) + '">&times;</button>' +
            '</span>';
        }

        filtersPanel.innerHTML =
            '<div class="gallery-filters-grid">' +
                '<div class="form-group">' +
                    '<label>Search</label>' +
                    '<input type="text" id="gf-query" placeholder="Filename..." value="' + esc(filterQuery) + '">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Date From</label>' +
                    '<input type="date" id="gf-date-from" value="' + esc(filterDateFrom) + '">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Date To</label>' +
                    '<input type="date" id="gf-date-to" value="' + esc(filterDateTo) + '">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Camera Make</label>' +
                    '<input type="text" id="gf-camera" placeholder="e.g. Canon" value="' + esc(filterCameraMake) + '">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Country</label>' +
                    '<input type="text" id="gf-country" placeholder="e.g. France" value="' + esc(filterCountry) + '">' +
                '</div>' +
                '<div class="form-group">' +
                    '<label>Tags</label>' +
                    '<input type="text" id="gf-tag-input" placeholder="Type and press Enter">' +
                    '<div id="gf-tags-chips" class="gallery-filter-chips">' + tagsHtml + '</div>' +
                '</div>' +
            '</div>' +
            '<div class="gallery-filters-actions">' +
                '<button class="btn btn-sm" id="gf-apply">Apply</button>' +
                '<button class="btn btn-sm btn-outline" id="gf-clear">Clear</button>' +
            '</div>';

        // Wire tag input
        var tagInput = document.getElementById('gf-tag-input');
        tagInput.addEventListener('keydown', function(e) {
            if (e.key === 'Enter') {
                e.preventDefault();
                var val = tagInput.value.trim();
                if (val && filterTags.indexOf(val) === -1) {
                    filterTags.push(val);
                    tagInput.value = '';
                    renderFiltersPanel();
                }
            }
        });

        // Wire tag chip remove buttons
        var chipXBtns = filtersPanel.querySelectorAll('.gallery-filter-chip-x');
        for (var cx = 0; cx < chipXBtns.length; cx++) {
            (function(btn) {
                btn.addEventListener('click', function() {
                    var tag = btn.getAttribute('data-tag');
                    var idx = filterTags.indexOf(tag);
                    if (idx !== -1) {
                        filterTags.splice(idx, 1);
                        renderFiltersPanel();
                    }
                });
            })(chipXBtns[cx]);
        }

        // Wire apply
        document.getElementById('gf-apply').addEventListener('click', function() {
            filterQuery = document.getElementById('gf-query').value.trim();
            filterDateFrom = document.getElementById('gf-date-from').value;
            filterDateTo = document.getElementById('gf-date-to').value;
            filterCameraMake = document.getElementById('gf-camera').value.trim();
            filterCountry = document.getElementById('gf-country').value.trim();
            offset = 0;
            loadGallery(false);
        });

        // Wire clear
        document.getElementById('gf-clear').addEventListener('click', function() {
            filterQuery = '';
            filterDateFrom = '';
            filterDateTo = '';
            filterTags = [];
            filterCameraMake = '';
            filterCountry = '';
            renderFiltersPanel();
            offset = 0;
            loadGallery(false);
        });
    }

    document.getElementById('gallery-filters-toggle').addEventListener('click', function() {
        filtersOpen = !filtersOpen;
        if (filtersOpen) {
            filtersPanel.classList.remove('hidden');
            renderFiltersPanel();
            // Close albums if open
            if (albumsOpen) {
                albumsOpen = false;
                document.getElementById('gallery-albums-panel').classList.add('hidden');
            }
        } else {
            filtersPanel.classList.add('hidden');
        }
    });

    // ── Albums Panel ─────────────────────────────────────────────────────────

    var albumsPanel = document.getElementById('gallery-albums-panel');

    function renderAlbumsPanel() {
        albumsPanel.innerHTML =
            '<div class="gallery-albums-tabs">' +
                '<button class="gallery-albums-tab' + (albumTab === 'date' ? ' active' : '') + '" data-tab="date">Date</button>' +
                '<button class="gallery-albums-tab' + (albumTab === 'location' ? ' active' : '') + '" data-tab="location">Location</button>' +
                '<button class="gallery-albums-tab' + (albumTab === 'camera' ? ' active' : '') + '" data-tab="camera">Camera</button>' +
            '</div>' +
            '<div id="gallery-albums-content" class="gallery-albums-content"></div>';

        // Wire tab buttons
        var tabBtns = albumsPanel.querySelectorAll('.gallery-albums-tab');
        for (var ti = 0; ti < tabBtns.length; ti++) {
            (function(btn) {
                btn.addEventListener('click', function() {
                    albumTab = btn.getAttribute('data-tab');
                    renderAlbumsPanel();
                    loadAlbumTab(albumTab);
                });
            })(tabBtns[ti]);
        }

        loadAlbumTab(albumTab);
    }

    function loadAlbumTab(tab) {
        var content = document.getElementById('gallery-albums-content');
        if (!content) return;

        if (albumData[tab]) {
            renderAlbumContent(tab, albumData[tab], content);
            return;
        }

        content.innerHTML = '<p style="padding:0.75rem;color:var(--text-muted)">Loading...</p>';

        var endpoint = '/api/v1/gallery/albums/' + tab;
        API.get(endpoint).then(function(data) {
            albumData[tab] = data;
            renderAlbumContent(tab, data, content);
        }).catch(function() {
            content.innerHTML = '<p style="padding:0.75rem;color:var(--text-muted)">Failed to load albums</p>';
        });
    }

    function renderAlbumContent(tab, data, content) {
        var html = '<div class="gallery-album-list">';

        if (tab === 'date') {
            if (!data || data.length === 0) {
                html += '<p class="gallery-album-empty">No date albums</p>';
            } else {
                for (var y = 0; y < data.length; y++) {
                    var year = data[y];
                    html += '<div class="gallery-album-group">' +
                        '<div class="gallery-album-group-header" data-toggle="date-' + year.year + '">' +
                            '<span class="gallery-album-arrow">&#9654;</span> ' +
                            esc(String(year.year)) +
                        '</div>' +
                        '<div class="gallery-album-group-body hidden" id="album-date-' + year.year + '">';
                    if (year.months) {
                        for (var m = 0; m < year.months.length; m++) {
                            var mo = year.months[m];
                            var monthName = getMonthName(mo.month);
                            html += '<a class="gallery-album-item" data-filter-type="date" data-year="' + year.year + '" data-month="' + mo.month + '">' +
                                esc(monthName) + ' <span class="gallery-album-count">' + esc(String(mo.count)) + '</span>' +
                            '</a>';
                        }
                    }
                    html += '</div></div>';
                }
            }
        } else if (tab === 'location') {
            if (!data || data.length === 0) {
                html += '<p class="gallery-album-empty">No location albums</p>';
            } else {
                for (var c = 0; c < data.length; c++) {
                    var country = data[c];
                    html += '<div class="gallery-album-group">' +
                        '<div class="gallery-album-group-header" data-toggle="loc-' + c + '">' +
                            '<span class="gallery-album-arrow">&#9654;</span> ' +
                            esc(country.country || 'Unknown') +
                        '</div>' +
                        '<div class="gallery-album-group-body hidden" id="album-loc-' + c + '">';
                    if (country.cities) {
                        for (var ci = 0; ci < country.cities.length; ci++) {
                            var city = country.cities[ci];
                            html += '<a class="gallery-album-item" data-filter-type="location" data-country="' + esc(country.country) + '" data-city="' + esc(city.city) + '">' +
                                esc(city.city || 'Unknown') + ' <span class="gallery-album-count">' + esc(String(city.count)) + '</span>' +
                            '</a>';
                        }
                    }
                    html += '</div></div>';
                }
            }
        } else if (tab === 'camera') {
            if (!data || data.length === 0) {
                html += '<p class="gallery-album-empty">No camera albums</p>';
            } else {
                for (var mk = 0; mk < data.length; mk++) {
                    var make = data[mk];
                    html += '<div class="gallery-album-group">' +
                        '<div class="gallery-album-group-header" data-toggle="cam-' + mk + '">' +
                            '<span class="gallery-album-arrow">&#9654;</span> ' +
                            esc(make.make || 'Unknown') +
                        '</div>' +
                        '<div class="gallery-album-group-body hidden" id="album-cam-' + mk + '">';
                    if (make.models) {
                        for (var mi = 0; mi < make.models.length; mi++) {
                            var model = make.models[mi];
                            html += '<a class="gallery-album-item" data-filter-type="camera" data-make="' + esc(make.make) + '" data-model="' + esc(model.model) + '">' +
                                esc(model.model || 'Unknown') + ' <span class="gallery-album-count">' + esc(String(model.count)) + '</span>' +
                            '</a>';
                        }
                    }
                    html += '</div></div>';
                }
            }
        }

        html += '</div>';
        content.innerHTML = html;

        // Wire group toggles
        var headers = content.querySelectorAll('.gallery-album-group-header');
        for (var h = 0; h < headers.length; h++) {
            (function(header) {
                header.addEventListener('click', function() {
                    var toggleId = header.getAttribute('data-toggle');
                    var body = document.getElementById('album-' + toggleId);
                    if (body) {
                        var arrow = header.querySelector('.gallery-album-arrow');
                        if (body.classList.contains('hidden')) {
                            body.classList.remove('hidden');
                            if (arrow) arrow.innerHTML = '&#9660;';
                        } else {
                            body.classList.add('hidden');
                            if (arrow) arrow.innerHTML = '&#9654;';
                        }
                    }
                });
            })(headers[h]);
        }

        // Wire album item clicks (apply filter)
        var albumItems = content.querySelectorAll('.gallery-album-item');
        for (var ai = 0; ai < albumItems.length; ai++) {
            (function(aItem) {
                aItem.addEventListener('click', function(e) {
                    e.preventDefault();
                    var filterType = aItem.getAttribute('data-filter-type');

                    if (filterType === 'date') {
                        var year = aItem.getAttribute('data-year');
                        var month = aItem.getAttribute('data-month');
                        var paddedMonth = month.length === 1 ? '0' + month : month;
                        // Set date range to the entire month
                        var daysInMonth = new Date(parseInt(year, 10), parseInt(month, 10), 0).getDate();
                        filterDateFrom = year + '-' + paddedMonth + '-01';
                        filterDateTo = year + '-' + paddedMonth + '-' + (daysInMonth < 10 ? '0' : '') + daysInMonth;
                        filterQuery = '';
                        filterCameraMake = '';
                        filterCountry = '';
                        filterTags = [];
                    } else if (filterType === 'location') {
                        filterCountry = aItem.getAttribute('data-country') || '';
                        filterDateFrom = '';
                        filterDateTo = '';
                        filterQuery = '';
                        filterCameraMake = '';
                        filterTags = [];
                    } else if (filterType === 'camera') {
                        filterCameraMake = aItem.getAttribute('data-make') || '';
                        filterDateFrom = '';
                        filterDateTo = '';
                        filterQuery = '';
                        filterCountry = '';
                        filterTags = [];
                    }

                    // Update filter panel inputs if visible
                    if (filtersOpen) {
                        renderFiltersPanel();
                    }

                    offset = 0;
                    loadGallery(false);
                });
            })(albumItems[ai]);
        }
    }

    document.getElementById('gallery-albums-toggle').addEventListener('click', function() {
        albumsOpen = !albumsOpen;
        if (albumsOpen) {
            albumsPanel.classList.remove('hidden');
            renderAlbumsPanel();
            // Close filters if open
            if (filtersOpen) {
                filtersOpen = false;
                filtersPanel.classList.add('hidden');
            }
        } else {
            albumsPanel.classList.add('hidden');
        }
    });

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
        html += '<div class="lb-section">';
        html += '<div class="lb-section-title">File</div>';
        html += '<div class="lb-meta-grid">';
        html += '<div class="lb-meta-key">Name</div><div class="lb-meta-val">' + esc(item.file_name) + '</div>';
        html += '<div class="lb-meta-key">Size</div><div class="lb-meta-val">' + formatBytes(item.size) + '</div>';
        if (meta.width && meta.height) {
            html += '<div class="lb-meta-key">Dimensions</div><div class="lb-meta-val">' + esc(String(meta.width)) + ' x ' + esc(String(meta.height)) + '</div>';
        }
        html += '<div class="lb-meta-key">Modified</div><div class="lb-meta-val">' + formatDate(item.mod_time) + '</div>';
        if (meta.date_taken) {
            html += '<div class="lb-meta-key">Date Taken</div><div class="lb-meta-val">' + formatDate(meta.date_taken) + '</div>';
        }
        html += '</div></div>';

        // Camera info
        if (meta.camera_make || meta.camera_model || meta.focal_length || meta.aperture || meta.shutter_speed || meta.iso) {
            html += '<div class="lb-section">';
            html += '<div class="lb-section-title">Camera</div>';
            html += '<div class="lb-meta-grid">';
            if (meta.camera_make) {
                html += '<div class="lb-meta-key">Make</div><div class="lb-meta-val">' + esc(meta.camera_make) + '</div>';
            }
            if (meta.camera_model) {
                html += '<div class="lb-meta-key">Model</div><div class="lb-meta-val">' + esc(meta.camera_model) + '</div>';
            }
            if (meta.focal_length) {
                html += '<div class="lb-meta-key">Focal Length</div><div class="lb-meta-val">' + esc(String(meta.focal_length)) + '</div>';
            }
            if (meta.aperture) {
                html += '<div class="lb-meta-key">Aperture</div><div class="lb-meta-val">' + esc(String(meta.aperture)) + '</div>';
            }
            if (meta.shutter_speed) {
                html += '<div class="lb-meta-key">Shutter Speed</div><div class="lb-meta-val">' + esc(String(meta.shutter_speed)) + '</div>';
            }
            if (meta.iso) {
                html += '<div class="lb-meta-key">ISO</div><div class="lb-meta-val">' + esc(String(meta.iso)) + '</div>';
            }
            html += '</div></div>';
        }

        // Location
        if (meta.latitude || meta.longitude || meta.location_city || meta.location_country) {
            html += '<div class="lb-section">';
            html += '<div class="lb-section-title">Location</div>';
            html += '<div class="lb-meta-grid">';
            if (meta.location_city) {
                html += '<div class="lb-meta-key">City</div><div class="lb-meta-val">' + esc(meta.location_city) + '</div>';
            }
            if (meta.location_country) {
                html += '<div class="lb-meta-key">Country</div><div class="lb-meta-val">' + esc(meta.location_country) + '</div>';
            }
            if (meta.latitude && meta.longitude) {
                html += '<div class="lb-meta-key">Coordinates</div><div class="lb-meta-val">' +
                    esc(String(meta.latitude.toFixed(6))) + ', ' + esc(String(meta.longitude.toFixed(6))) + '</div>';
            }
            html += '</div></div>';
        }

        // Tags
        html += '<div class="lb-section">';
        html += '<div class="lb-section-title">Tags</div>';
        html += '<div id="lb-tags-container">';
        html += renderLightboxTags(meta.tags || []);
        html += '</div>';
        html += '<div class="lb-tag-add">' +
            '<input type="text" id="lb-tag-input" placeholder="Add tag...">' +
        '</div>';
        html += '</div>';

        body.innerHTML = html;

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

        // Wire tag delete buttons
        wireTagDeleteButtons(item);
    }

    function renderLightboxTags(tags) {
        if (!tags || tags.length === 0) {
            return '<span class="lb-tags-empty">No tags</span>';
        }
        var html = '';
        for (var i = 0; i < tags.length; i++) {
            html += '<span class="lb-tag-chip">' +
                esc(tags[i]) +
                '<button class="lb-tag-chip-x" data-tag="' + esc(tags[i]) + '">&times;</button>' +
            '</span>';
        }
        return html;
    }

    function wireTagDeleteButtons(item) {
        var btns = document.querySelectorAll('.lb-tag-chip-x');
        for (var i = 0; i < btns.length; i++) {
            (function(btn) {
                btn.addEventListener('click', function() {
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
