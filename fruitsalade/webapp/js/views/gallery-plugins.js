// Admin gallery plugins management view
function renderGalleryPlugins() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Gallery Plugins</h2>' +
            '<div class="btn-group">' +
                '<button class="btn" id="btn-reprocess-all">Reprocess All</button>' +
                '<button class="btn" id="btn-add-plugin">Add Plugin</button>' +
            '</div>' +
        '</div>' +
        '<div id="plugins-content">Loading...</div>';

    document.getElementById('btn-add-plugin').addEventListener('click', showPluginCreateForm);
    document.getElementById('btn-reprocess-all').addEventListener('click', reprocessAll);

    loadPlugins();
}

function loadPlugins() {
    API.get('/api/v1/admin/gallery/plugins').then(function(plugins) {
        renderPluginsTable(plugins);
    }).catch(function() {
        document.getElementById('plugins-content').innerHTML =
            '<div class="alert alert-error">Failed to load plugins</div>';
    });
}

function renderPluginsTable(plugins) {
    var content = document.getElementById('plugins-content');
    if (!plugins || plugins.length === 0) {
        content.innerHTML = '<p>No gallery plugins configured.</p>';
        return;
    }

    var html = '<div class="table-container"><table class="data-table">' +
        '<thead><tr>' +
            '<th>Name</th>' +
            '<th>Webhook URL</th>' +
            '<th>Enabled</th>' +
            '<th>Last Health</th>' +
            '<th>Last Error</th>' +
            '<th>Actions</th>' +
        '</tr></thead><tbody>';

    for (var i = 0; i < plugins.length; i++) {
        var plugin = plugins[i];
        html += '<tr>' +
            '<td>' + esc(plugin.name) + '</td>' +
            '<td>' + esc(plugin.webhook_url) + '</td>' +
            '<td>' + (plugin.enabled
                ? '<span class="badge badge-green">Enabled</span>'
                : '<span class="badge badge-grey">Disabled</span>') + '</td>' +
            '<td>' + (plugin.last_health ? formatDate(plugin.last_health) : '-') + '</td>' +
            '<td>' + (plugin.last_error ? esc(plugin.last_error) : '-') + '</td>' +
            '<td>' +
                '<div class="btn-group">' +
                    '<button class="btn btn-sm btn-outline" data-action="edit-plugin" data-id="' + plugin.id + '">Edit</button>' +
                    '<button class="btn btn-sm btn-outline" data-action="test-plugin" data-id="' + plugin.id + '">Test</button>' +
                    '<button class="btn btn-sm btn-danger" data-action="delete-plugin" data-id="' + plugin.id + '">Delete</button>' +
                '</div>' +
            '</td>' +
        '</tr>';
    }

    html += '</tbody></table></div>';
    content.innerHTML = html;

    // Wire actions
    content.querySelectorAll('[data-action]').forEach(function(btn) {
        btn.addEventListener('click', function(e) {
            var action = e.currentTarget.getAttribute('data-action');
            var id = parseInt(e.currentTarget.getAttribute('data-id'), 10);
            if (action === 'edit-plugin') {
                showPluginEditForm(id);
            } else if (action === 'test-plugin') {
                testPlugin(id, e.currentTarget);
            } else if (action === 'delete-plugin') {
                deletePlugin(id);
            }
        });
    });
}

// ─── Create / Edit Form ─────────────────────────────────────────────────────

function showPluginCreateForm() {
    var contentDiv = document.createElement('div');
    contentDiv.innerHTML = pluginFormHTML(null);

    Modal.open({
        title: 'Add Plugin',
        content: contentDiv
    });

    wirePluginForm(null);
}

function showPluginEditForm(id) {
    API.get('/api/v1/admin/gallery/plugins/' + id).then(function(plugin) {
        var contentDiv = document.createElement('div');
        contentDiv.innerHTML = pluginFormHTML(plugin);

        Modal.open({
            title: 'Edit Plugin',
            content: contentDiv
        });

        wirePluginForm(plugin);
    }).catch(function() {
        Toast.error('Failed to load plugin');
    });
}

function pluginFormHTML(plugin) {
    var isEdit = !!plugin;
    var name = isEdit ? plugin.name : '';
    var webhookUrl = isEdit ? plugin.webhook_url : '';
    var enabled = isEdit ? plugin.enabled : true;
    var config = '';

    if (isEdit && plugin.config) {
        try {
            config = JSON.stringify(plugin.config, null, 2);
        } catch (e) {
            config = '';
        }
    }

    var html = '<form id="plugin-form">' +
        '<div class="form-group">' +
            '<label for="plugin-name">Name</label>' +
            '<input type="text" id="plugin-name" value="' + esc(name) + '" required>' +
        '</div>' +
        '<div class="form-group">' +
            '<label for="plugin-webhook-url">Webhook URL</label>' +
            '<input type="text" id="plugin-webhook-url" value="' + esc(webhookUrl) + '" required placeholder="https://example.com/tag">' +
        '</div>' +
        '<div class="form-group">' +
            '<label><input type="checkbox" id="plugin-enabled"' + (enabled ? ' checked' : '') + '> Enabled</label>' +
        '</div>' +
        '<div class="form-group">' +
            '<label for="plugin-config">Config (JSON)</label>' +
            '<textarea id="plugin-config" rows="5" placeholder=\'{"key": "value"}\'>' + esc(config) + '</textarea>' +
        '</div>' +
        '<button type="submit" class="btn">' + (isEdit ? 'Update' : 'Create') + '</button>' +
    '</form>';

    return html;
}

function wirePluginForm(existingPlugin) {
    document.getElementById('plugin-form').addEventListener('submit', function(e) {
        e.preventDefault();

        var name = document.getElementById('plugin-name').value.trim();
        if (!name) { Toast.error('Name is required'); return; }

        var webhookUrl = document.getElementById('plugin-webhook-url').value.trim();
        if (!webhookUrl) { Toast.error('Webhook URL is required'); return; }

        var enabled = document.getElementById('plugin-enabled').checked;
        var configStr = document.getElementById('plugin-config').value.trim();
        var config = null;

        if (configStr) {
            try {
                config = JSON.parse(configStr);
            } catch (err) {
                Toast.error('Config must be valid JSON');
                return;
            }
        }

        var body = {
            name: name,
            webhook_url: webhookUrl,
            enabled: enabled,
            config: config
        };

        var promise;
        if (existingPlugin) {
            promise = API.put('/api/v1/admin/gallery/plugins/' + existingPlugin.id, body);
        } else {
            promise = API.post('/api/v1/admin/gallery/plugins', body);
        }

        promise.then(function() {
            Toast.success(existingPlugin ? 'Plugin updated' : 'Plugin created');
            Modal.close();
            loadPlugins();
        }).catch(function() {
            Toast.error('Operation failed');
        });
    });
}

// ─── Actions ────────────────────────────────────────────────────────────────

function testPlugin(id, btn) {
    var origText = btn.textContent;
    btn.textContent = 'Testing...';
    btn.disabled = true;

    API.post('/api/v1/admin/gallery/plugins/' + id + '/test').then(function(data) {
        btn.textContent = origText;
        btn.disabled = false;

        var contentDiv = document.createElement('div');
        var html = '';

        if (data.success) {
            html += '<p><span class="badge badge-green">Success</span></p>';
            if (data.tags && data.tags.length > 0) {
                html += '<p><strong>Returned Tags:</strong></p><ul>';
                for (var i = 0; i < data.tags.length; i++) {
                    html += '<li>' + esc(data.tags[i]) + '</li>';
                }
                html += '</ul>';
            } else {
                html += '<p>No tags returned.</p>';
            }
        } else {
            html += '<p><span class="badge badge-grey">Failed</span></p>';
            if (data.error) {
                html += '<p><strong>Error:</strong> ' + esc(data.error) + '</p>';
            }
        }

        contentDiv.innerHTML = html;

        Modal.open({
            title: 'Plugin Test Result',
            content: contentDiv
        });
    }).catch(function() {
        btn.textContent = origText;
        btn.disabled = false;
        Toast.error('Test request failed');
    });
}

function deletePlugin(id) {
    if (!confirm('Delete this plugin? This cannot be undone.')) {
        return;
    }

    API.del('/api/v1/admin/gallery/plugins/' + id).then(function() {
        Toast.success('Plugin deleted');
        loadPlugins();
    }).catch(function() {
        Toast.error('Failed to delete plugin');
    });
}

function reprocessAll() {
    if (!confirm('Reprocess all files with gallery plugins? This may take a while.')) {
        return;
    }

    API.post('/api/v1/admin/gallery/reprocess').then(function(data) {
        Toast.success(data.message || 'Reprocessing started');
    }).catch(function() {
        Toast.error('Failed to start reprocessing');
    });
}
