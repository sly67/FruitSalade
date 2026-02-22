function renderSearch() {
    var hash = window.location.hash.replace('#search', '').replace(/^\//, '');
    var initialQuery = hash ? decodeURIComponent(hash) : '';

    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="search-page">' +
            '<div class="search-page-header">' +
                '<input type="text" id="search-page-input" class="search-page-input" placeholder="Search files, folders, tags..." autofocus>' +
            '</div>' +
            '<div class="search-filters" id="search-filters">' +
                '<button class="search-pill active" data-type="all">All</button>' +
                '<button class="search-pill" data-type="files">Files</button>' +
                '<button class="search-pill" data-type="dirs">Folders</button>' +
                '<button class="search-pill" data-type="images">Images</button>' +
            '</div>' +
            '<div id="search-results" class="search-results">' +
                '<div class="empty-state">' +
                    '<span class="empty-icon">&#128269;</span>' +
                    '<p>Start typing to search</p>' +
                '</div>' +
            '</div>' +
            '<div id="search-recent" class="search-recent"></div>' +
        '</div>';

    var input = document.getElementById('search-page-input');
    var resultsDiv = document.getElementById('search-results');
    var recentDiv = document.getElementById('search-recent');
    var searchTimer = null;
    var currentType = 'all';

    // Set initial query
    if (initialQuery) {
        input.value = initialQuery;
    }

    // Filter pills
    document.querySelectorAll('.search-pill').forEach(function(pill) {
        pill.addEventListener('click', function() {
            document.querySelectorAll('.search-pill').forEach(function(p) { p.classList.remove('active'); });
            pill.classList.add('active');
            currentType = pill.getAttribute('data-type');
            var q = input.value.trim();
            if (q) doSearch(q, currentType);
        });
    });

    // Search input with debounce
    input.addEventListener('input', function() {
        var q = input.value.trim();
        clearTimeout(searchTimer);
        if (!q) {
            showRecentSearches();
            resultsDiv.innerHTML =
                '<div class="empty-state">' +
                    '<span class="empty-icon">&#128269;</span>' +
                    '<p>Start typing to search</p>' +
                '</div>';
            return;
        }
        recentDiv.innerHTML = '';
        searchTimer = setTimeout(function() {
            doSearch(q, currentType);
        }, 300);
    });

    // Enter key saves to recent
    input.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
            var q = input.value.trim();
            if (q) {
                saveRecentSearch(q);
                doSearch(q, currentType);
            }
        }
    });

    function doSearch(query, typeFilter) {
        var url = '/api/v1/search?q=' + encodeURIComponent(query);
        if (typeFilter && typeFilter !== 'all') {
            url += '&type=' + typeFilter;
        }

        resultsDiv.innerHTML =
            '<div style="padding:1.5rem;color:var(--text-muted)">Searching...</div>';

        API.get(url).then(function(results) {
            renderResults(results, query);
        }).catch(function() {
            resultsDiv.innerHTML =
                '<div class="alert alert-error">Search failed</div>';
        });
    }

    function renderResults(items, query) {
        if (!items || items.length === 0) {
            resultsDiv.innerHTML =
                '<div class="empty-state">' +
                    '<span class="empty-icon">&#128269;</span>' +
                    '<p>No results for "' + esc(query) + '"</p>' +
                '</div>';
            return;
        }

        var html = '<div class="search-count">' + items.length + ' result' + (items.length !== 1 ? 's' : '') + '</div>';
        html += '<table class="responsive-table"><thead><tr>' +
            '<th>Name</th><th>Path</th><th>Size</th><th>Modified</th>' +
            '</tr></thead><tbody>';

        for (var i = 0; i < items.length; i++) {
            var f = items[i];
            var iconHtml = FileTypes.icon(f.name, f.is_dir);
            var href = f.is_dir ? '#browser' + f.path : '#viewer' + f.path;
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
        resultsDiv.innerHTML = html;
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

    // Recent searches
    function getRecentSearches() {
        try {
            return JSON.parse(localStorage.getItem('recentSearches') || '[]');
        } catch (e) {
            return [];
        }
    }

    function saveRecentSearch(q) {
        var recent = getRecentSearches().filter(function(s) { return s !== q; });
        recent.unshift(q);
        if (recent.length > 10) recent = recent.slice(0, 10);
        localStorage.setItem('recentSearches', JSON.stringify(recent));
    }

    function showRecentSearches() {
        var recent = getRecentSearches();
        if (recent.length === 0) {
            recentDiv.innerHTML = '';
            return;
        }
        var html = '<div class="search-recent-label">Recent searches</div>';
        for (var i = 0; i < recent.length; i++) {
            html += '<button class="search-recent-item" data-query="' + esc(recent[i]) + '">' +
                '&#128269; ' + esc(recent[i]) + '</button>';
        }
        recentDiv.innerHTML = html;

        recentDiv.querySelectorAll('.search-recent-item').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var q = btn.getAttribute('data-query');
                input.value = q;
                recentDiv.innerHTML = '';
                doSearch(q, currentType);
            });
        });
    }

    // Initial render
    if (initialQuery) {
        doSearch(initialQuery, currentType);
    } else {
        showRecentSearches();
    }
}
