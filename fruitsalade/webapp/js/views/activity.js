// Activity feed view — shows file operations timeline
function renderActivity() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar"><h2>Activity</h2></div>' +
        '<div id="activity-container" style="padding:0 1.5rem">' +
            '<div class="skeleton skeleton-card"></div>' +
            '<div class="skeleton skeleton-card"></div>' +
        '</div>';

    var entries = [];
    var loading = false;

    function loadActivity(before) {
        if (loading) return;
        loading = true;

        var url = '/api/v1/activity?limit=50';
        if (before) {
            url += '&before=' + encodeURIComponent(before);
        }

        API.get(url).then(function(data) {
            loading = false;
            entries = entries.concat(data || []);
            renderEntries();
        }).catch(function() {
            loading = false;
            document.getElementById('activity-container').innerHTML =
                '<div class="alert alert-error">Failed to load activity</div>';
        });
    }

    function renderEntries() {
        var container = document.getElementById('activity-container');
        if (!container) return;

        if (entries.length === 0) {
            container.innerHTML = '<p class="text-muted" style="padding:1rem">No activity yet</p>';
            return;
        }

        // Group by date
        var groups = {};
        var groupOrder = [];
        for (var i = 0; i < entries.length; i++) {
            var e = entries[i];
            var dateKey = e.created_at ? e.created_at.substring(0, 10) : 'Unknown';
            if (!groups[dateKey]) {
                groups[dateKey] = [];
                groupOrder.push(dateKey);
            }
            groups[dateKey].push(e);
        }

        var html = '<div class="activity-list">';
        for (var g = 0; g < groupOrder.length; g++) {
            var dateKey = groupOrder[g];
            html += '<div class="activity-date-header">' + formatActivityDate(dateKey) + '</div>';
            var items = groups[dateKey];
            for (var j = 0; j < items.length; j++) {
                html += renderActivityEntry(items[j]);
            }
        }
        html += '</div>';

        // Load more button
        if (entries.length >= 50 && entries.length % 50 === 0) {
            html += '<div style="text-align:center;padding:1rem">' +
                '<button class="btn btn-outline" id="activity-load-more">Load More</button>' +
            '</div>';
        }

        container.innerHTML = html;

        var loadMore = document.getElementById('activity-load-more');
        if (loadMore) {
            loadMore.addEventListener('click', function() {
                var last = entries[entries.length - 1];
                if (last && last.created_at) {
                    loadActivity(last.created_at);
                }
            });
        }
    }

    function renderActivityEntry(entry) {
        var icon = actionIcon(entry.action);
        var filename = entry.resource_path ? entry.resource_path.split('/').pop() : '';
        var username = entry.username || 'Unknown';
        var timeStr = formatActivityTime(entry.created_at);
        var verb = actionVerb(entry.action);

        return '<div class="activity-entry">' +
            '<span class="activity-icon">' + icon + '</span>' +
            '<div class="activity-body">' +
                '<span class="activity-text">' +
                    '<strong>' + esc(username) + '</strong> ' + verb + ' ' +
                    '<a class="activity-path" href="#browser/' + encodeURIComponent(entry.resource_path.replace(/^\//, '')) + '">' + esc(filename) + '</a>' +
                '</span>' +
                '<span class="activity-time">' + timeStr + '</span>' +
                '<span class="activity-full-path text-muted">' + esc(entry.resource_path) + '</span>' +
            '</div>' +
        '</div>';
    }

    function actionIcon(action) {
        switch (action) {
            case 'create': return '&#128196;';
            case 'modify': return '&#9998;';
            case 'delete': return '&#128465;';
            case 'version': return '&#128260;';
            default: return '&#128196;';
        }
    }

    function actionVerb(action) {
        switch (action) {
            case 'create': return 'created';
            case 'modify': return 'modified';
            case 'delete': return 'deleted';
            case 'version': return 'rolled back';
            default: return action;
        }
    }

    function formatActivityDate(dateStr) {
        var today = new Date().toISOString().substring(0, 10);
        var yesterday = new Date(Date.now() - 86400000).toISOString().substring(0, 10);
        if (dateStr === today) return 'Today';
        if (dateStr === yesterday) return 'Yesterday';
        var d = new Date(dateStr + 'T00:00:00');
        var months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
        return months[d.getMonth()] + ' ' + d.getDate() + ', ' + d.getFullYear();
    }

    function formatActivityTime(isoStr) {
        if (!isoStr) return '';
        var d = new Date(isoStr);
        var h = d.getHours();
        var m = d.getMinutes();
        var ampm = h >= 12 ? 'PM' : 'AM';
        h = h % 12 || 12;
        return h + ':' + (m < 10 ? '0' : '') + m + ' ' + ampm;
    }

    loadActivity(null);
}
