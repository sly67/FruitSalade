// Admin storage locations management view
function renderStorage() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Storage Locations</h2>' +
            '<button class="btn" id="btn-add-storage">Add Storage Location</button>' +
        '</div>' +
        '<div id="storage-content">Loading...</div>';

    document.getElementById('btn-add-storage').addEventListener('click', showStorageCreateForm);

    loadStorageLocations();
}

function loadStorageLocations() {
    API.get('/api/v1/admin/storage').then(function(locations) {
        renderStorageTable(locations);
    }).catch(function() {
        document.getElementById('storage-content').innerHTML =
            '<div class="alert alert-error">Failed to load storage locations</div>';
    });
}

function renderStorageTable(locations) {
    var content = document.getElementById('storage-content');
    if (!locations || locations.length === 0) {
        content.innerHTML = '<p>No storage locations configured.</p>';
        return;
    }

    var html = '<div class="table-container"><table class="data-table">' +
        '<thead><tr>' +
            '<th>Name</th>' +
            '<th>Type</th>' +
            '<th>Group</th>' +
            '<th>Priority</th>' +
            '<th>Default</th>' +
            '<th>Files</th>' +
            '<th>Actions</th>' +
        '</tr></thead><tbody>';

    for (var i = 0; i < locations.length; i++) {
        var loc = locations[i];
        html += '<tr>' +
            '<td>' + esc(loc.name) + '</td>' +
            '<td><span class="badge badge-' + backendBadgeColor(loc.backend_type) + '">' +
                esc(loc.backend_type.toUpperCase()) + '</span></td>' +
            '<td>' + (loc.group_id ? '<span class="badge badge-blue">Group #' + loc.group_id + '</span>' : '-') + '</td>' +
            '<td>' + loc.priority + '</td>' +
            '<td>' + (loc.is_default ? '<span class="badge badge-green">Default</span>' : '-') + '</td>' +
            '<td id="storage-stats-' + loc.id + '">...</td>' +
            '<td>' +
                '<div class="btn-group">' +
                    '<button class="btn btn-sm btn-outline" data-action="edit-storage" data-id="' + loc.id + '">Edit</button>' +
                    '<button class="btn btn-sm btn-outline" data-action="test-storage" data-id="' + loc.id + '">Test</button>' +
                    (!loc.is_default ? '<button class="btn btn-sm btn-outline" data-action="set-default" data-id="' + loc.id + '">Set Default</button>' : '') +
                    '<button class="btn btn-sm btn-danger" data-action="delete-storage" data-id="' + loc.id + '">Delete</button>' +
                '</div>' +
            '</td>' +
        '</tr>';
    }

    html += '</tbody></table></div>';
    content.innerHTML = html;

    // Load stats for each location
    for (var j = 0; j < locations.length; j++) {
        loadStorageStats(locations[j].id);
    }

    // Wire actions
    content.querySelectorAll('[data-action]').forEach(function(btn) {
        btn.addEventListener('click', function(e) {
            var action = e.currentTarget.getAttribute('data-action');
            var id = parseInt(e.currentTarget.getAttribute('data-id'), 10);
            if (action === 'edit-storage') {
                showStorageEditForm(id);
            } else if (action === 'test-storage') {
                testStorageLocation(id, e.currentTarget);
            } else if (action === 'set-default') {
                setDefaultStorage(id);
            } else if (action === 'delete-storage') {
                deleteStorageLocation(id);
            }
        });
    });
}

function loadStorageStats(locationID) {
    API.get('/api/v1/admin/storage/' + locationID + '/stats').then(function(stats) {
        var el = document.getElementById('storage-stats-' + locationID);
        if (el) {
            el.textContent = stats.file_count + ' files (' + formatBytes(stats.total_size) + ')';
        }
    }).catch(function() {
        var el = document.getElementById('storage-stats-' + locationID);
        if (el) el.textContent = '-';
    });
}

function backendBadgeColor(type) {
    switch (type) {
        case 's3': return 'blue';
        case 'local': return 'green';
        case 'smb': return 'yellow';
        default: return 'grey';
    }
}

// ─── Create Form ────────────────────────────────────────────────────────────

function showStorageCreateForm() {
    var contentDiv = document.createElement('div');
    contentDiv.innerHTML = storageFormHTML(null);

    Modal.open({
        title: 'Add Storage Location',
        content: contentDiv,
        className: 'storage-modal'
    });

    wireStorageForm(null);
}

function showStorageEditForm(id) {
    API.get('/api/v1/admin/storage/' + id).then(function(loc) {
        var contentDiv = document.createElement('div');
        contentDiv.innerHTML = storageFormHTML(loc);

        Modal.open({
            title: 'Edit Storage Location',
            content: contentDiv,
            className: 'storage-modal'
        });

        wireStorageForm(loc);
    }).catch(function() {
        Toast.error('Failed to load storage location');
    });
}

function storageFormHTML(loc) {
    var isEdit = !!loc;
    var name = isEdit ? loc.name : '';
    var backendType = isEdit ? loc.backend_type : 's3';
    var priority = isEdit ? loc.priority : 0;
    var groupID = isEdit && loc.group_id ? loc.group_id : '';
    var config = isEdit && loc.config ? loc.config : {};

    var html = '<form id="storage-form">' +
        '<div class="form-group">' +
            '<label for="storage-name">Name</label>' +
            '<input type="text" id="storage-name" value="' + esc(name) + '" required>' +
        '</div>' +
        '<div class="form-group">' +
            '<label for="storage-type">Backend Type</label>' +
            '<select id="storage-type"' + (isEdit ? ' disabled' : '') + '>' +
                '<option value="s3"' + (backendType === 's3' ? ' selected' : '') + '>S3 / MinIO</option>' +
                '<option value="local"' + (backendType === 'local' ? ' selected' : '') + '>Local Filesystem</option>' +
                '<option value="smb"' + (backendType === 'smb' ? ' selected' : '') + '>SMB Network Share</option>' +
            '</select>' +
        '</div>' +
        '<div class="form-group">' +
            '<label for="storage-group">Group (optional, assign to root group)</label>' +
            '<select id="storage-group">' +
                '<option value="">None (available for all)</option>' +
            '</select>' +
        '</div>' +
        '<div class="form-group">' +
            '<label for="storage-priority">Priority (higher = preferred)</label>' +
            '<input type="number" id="storage-priority" value="' + priority + '">' +
        '</div>' +
        '<div id="storage-config-fields"></div>' +
        '<button type="submit" class="btn">' + (isEdit ? 'Update' : 'Create') + '</button>' +
    '</form>';

    return html;
}

function wireStorageForm(existingLoc) {
    var typeSelect = document.getElementById('storage-type');
    var configArea = document.getElementById('storage-config-fields');
    var groupSelect = document.getElementById('storage-group');

    // Load groups
    API.get('/api/v1/admin/groups').then(function(groups) {
        if (groups) {
            for (var i = 0; i < groups.length; i++) {
                if (!groups[i].parent_id) {
                    var opt = document.createElement('option');
                    opt.value = groups[i].id;
                    opt.textContent = groups[i].name;
                    if (existingLoc && existingLoc.group_id === groups[i].id) {
                        opt.selected = true;
                    }
                    groupSelect.appendChild(opt);
                }
            }
        }
    }).catch(function() {});

    // Render config fields for current type
    var currentType = typeSelect.value;
    var currentConfig = existingLoc && existingLoc.config ? existingLoc.config : {};
    renderConfigFields(configArea, currentType, currentConfig);

    // Update config fields when type changes
    typeSelect.addEventListener('change', function() {
        renderConfigFields(configArea, typeSelect.value, {});
    });

    // Form submit
    document.getElementById('storage-form').addEventListener('submit', function(e) {
        e.preventDefault();

        var name = document.getElementById('storage-name').value.trim();
        if (!name) { Toast.error('Name is required'); return; }

        var type = typeSelect.value;
        var groupVal = groupSelect.value;
        var priority = parseInt(document.getElementById('storage-priority').value, 10) || 0;

        var config = collectConfigValues(type);

        var body = {
            name: name,
            backend_type: type,
            config: config,
            priority: priority
        };
        if (groupVal) {
            body.group_id = parseInt(groupVal, 10);
        }

        var promise;
        if (existingLoc) {
            promise = API.put('/api/v1/admin/storage/' + existingLoc.id, body);
        } else {
            promise = API.post('/api/v1/admin/storage', body);
        }

        promise.then(function(resp) {
            return resp.json().then(function(data) {
                if (resp.ok || resp.status === 201) {
                    Toast.success(existingLoc ? 'Storage location updated' : 'Storage location created');
                    Modal.close();
                    loadStorageLocations();
                } else {
                    Toast.error(data.error || 'Operation failed');
                }
            });
        }).catch(function() {
            Toast.error('Operation failed');
        });
    });
}

function renderConfigFields(container, backendType, config) {
    var html = '<div class="config-section"><h4>Backend Configuration</h4>';

    switch (backendType) {
        case 's3':
            html += configField('endpoint', 'Endpoint', config.endpoint || '', 'text', 'e.g. localhost:9000');
            html += configField('bucket', 'Bucket', config.bucket || '', 'text', 'e.g. fruitsalade');
            html += configField('access_key', 'Access Key', config.access_key || '', 'text');
            html += configField('secret_key', 'Secret Key',
                config.secret_key === '***' ? '' : (config.secret_key || ''), 'password',
                config.secret_key === '***' ? 'Leave blank to keep current' : '');
            html += configField('region', 'Region', config.region || 'us-east-1', 'text');
            html += '<div class="form-group">' +
                '<label><input type="checkbox" id="cfg-use_ssl"' +
                (config.use_ssl ? ' checked' : '') + '> Use SSL</label></div>';
            break;

        case 'local':
            html += configField('root_path', 'Root Path', config.root_path || '', 'text', 'e.g. /data/storage');
            html += '<div class="form-group">' +
                '<label><input type="checkbox" id="cfg-create_dirs"' +
                (config.create_dirs !== false ? ' checked' : '') + '> Auto-create directories</label></div>';
            break;

        case 'smb':
            html += configField('server', 'SMB Server', config.server || '', 'text', 'e.g. //fileserver/share');
            html += configField('username', 'Username', config.username || '', 'text');
            html += configField('password', 'Password',
                config.password === '***' ? '' : (config.password || ''), 'password',
                config.password === '***' ? 'Leave blank to keep current' : '');
            html += configField('domain', 'Domain', config.domain || '', 'text');
            html += configField('mount_path', 'Mount Path', config.mount_path || '', 'text',
                'Local path where the share is mounted');
            break;
    }

    html += '</div>';
    container.innerHTML = html;
}

function configField(id, label, value, type, placeholder) {
    return '<div class="form-group">' +
        '<label for="cfg-' + id + '">' + esc(label) + '</label>' +
        '<input type="' + (type || 'text') + '" id="cfg-' + id + '" value="' + esc(value) + '"' +
        (placeholder ? ' placeholder="' + esc(placeholder) + '"' : '') + '>' +
    '</div>';
}

function collectConfigValues(backendType) {
    var config = {};

    switch (backendType) {
        case 's3':
            config.endpoint = document.getElementById('cfg-endpoint').value.trim();
            config.bucket = document.getElementById('cfg-bucket').value.trim();
            config.access_key = document.getElementById('cfg-access_key').value.trim();
            var secretVal = document.getElementById('cfg-secret_key').value.trim();
            if (secretVal) config.secret_key = secretVal;
            config.region = document.getElementById('cfg-region').value.trim() || 'us-east-1';
            config.use_ssl = document.getElementById('cfg-use_ssl').checked;
            break;

        case 'local':
            config.root_path = document.getElementById('cfg-root_path').value.trim();
            config.create_dirs = document.getElementById('cfg-create_dirs').checked;
            break;

        case 'smb':
            config.server = document.getElementById('cfg-server').value.trim();
            config.username = document.getElementById('cfg-username').value.trim();
            var passVal = document.getElementById('cfg-password').value.trim();
            if (passVal) config.password = passVal;
            config.domain = document.getElementById('cfg-domain').value.trim();
            config.mount_path = document.getElementById('cfg-mount_path').value.trim();
            break;
    }

    return config;
}

// ─── Actions ────────────────────────────────────────────────────────────────

function testStorageLocation(id, btn) {
    var origText = btn.textContent;
    btn.textContent = 'Testing...';
    btn.disabled = true;

    API.post('/api/v1/admin/storage/' + id + '/test').then(function(resp) {
        return resp.json().then(function(data) {
            btn.textContent = origText;
            btn.disabled = false;
            if (data.success) {
                Toast.success('Connection test passed!');
            } else {
                Toast.error('Test failed: ' + (data.error || 'Unknown error'));
            }
        });
    }).catch(function() {
        btn.textContent = origText;
        btn.disabled = false;
        Toast.error('Test request failed');
    });
}

function setDefaultStorage(id) {
    API.post('/api/v1/admin/storage/' + id + '/default').then(function(resp) {
        return resp.json().then(function(data) {
            if (resp.ok) {
                Toast.success('Default storage location updated');
                loadStorageLocations();
            } else {
                Toast.error(data.error || 'Failed to set default');
            }
        });
    }).catch(function() {
        Toast.error('Failed to set default');
    });
}

function deleteStorageLocation(id) {
    if (!confirm('Delete this storage location? This cannot be undone. Files using this location must be migrated first.')) {
        return;
    }

    API.del('/api/v1/admin/storage/' + id).then(function(resp) {
        return resp.json().then(function(data) {
            if (resp.ok) {
                Toast.success('Storage location deleted');
                loadStorageLocations();
            } else {
                Toast.error(data.error || 'Failed to delete');
            }
        });
    }).catch(function() {
        Toast.error('Failed to delete');
    });
}
