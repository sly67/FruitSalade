// Admin dashboard stats view (ported from admin UI)
function renderDashboard() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar"><h2>Dashboard</h2></div>' +
        '<div class="stats-grid" id="stats-grid">Loading...</div>';

    API.get('/api/v1/admin/stats').then(function(data) {
        document.getElementById('stats-grid').innerHTML =
            statCard('Users', data.users) +
            statCard('Active Sessions', data.active_sessions) +
            statCard('Files', data.files) +
            statCard('Storage', formatBytes(data.total_storage)) +
            statCard('Active Share Links', data.active_share_links) +
            statCard('Total Share Links', data.total_share_links);
    }).catch(function() {
        document.getElementById('stats-grid').innerHTML =
            '<div class="alert alert-error">Failed to load stats</div>';
    });
}

function statCard(label, value) {
    return '<div class="stat-card">' +
        '<div class="stat-label">' + esc(label) + '</div>' +
        '<div class="stat-value">' + esc(String(value)) + '</div>' +
    '</div>';
}
