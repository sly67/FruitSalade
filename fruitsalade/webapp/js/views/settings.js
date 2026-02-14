// Server config viewer/editor (admin-only)
function renderSettings() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar"><h2>Server Settings</h2></div>' +
        '<div id="settings-content">Loading...</div>';

    API.get('/api/v1/admin/config').then(function(data) {
        renderConfigSections(data);
    }).catch(function() {
        document.getElementById('settings-content').innerHTML =
            '<div class="alert alert-error">Failed to load configuration</div>';
    });
}

function renderConfigSections(data) {
    var content = document.getElementById('settings-content');
    var html = '';

    // Read-only sections
    html += configCard('Server', [
        { label: 'Listen Address', value: data.server.listen_addr },
        { label: 'Metrics Address', value: data.server.metrics_addr }
    ]);

    html += configCard('Storage', [
        { label: 'S3 Endpoint', value: data.storage.s3_endpoint },
        { label: 'S3 Bucket', value: data.storage.s3_bucket },
        { label: 'S3 Region', value: data.storage.s3_region },
        { label: 'S3 SSL', value: data.storage.s3_use_ssl ? 'Yes' : 'No' }
    ]);

    html += configCard('Database', [
        { label: 'Connected', value: data.database.connected ? 'Yes' : 'No' }
    ]);

    html += configCard('Authentication', [
        { label: 'JWT Configured', value: data.auth.jwt_configured ? 'Yes' : 'No' },
        { label: 'OIDC Issuer', value: data.auth.oidc_issuer || 'Not configured' }
    ]);

    html += configCard('TLS', [
        { label: 'Enabled', value: data.tls.enabled ? 'Yes' : 'No' },
        { label: 'Certificate', value: data.tls.cert_file || 'N/A' }
    ]);

    // Editable runtime section
    var rt = data.runtime;
    html += '<div class="settings-card editable">' +
        '<h3>Runtime Settings</h3>' +
        '<p class="settings-hint">These settings can be changed at runtime.</p>' +
        '<form id="runtime-form">' +
            '<div class="form-group">' +
                '<label for="cfg-log-level">Log Level</label>' +
                '<select id="cfg-log-level" class="settings-select">' +
                    logLevelOption('debug', rt.log_level) +
                    logLevelOption('info', rt.log_level) +
                    logLevelOption('warn', rt.log_level) +
                    logLevelOption('error', rt.log_level) +
                '</select>' +
            '</div>' +
            '<div class="form-group">' +
                '<label for="cfg-max-upload">Max Upload Size (bytes)</label>' +
                '<input type="number" id="cfg-max-upload" value="' + rt.max_upload_size + '">' +
            '</div>' +
            '<div class="form-group">' +
                '<label for="cfg-max-storage">Default Max Storage (bytes, 0 = unlimited)</label>' +
                '<input type="number" id="cfg-max-storage" value="' + rt.default_max_storage + '">' +
            '</div>' +
            '<div class="form-group">' +
                '<label for="cfg-max-bandwidth">Default Max Bandwidth/Day (bytes, 0 = unlimited)</label>' +
                '<input type="number" id="cfg-max-bandwidth" value="' + rt.default_max_bandwidth + '">' +
            '</div>' +
            '<div class="form-group">' +
                '<label for="cfg-rpm">Default Requests Per Minute (0 = unlimited)</label>' +
                '<input type="number" id="cfg-rpm" value="' + rt.default_requests_per_min + '">' +
            '</div>' +
            '<button type="submit" class="btn">Save Changes</button>' +
        '</form>' +
    '</div>';

    content.innerHTML = html;

    // Wire form submit
    document.getElementById('runtime-form').addEventListener('submit', function(e) {
        e.preventDefault();
        var updates = {
            log_level: document.getElementById('cfg-log-level').value,
            max_upload_size: parseInt(document.getElementById('cfg-max-upload').value, 10) || 0,
            default_max_storage: parseInt(document.getElementById('cfg-max-storage').value, 10) || 0,
            default_max_bandwidth: parseInt(document.getElementById('cfg-max-bandwidth').value, 10) || 0,
            default_requests_per_min: parseInt(document.getElementById('cfg-rpm').value, 10) || 0
        };

        API.put('/api/v1/admin/config', updates).then(function(resp) {
            return resp.json().then(function(data) {
                if (resp.ok) {
                    Toast.success('Settings saved successfully');
                } else {
                    Toast.error(data.error || 'Failed to save settings');
                }
            });
        }).catch(function() {
            Toast.error('Failed to save settings');
        });
    });
}

function configCard(title, items) {
    var html = '<div class="settings-card">' +
        '<h3>' + esc(title) + '</h3>' +
        '<div class="settings-items">';
    for (var i = 0; i < items.length; i++) {
        html += '<div class="settings-item">' +
            '<span class="settings-label">' + esc(items[i].label) + '</span>' +
            '<span class="settings-value">' + esc(String(items[i].value)) + '</span>' +
        '</div>';
    }
    html += '</div></div>';
    return html;
}

function logLevelOption(level, current) {
    return '<option value="' + level + '"' + (current === level ? ' selected' : '') + '>' +
        level.charAt(0).toUpperCase() + level.slice(1) + '</option>';
}

