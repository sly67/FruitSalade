// User + Admin dashboard view

// Data caches (reset on renderDashboard entry, preserved across tab switches)
var _dashboardData = null;
var _adminStatsData = null;
var _storageDashData = null;
var _activeTab = 'overview';

function renderDashboard() {
    // Reset caches on fresh navigation
    _dashboardData = null;
    _adminStatsData = null;
    _storageDashData = null;
    _activeTab = 'overview';

    var app = document.getElementById('app');
    var isAdmin = sessionStorage.getItem('is_admin') === 'true';

    var tabNav = '<div class="fm-tab-nav" role="tablist">' +
        '<button class="fm-tab active" data-dash-tab="overview" role="tab" aria-selected="true">Overview</button>' +
        '<button class="fm-tab" data-dash-tab="security" role="tab" aria-selected="false">Security</button>';
    if (isAdmin) {
        tabNav += '<button class="fm-tab" data-dash-tab="analytics" role="tab" aria-selected="false">Analytics</button>';
    }
    tabNav += '</div>';

    app.innerHTML =
        '<div class="toolbar"><h2>Dashboard</h2></div>' +
        tabNav +
        '<div id="dash-tab-content"></div>';

    // Wire tab clicks
    var tabs = app.querySelectorAll('.fm-tab');
    tabs.forEach(function(tab) {
        tab.addEventListener('click', function() {
            var target = tab.getAttribute('data-dash-tab');
            if (target === _activeTab) return;
            _activeTab = target;
            tabs.forEach(function(t) {
                t.classList.remove('active');
                t.setAttribute('aria-selected', 'false');
            });
            tab.classList.add('active');
            tab.setAttribute('aria-selected', 'true');
            if (target === 'overview') renderOverviewTab();
            else if (target === 'security') renderSecurityTab();
            else if (target === 'analytics') renderAnalyticsTab();
        });
    });

    // Default tab
    renderOverviewTab();
}

// ─── Overview Tab ─────────────────────────────────────────────────────────────

function renderOverviewTab() {
    var container = document.getElementById('dash-tab-content');

    // Use cached data if available
    if (_dashboardData) {
        renderOverviewContent(container, _dashboardData);
        return;
    }

    container.innerHTML =
        '<div style="padding:0 1.5rem">' +
            '<div class="dashboard-grid">' +
                '<div class="skeleton skeleton-card"></div>' +
                '<div class="skeleton skeleton-card"></div>' +
                '<div class="skeleton skeleton-card"></div>' +
                '<div class="skeleton skeleton-card"></div>' +
            '</div>' +
        '</div>';

    API.get('/api/v1/user/dashboard').then(function(data) {
        _dashboardData = data;
        renderOverviewContent(container, data);
    }).catch(function() {
        container.innerHTML =
            '<div style="padding:0 1.5rem"><div class="alert alert-error">Failed to load dashboard</div></div>';
    });
}

function renderOverviewContent(container, data) {
    var html = '<div style="padding:0 1.5rem">';

    // Welcome header
    html += '<div class="dashboard-section">' +
        '<h3>Welcome, ' + esc(data.username) + '</h3>' +
    '</div>';

    // Quota / Usage section
    var hasStorageQuota = data.quota.max_storage_bytes > 0;
    var hasBandwidthQuota = data.quota.max_bandwidth_per_day > 0;
    var hasLimits = data.quota.max_requests_per_minute > 0 || data.quota.max_upload_size_bytes > 0;

    html += '<div class="dashboard-section">' +
        '<div class="quota-grid">';
    if (hasStorageQuota) {
        html += quotaCard('Storage', data.storage_used, data.quota.max_storage_bytes);
    } else {
        html += metricCard('Storage Used', formatBytes(data.storage_used));
    }
    if (hasBandwidthQuota) {
        html += quotaCard('Bandwidth Today', data.bandwidth_today, data.quota.max_bandwidth_per_day);
    } else {
        html += metricCard('Bandwidth Today', formatBytes(data.bandwidth_today));
    }
    html += '</div>';
    if (hasLimits) {
        html += '<div class="quota-grid" style="margin-top:0.75rem">' +
            limitCard('Requests/min', data.quota.max_requests_per_minute) +
            limitCard('Max Upload', data.quota.max_upload_size_bytes) +
        '</div>';
    }
    html += '</div>';

    // Quick stats — inside quota-card style
    html += '<div class="dashboard-section">' +
        '<div class="quota-grid">' +
            metricCard('My Files', String(data.file_count)) +
            metricCard('Active Share Links', String(data.share_link_count)) +
        '</div>' +
    '</div>';

    // Bandwidth history chart (7 days)
    if (data.bandwidth_history && data.bandwidth_history.length > 0) {
        html += '<div class="dashboard-section">' +
            '<h3>Bandwidth (Last 7 Days)</h3>' +
            '<canvas id="chart-bw-history" class="chart-canvas" width="500" height="220"></canvas>' +
        '</div>';
    }

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
        html += '<div class="dashboard-placeholder">You are not a member of any groups yet</div>';
    }
    html += '</div>';

    html += '</div>';
    container.innerHTML = html;

    // Draw bandwidth history stacked bar chart
    if (data.bandwidth_history && data.bandwidth_history.length > 0) {
        drawBandwidthChart('chart-bw-history', data.bandwidth_history);
    }
}

function drawBandwidthChart(canvasId, history) {
    var canvas = document.getElementById(canvasId);
    if (!canvas) return;
    var ctx = canvas.getContext('2d');
    var dpr = window.devicePixelRatio || 1;
    var w = canvas.clientWidth;
    var h = canvas.clientHeight;
    canvas.width = w * dpr;
    canvas.height = h * dpr;
    ctx.scale(dpr, dpr);

    var textColor = getChartTextColor();
    var borderColor = getChartBorderColor();
    var uploadColor = '#2563EB';
    var downloadColor = '#16A34A';

    var padding = { top: 20, right: 20, bottom: 40, left: 70 };
    var chartW = w - padding.left - padding.right;
    var chartH = h - padding.top - padding.bottom;
    var n = history.length;
    var barGap = 8;
    var barW = Math.min(40, (chartW - barGap * (n + 1)) / n);

    // Find max total
    var maxVal = 0;
    for (var i = 0; i < n; i++) {
        var total = (history[i].bytes_in || 0) + (history[i].bytes_out || 0);
        if (total > maxVal) maxVal = total;
    }
    if (maxVal === 0) maxVal = 1;

    // Grid lines
    ctx.strokeStyle = borderColor;
    ctx.lineWidth = 0.5;
    for (var g = 0; g <= 4; g++) {
        var gy = padding.top + chartH - (g / 4) * chartH;
        ctx.beginPath();
        ctx.moveTo(padding.left, gy);
        ctx.lineTo(padding.left + chartW, gy);
        ctx.stroke();

        ctx.fillStyle = textColor;
        ctx.font = '10px -apple-system, sans-serif';
        ctx.textAlign = 'right';
        ctx.textBaseline = 'middle';
        ctx.fillText(formatBytes((g / 4) * maxVal), padding.left - 6, gy);
    }

    // Bars
    for (var i = 0; i < n; i++) {
        var bIn = history[i].bytes_in || 0;
        var bOut = history[i].bytes_out || 0;
        var totalH = ((bIn + bOut) / maxVal) * chartH;
        var upH = (bIn / maxVal) * chartH;
        var downH = (bOut / maxVal) * chartH;

        var x = padding.left + barGap + i * (barW + barGap);
        var baseY = padding.top + chartH;

        // Download bar (green, bottom)
        if (downH > 0) {
            ctx.fillStyle = downloadColor;
            ctx.beginPath();
            roundRect(ctx, x, baseY - downH, barW, downH, 3);
            ctx.fill();
        }

        // Upload bar (blue, stacked on top)
        if (upH > 0) {
            ctx.fillStyle = uploadColor;
            ctx.beginPath();
            roundRect(ctx, x, baseY - downH - upH, barW, upH, 3);
            ctx.fill();
        }

        // X-axis label (day name)
        ctx.fillStyle = textColor;
        ctx.font = '10px -apple-system, sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'top';
        var dateLabel = history[i].date ? history[i].date.substring(5) : '';
        ctx.fillText(dateLabel, x + barW / 2, padding.top + chartH + 6);
    }

    // Legend
    ctx.font = '11px -apple-system, sans-serif';
    ctx.textAlign = 'left';
    ctx.textBaseline = 'middle';
    var legendX = padding.left;
    var legendY = padding.top + chartH + 24;

    ctx.fillStyle = uploadColor;
    ctx.fillRect(legendX, legendY - 5, 12, 10);
    ctx.fillStyle = textColor;
    ctx.fillText('Upload', legendX + 16, legendY);

    ctx.fillStyle = downloadColor;
    ctx.fillRect(legendX + 80, legendY - 5, 12, 10);
    ctx.fillStyle = textColor;
    ctx.fillText('Download', legendX + 96, legendY);
}

// ─── Security Tab ─────────────────────────────────────────────────────────────

function renderSecurityTab() {
    var container = document.getElementById('dash-tab-content');
    container.innerHTML =
        '<div style="padding:0 1.5rem">' +
            '<div class="dashboard-section">' +
                '<h3>Security</h3>' +
                '<div id="totp-status-area"><span class="text-muted">Loading 2FA status...</span></div>' +
            '</div>' +
        '</div>';
    loadTOTPStatus();
}

// ─── Analytics Tab (Admin only) ───────────────────────────────────────────────

function renderAnalyticsTab() {
    var container = document.getElementById('dash-tab-content');

    // Use cached data if available
    if (_adminStatsData && _storageDashData) {
        renderAnalyticsContent(container, _adminStatsData, _storageDashData);
        return;
    }

    container.innerHTML =
        '<div style="padding:0 1.5rem">' +
            '<div class="dashboard-grid">' +
                '<div class="skeleton skeleton-card"></div>' +
                '<div class="skeleton skeleton-card"></div>' +
                '<div class="skeleton skeleton-card"></div>' +
                '<div class="skeleton skeleton-card"></div>' +
            '</div>' +
        '</div>';

    Promise.all([
        API.get('/api/v1/admin/stats'),
        API.get('/api/v1/admin/storage-dashboard')
    ]).then(function(results) {
        _adminStatsData = results[0];
        _storageDashData = results[1];
        renderAnalyticsContent(container, _adminStatsData, _storageDashData);
    }).catch(function() {
        container.innerHTML =
            '<div style="padding:0 1.5rem"><div class="alert alert-error">Failed to load analytics</div></div>';
    });
}

function renderAnalyticsContent(container, stats, storage) {
    var html = '<div style="padding:0 1.5rem">';

    // System stats — 4 cards (Users, Sessions, Groups, Share Links)
    html += '<div class="dashboard-section">' +
        '<h3>System Overview</h3>' +
        '<div class="stats-grid">' +
            statCard('Users', stats.users) +
            statCard('Active Sessions', stats.active_sessions) +
            statCard('Groups', stats.groups) +
            statCard('Share Links', stats.active_share_links) +
        '</div>' +
    '</div>';

    // Storage summary — 3 cards
    html += '<div class="dashboard-section">' +
        '<h3>Storage Analytics</h3>' +
        '<div class="stats-grid">' +
            statCard('Total Storage', formatBytes(storage.total_size)) +
            statCard('Total Files', storage.total_files) +
            statCard('Trash', formatBytes(storage.trash_size) + ' (' + storage.trash_count + ' files)') +
        '</div>';

    // Chart grid
    html += '<div class="chart-row">' +
        '<div class="chart-section">' +
            '<h4>Storage by User</h4>' +
            '<canvas id="chart-by-user" class="chart-canvas" width="400" height="250"></canvas>' +
            '<div id="legend-by-user" class="chart-legend"></div>' +
        '</div>' +
        '<div class="chart-section">' +
            '<h4>File Type Distribution</h4>' +
            '<canvas id="chart-by-type" class="chart-canvas chart-donut" width="300" height="300"></canvas>' +
            '<div id="legend-by-type" class="chart-legend"></div>' +
        '</div>' +
    '</div>';

    html += '<div class="chart-row">' +
        '<div class="chart-section">' +
            '<h4>Storage by Group</h4>' +
            '<canvas id="chart-by-group" class="chart-canvas" width="400" height="250"></canvas>' +
            '<div id="legend-by-group" class="chart-legend"></div>' +
        '</div>' +
        '<div class="chart-section">' +
            '<h4>Storage Growth (90 days)</h4>' +
            '<canvas id="chart-growth" class="chart-canvas" width="400" height="250"></canvas>' +
        '</div>' +
    '</div>';

    html += '</div>'; // close storage section
    html += '</div>'; // close padding wrapper
    container.innerHTML = html;

    // Render charts with empty-state fallback
    var palette = chartPalette();

    if (storage.by_user && storage.by_user.length > 0) {
        try {
            drawBarChart('chart-by-user', 'legend-by-user', storage.by_user.map(function(u) {
                return { label: u.username, value: u.size };
            }), palette);
        } catch (e) {
            showChartEmpty('chart-by-user', 'legend-by-user', 'Error rendering chart');
        }
    } else {
        showChartEmpty('chart-by-user', 'legend-by-user', 'No user storage data');
    }

    if (storage.by_category && storage.by_category.length > 0) {
        try {
            drawDonutChart('chart-by-type', 'legend-by-type', storage.by_category.map(function(c) {
                return { label: c.category, value: c.size };
            }), palette);
        } catch (e) {
            showChartEmpty('chart-by-type', 'legend-by-type', 'Error rendering chart');
        }
    } else {
        showChartEmpty('chart-by-type', 'legend-by-type', 'No file type data');
    }

    if (storage.by_group && storage.by_group.length > 0) {
        try {
            drawBarChart('chart-by-group', 'legend-by-group', storage.by_group.map(function(g) {
                return { label: g.group_name, value: g.size };
            }), palette);
        } catch (e) {
            showChartEmpty('chart-by-group', 'legend-by-group', 'Error rendering chart');
        }
    } else {
        showChartEmpty('chart-by-group', 'legend-by-group', 'No group storage data');
    }

    if (storage.growth && storage.growth.length > 0) {
        try {
            drawLineChart('chart-growth', storage.growth.map(function(p) {
                return { label: p.date.substring(5), value: p.total_size };
            }));
        } catch (e) {
            showChartEmpty('chart-growth', null, 'Error rendering chart');
        }
    } else {
        showChartEmpty('chart-growth', null, 'No growth data yet');
    }
}

// ─── Chart Empty State ────────────────────────────────────────────────────────

function showChartEmpty(canvasId, legendId, message) {
    var canvas = document.getElementById(canvasId);
    if (!canvas) return;
    var placeholder = document.createElement('div');
    placeholder.className = 'chart-empty-state';
    placeholder.textContent = message;
    canvas.parentNode.replaceChild(placeholder, canvas);
    if (legendId) {
        var legend = document.getElementById(legendId);
        if (legend) legend.style.display = 'none';
    }
}

// ─── Chart Helpers ───────────────────────────────────────────────────────────

function chartPalette() {
    return [
        '#2563EB', '#DC2626', '#16A34A', '#F59E0B',
        '#8B5CF6', '#EC4899', '#06B6D4', '#F97316'
    ];
}

function getChartTextColor() {
    return getComputedStyle(document.documentElement).getPropertyValue('--text').trim() || '#0F172A';
}

function getChartBorderColor() {
    return getComputedStyle(document.documentElement).getPropertyValue('--border').trim() || '#E2E8F0';
}

function drawBarChart(canvasId, legendId, items, colors) {
    var canvas = document.getElementById(canvasId);
    if (!canvas) return;
    var ctx = canvas.getContext('2d');
    var dpr = window.devicePixelRatio || 1;
    var w = canvas.clientWidth;
    var h = canvas.clientHeight;
    canvas.width = w * dpr;
    canvas.height = h * dpr;
    ctx.scale(dpr, dpr);

    var textColor = getChartTextColor();
    var borderColor = getChartBorderColor();
    var max = 0;
    for (var i = 0; i < items.length; i++) {
        if (items[i].value > max) max = items[i].value;
    }
    if (max === 0) max = 1;

    var padding = { top: 10, right: 20, bottom: 10, left: 100 };
    var barHeight = Math.min(28, (h - padding.top - padding.bottom) / items.length - 4);
    var chartW = w - padding.left - padding.right;

    ctx.font = '12px -apple-system, sans-serif';
    ctx.textBaseline = 'middle';

    for (var i = 0; i < items.length; i++) {
        var y = padding.top + i * (barHeight + 4);
        var barW = (items[i].value / max) * chartW;

        // Label
        ctx.fillStyle = textColor;
        ctx.textAlign = 'right';
        var label = items[i].label;
        if (label.length > 14) label = label.substring(0, 12) + '..';
        ctx.fillText(label, padding.left - 8, y + barHeight / 2);

        // Bar
        ctx.fillStyle = colors[i % colors.length];
        ctx.beginPath();
        roundRect(ctx, padding.left, y, Math.max(barW, 2), barHeight, 4);
        ctx.fill();

        // Value label
        ctx.fillStyle = textColor;
        ctx.textAlign = 'left';
        ctx.fillText(formatBytes(items[i].value), padding.left + barW + 6, y + barHeight / 2);
    }

    // Legend
    if (legendId) {
        var legendEl = document.getElementById(legendId);
        if (legendEl) {
            var lhtml = '';
            for (var i = 0; i < items.length; i++) {
                lhtml += '<span class="chart-legend-item">' +
                    '<span class="chart-legend-dot" style="background:' + colors[i % colors.length] + '"></span>' +
                    esc(items[i].label) + '</span>';
            }
            legendEl.innerHTML = lhtml;
        }
    }
}

function drawDonutChart(canvasId, legendId, items, colors) {
    var canvas = document.getElementById(canvasId);
    if (!canvas) return;
    var ctx = canvas.getContext('2d');
    var dpr = window.devicePixelRatio || 1;
    var w = canvas.clientWidth;
    var h = canvas.clientHeight;
    canvas.width = w * dpr;
    canvas.height = h * dpr;
    ctx.scale(dpr, dpr);

    var cx = w / 2;
    var cy = h / 2;
    var outerR = Math.min(w, h) / 2 - 10;
    var innerR = outerR * 0.55;

    var total = 0;
    for (var i = 0; i < items.length; i++) total += items[i].value;
    if (total === 0) total = 1;

    var startAngle = -Math.PI / 2;
    for (var i = 0; i < items.length; i++) {
        var sliceAngle = (items[i].value / total) * Math.PI * 2;
        ctx.beginPath();
        ctx.arc(cx, cy, outerR, startAngle, startAngle + sliceAngle);
        ctx.arc(cx, cy, innerR, startAngle + sliceAngle, startAngle, true);
        ctx.closePath();
        ctx.fillStyle = colors[i % colors.length];
        ctx.fill();
        startAngle += sliceAngle;
    }

    // Center text
    var textColor = getChartTextColor();
    ctx.fillStyle = textColor;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.font = 'bold 16px -apple-system, sans-serif';
    ctx.fillText(formatBytes(total), cx, cy - 8);
    ctx.font = '12px -apple-system, sans-serif';
    ctx.fillText('total', cx, cy + 10);

    // Legend
    if (legendId) {
        var legendEl = document.getElementById(legendId);
        if (legendEl) {
            var lhtml = '';
            for (var i = 0; i < items.length; i++) {
                var pct = Math.round((items[i].value / total) * 100);
                lhtml += '<span class="chart-legend-item">' +
                    '<span class="chart-legend-dot" style="background:' + colors[i % colors.length] + '"></span>' +
                    esc(items[i].label) + ' (' + pct + '%)</span>';
            }
            legendEl.innerHTML = lhtml;
        }
    }
}

function drawLineChart(canvasId, points) {
    var canvas = document.getElementById(canvasId);
    if (!canvas) return;
    var ctx = canvas.getContext('2d');
    var dpr = window.devicePixelRatio || 1;
    var w = canvas.clientWidth;
    var h = canvas.clientHeight;
    canvas.width = w * dpr;
    canvas.height = h * dpr;
    ctx.scale(dpr, dpr);

    var textColor = getChartTextColor();
    var borderColor = getChartBorderColor();
    var primaryColor = '#2563EB';

    var padding = { top: 20, right: 20, bottom: 30, left: 60 };
    var chartW = w - padding.left - padding.right;
    var chartH = h - padding.top - padding.bottom;

    var maxVal = 0;
    for (var i = 0; i < points.length; i++) {
        if (points[i].value > maxVal) maxVal = points[i].value;
    }
    if (maxVal === 0) maxVal = 1;

    // Grid lines
    ctx.strokeStyle = borderColor;
    ctx.lineWidth = 0.5;
    for (var g = 0; g <= 4; g++) {
        var gy = padding.top + chartH - (g / 4) * chartH;
        ctx.beginPath();
        ctx.moveTo(padding.left, gy);
        ctx.lineTo(padding.left + chartW, gy);
        ctx.stroke();

        // Y-axis labels
        ctx.fillStyle = textColor;
        ctx.font = '10px -apple-system, sans-serif';
        ctx.textAlign = 'right';
        ctx.textBaseline = 'middle';
        ctx.fillText(formatBytes((g / 4) * maxVal), padding.left - 6, gy);
    }

    if (points.length < 2) return;

    // Area fill
    ctx.beginPath();
    for (var i = 0; i < points.length; i++) {
        var x = padding.left + (i / (points.length - 1)) * chartW;
        var y = padding.top + chartH - (points[i].value / maxVal) * chartH;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
    }
    ctx.lineTo(padding.left + chartW, padding.top + chartH);
    ctx.lineTo(padding.left, padding.top + chartH);
    ctx.closePath();
    ctx.fillStyle = 'rgba(37,99,235,0.1)';
    ctx.fill();

    // Line
    ctx.beginPath();
    for (var i = 0; i < points.length; i++) {
        var x = padding.left + (i / (points.length - 1)) * chartW;
        var y = padding.top + chartH - (points[i].value / maxVal) * chartH;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
    }
    ctx.strokeStyle = primaryColor;
    ctx.lineWidth = 2;
    ctx.stroke();

    // X-axis labels (show every Nth)
    ctx.fillStyle = textColor;
    ctx.font = '10px -apple-system, sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'top';
    var step = Math.max(1, Math.floor(points.length / 6));
    for (var i = 0; i < points.length; i += step) {
        var x = padding.left + (i / (points.length - 1)) * chartW;
        ctx.fillText(points[i].label, x, padding.top + chartH + 6);
    }
}

function roundRect(ctx, x, y, w, h, r) {
    ctx.moveTo(x + r, y);
    ctx.lineTo(x + w - r, y);
    ctx.quadraticCurveTo(x + w, y, x + w, y + r);
    ctx.lineTo(x + w, y + h - r);
    ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
    ctx.lineTo(x + r, y + h);
    ctx.quadraticCurveTo(x, y + h, x, y + h - r);
    ctx.lineTo(x, y + r);
    ctx.quadraticCurveTo(x, y, x + r, y);
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

function metricCard(label, value) {
    return '<div class="quota-card">' +
        '<div class="quota-label">' + esc(label) + '</div>' +
        '<div class="quota-value" style="font-size:1.25rem;font-weight:600;margin-top:0.5rem">' +
            esc(value) +
        '</div>' +
    '</div>';
}

function statCard(label, value) {
    return '<div class="stat-card">' +
        '<div class="stat-label">' + esc(label) + '</div>' +
        '<div class="stat-value">' + esc(String(value)) + '</div>' +
    '</div>';
}

// ─── 2FA / TOTP ──────────────────────────────────────────────────────────────

function loadTOTPStatus() {
    var area = document.getElementById('totp-status-area');
    if (!area) return;

    API.get('/api/v1/auth/totp/status').then(function(data) {
        if (data.enabled) {
            area.innerHTML =
                '<div class="totp-status-row">' +
                    '<span class="totp-badge totp-enabled">2FA Enabled</span>' +
                    '<button class="btn btn-sm btn-outline" id="totp-regen-btn">Regenerate Backup Codes</button> ' +
                    '<button class="btn btn-sm btn-danger" id="totp-disable-btn">Disable 2FA</button>' +
                '</div>';
            document.getElementById('totp-disable-btn').addEventListener('click', showDisableTOTPModal);
            document.getElementById('totp-regen-btn').addEventListener('click', regenerateBackupCodes);
        } else {
            area.innerHTML =
                '<div class="totp-status-row">' +
                    '<span class="totp-badge totp-disabled">2FA Not Enabled</span>' +
                    '<button class="btn btn-sm" id="totp-enable-btn">Enable 2FA</button>' +
                '</div>';
            document.getElementById('totp-enable-btn').addEventListener('click', startTOTPSetup);
        }
    }).catch(function() {
        area.innerHTML = '<span class="text-muted">Unable to load 2FA status</span>';
    });
}

function startTOTPSetup() {
    API.post('/api/v1/auth/totp/setup', {}).then(function(resp) {
        return resp.json();
    }).then(function(data) {
        if (data.error) return;
        showTOTPSetupModal(data);
    });
}

function showTOTPSetupModal(setup) {
    var overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.id = 'totp-modal-overlay';

    var modal = document.createElement('div');
    modal.className = 'modal totp-setup-modal';
    modal.innerHTML =
        '<h3>Enable Two-Factor Authentication</h3>' +
        '<p>Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.):</p>' +
        '<div class="totp-qr-container">' +
            '<img src="data:image/png;base64,' + setup.qr_png + '" alt="QR Code" class="totp-qr-img">' +
        '</div>' +
        '<p class="totp-secret-label">Or enter this secret manually:</p>' +
        '<code class="totp-secret-code">' + esc(setup.secret) + '</code>' +
        '<div class="form-group" style="margin-top:1rem">' +
            '<label for="totp-verify-code">Enter the 6-digit code to verify:</label>' +
            '<input type="text" id="totp-verify-code" class="totp-code-input" inputmode="numeric" maxlength="6" autocomplete="one-time-code">' +
        '</div>' +
        '<div id="totp-setup-error"></div>' +
        '<div class="modal-actions">' +
            '<button class="btn" id="totp-confirm-enable">Enable</button>' +
            '<button class="btn btn-outline" id="totp-cancel-setup">Cancel</button>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) { overlay.remove(); }
    });
    document.getElementById('totp-cancel-setup').addEventListener('click', function() { overlay.remove(); });

    document.getElementById('totp-confirm-enable').addEventListener('click', function() {
        var code = document.getElementById('totp-verify-code').value.trim();
        var errDiv = document.getElementById('totp-setup-error');
        errDiv.innerHTML = '';
        if (!code) return;

        API.post('/api/v1/auth/totp/enable', { secret: setup.secret, code: code }).then(function(resp) {
            return resp.json();
        }).then(function(data) {
            if (data.error) {
                errDiv.innerHTML = '<div class="alert alert-error">' + esc(data.error) + '</div>';
                return;
            }
            overlay.remove();
            showBackupCodesModal(data.backup_codes);
            loadTOTPStatus();
        });
    });
}

function showBackupCodesModal(codes) {
    var overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.id = 'backup-codes-overlay';

    var modal = document.createElement('div');
    modal.className = 'modal';
    var codesHTML = '<div class="backup-codes-grid">';
    for (var i = 0; i < codes.length; i++) {
        codesHTML += '<span class="backup-code">' + esc(codes[i]) + '</span>';
    }
    codesHTML += '</div>';

    modal.innerHTML =
        '<h3>Backup Codes</h3>' +
        '<p class="alert alert-warning">Save these backup codes in a safe place. Each code can only be used once. You will not see them again.</p>' +
        codesHTML +
        '<div class="modal-actions">' +
            '<button class="btn" id="backup-codes-done">I have saved these codes</button>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    document.getElementById('backup-codes-done').addEventListener('click', function() { overlay.remove(); });
}

function showDisableTOTPModal() {
    var overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.id = 'totp-disable-overlay';

    var modal = document.createElement('div');
    modal.className = 'modal';
    modal.innerHTML =
        '<h3>Disable Two-Factor Authentication</h3>' +
        '<p>Enter your password and a current TOTP code to disable 2FA.</p>' +
        '<div class="form-group">' +
            '<label for="totp-disable-pw">Password</label>' +
            '<input type="password" id="totp-disable-pw" autocomplete="current-password">' +
        '</div>' +
        '<div class="form-group">' +
            '<label for="totp-disable-code">TOTP Code</label>' +
            '<input type="text" id="totp-disable-code" class="totp-code-input" inputmode="numeric" maxlength="6">' +
        '</div>' +
        '<div id="totp-disable-error"></div>' +
        '<div class="modal-actions">' +
            '<button class="btn btn-danger" id="totp-confirm-disable">Disable 2FA</button>' +
            '<button class="btn btn-outline" id="totp-cancel-disable">Cancel</button>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) overlay.remove();
    });
    document.getElementById('totp-cancel-disable').addEventListener('click', function() { overlay.remove(); });

    document.getElementById('totp-confirm-disable').addEventListener('click', function() {
        var pw = document.getElementById('totp-disable-pw').value;
        var code = document.getElementById('totp-disable-code').value.trim();
        var errDiv = document.getElementById('totp-disable-error');
        errDiv.innerHTML = '';
        if (!pw || !code) return;

        API.post('/api/v1/auth/totp/disable', { password: pw, code: code }).then(function(resp) {
            return resp.json();
        }).then(function(data) {
            if (data.error) {
                errDiv.innerHTML = '<div class="alert alert-error">' + esc(data.error) + '</div>';
                return;
            }
            overlay.remove();
            loadTOTPStatus();
        });
    });
}

function regenerateBackupCodes() {
    if (!confirm('This will invalidate all existing backup codes. Continue?')) return;

    API.post('/api/v1/auth/totp/backup', {}).then(function(resp) {
        return resp.json();
    }).then(function(data) {
        if (data.backup_codes) {
            showBackupCodesModal(data.backup_codes);
        }
    });
}
