// User + Admin dashboard view
function renderDashboard() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar"><h2>Dashboard</h2></div>' +
        '<div id="dashboard-content">' +
            '<div class="dashboard-grid">' +
                '<div class="skeleton skeleton-card"></div>' +
                '<div class="skeleton skeleton-card"></div>' +
                '<div class="skeleton skeleton-card"></div>' +
                '<div class="skeleton skeleton-card"></div>' +
            '</div>' +
        '</div>';

    API.get('/api/v1/user/dashboard').then(function(data) {
        var html = '';

        // Welcome header
        html += '<div class="dashboard-section">' +
            '<h3>Welcome, ' + esc(data.username) + '</h3>' +
        '</div>';

        // Quota / Usage section
        html += '<div class="dashboard-section">' +
            '<div class="quota-grid">' +
                quotaCard('Storage', data.storage_used, data.quota.max_storage_bytes) +
                quotaCard('Bandwidth Today', data.bandwidth_today, data.quota.max_bandwidth_per_day) +
            '</div>' +
            '<div class="quota-grid" style="margin-top:0.75rem">' +
                limitCard('Requests/min', data.quota.max_requests_per_minute) +
                limitCard('Max Upload', data.quota.max_upload_size_bytes) +
            '</div>' +
        '</div>';

        // Quick stats
        html += '<div class="dashboard-section">' +
            '<div class="quick-stats-row">' +
                quickStat(data.file_count, 'files') +
                quickStat(data.share_link_count, 'active share links') +
            '</div>' +
        '</div>';

        // Groups
        html += '<div class="dashboard-section">' +
            '<h3>My Groups</h3>';
        if (data.groups && data.groups.length > 0) {
            html += '<div class="group-list">';
            for (var i = 0; i < data.groups.length; i++) {
                var g = data.groups[i];
                html += '<span class="group-chip">' +
                    esc(g.group_name) +
                    ' <span class="role-badge role-' + esc(g.role) + '">' + esc(g.role) + '</span>' +
                '</span>';
            }
            html += '</div>';
        } else {
            html += '<p class="dashboard-empty">No group memberships</p>';
        }
        html += '</div>';

        // Admin section
        if (sessionStorage.getItem('is_admin') === 'true') {
            html += '<div id="admin-stats-section"></div>';
        }

        document.getElementById('dashboard-content').innerHTML = html;

        // Load admin stats if admin
        if (sessionStorage.getItem('is_admin') === 'true') {
            loadAdminStats();
        }
    }).catch(function() {
        document.getElementById('dashboard-content').innerHTML =
            '<div class="alert alert-error">Failed to load dashboard</div>';
    });
}

function loadAdminStats() {
    API.get('/api/v1/admin/stats').then(function(data) {
        var el = document.getElementById('admin-stats-section');
        if (!el) return;
        el.innerHTML =
            '<div class="dashboard-section">' +
                '<h3>System Overview</h3>' +
                '<div class="stats-grid">' +
                    statCard('Users', data.users) +
                    statCard('Active Sessions', data.active_sessions) +
                    statCard('Files', data.files) +
                    statCard('Storage', formatBytes(data.total_storage)) +
                    statCard('Share Links', data.active_share_links) +
                    statCard('Groups', data.groups) +
                '</div>' +
            '</div>';
    }).catch(function() {
        var el = document.getElementById('admin-stats-section');
        if (el) el.innerHTML = '';
    });
}

function quotaCard(label, used, limit) {
    var pct = 0;
    var usedStr = formatBytes(used);
    var limitStr = limit > 0 ? formatBytes(limit) : 'Unlimited';
    if (limit > 0) {
        pct = Math.min(100, Math.round((used / limit) * 100));
    }
    var barClass = 'progress-bar-fill';
    if (pct > 95) barClass += ' danger';
    else if (pct > 80) barClass += ' warning';

    return '<div class="quota-card">' +
        '<div class="quota-label">' + esc(label) + '</div>' +
        '<div class="progress-bar"><div class="' + barClass + '" style="width:' + pct + '%"></div></div>' +
        '<div class="quota-value">' + esc(usedStr) + ' / ' + esc(limitStr) +
            (limit > 0 ? ' (' + pct + '%)' : '') +
        '</div>' +
    '</div>';
}

function limitCard(label, value) {
    var display = value > 0 ? (label === 'Max Upload' ? formatBytes(value) : String(value)) : 'Unlimited';
    return '<div class="quota-card">' +
        '<div class="quota-label">' + esc(label) + '</div>' +
        '<div class="quota-value" style="font-size:1.25rem;font-weight:600;margin-top:0.5rem">' +
            esc(display) +
        '</div>' +
    '</div>';
}

function quickStat(count, label) {
    return '<span class="quick-stat-item"><strong>' + esc(String(count)) + '</strong> ' + esc(label) + '</span>';
}

function statCard(label, value) {
    return '<div class="stat-card">' +
        '<div class="stat-label">' + esc(label) + '</div>' +
        '<div class="stat-value">' + esc(String(value)) + '</div>' +
    '</div>';
}
