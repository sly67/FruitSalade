function renderFiles() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar"><h2>Files</h2></div>' +
        '<div id="files-breadcrumb"></div>' +
        '<div id="files-table">Loading...</div>';

    var hash = window.location.hash;
    var path = '';
    if (hash.indexOf('#files/') === 0) {
        path = hash.substring(7);
    }

    loadFileTree(path);
}

function loadFileTree(path) {
    var apiPath = path ? '/api/v1/tree/' + path : '/api/v1/tree';

    // Build breadcrumb
    var bc = '<div class="breadcrumb"><a href="#files">root</a>';
    if (path) {
        var parts = path.split('/');
        var cumulative = '';
        for (var i = 0; i < parts.length; i++) {
            cumulative += (i > 0 ? '/' : '') + parts[i];
            bc += '<span class="sep">/</span><a href="#files/' + esc(cumulative) + '">' + esc(parts[i]) + '</a>';
        }
    }
    bc += '</div>';
    document.getElementById('files-breadcrumb').innerHTML = bc;

    API.get(apiPath).then(function(data) {
        var root = data.root;
        if (!root) {
            document.getElementById('files-table').innerHTML = '<p>No files found.</p>';
            return;
        }

        var children = root.children || [];
        if (children.length === 0) {
            document.getElementById('files-table').innerHTML = '<p>Empty directory.</p>';
            return;
        }

        // Sort: directories first, then by name
        children.sort(function(a, b) {
            if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
            return a.name.localeCompare(b.name);
        });

        var rows = '';
        for (var i = 0; i < children.length; i++) {
            var f = children[i];
            var nameCell;
            if (f.is_dir) {
                var childPath = path ? path + '/' + f.name : f.name;
                nameCell = '<a href="#files/' + esc(childPath) + '">' + esc(f.name) + '/</a>';
            } else {
                nameCell = esc(f.name);
            }

            rows += '<tr>' +
                '<td>' + nameCell + '</td>' +
                '<td>' + (f.is_dir ? '<span class="badge badge-blue">Dir</span>' : formatBytes(f.size)) + '</td>' +
                '<td>' + (f.version || '-') + '</td>' +
                '<td>' + formatDate(f.mod_time) + '</td>' +
            '</tr>';
        }

        document.getElementById('files-table').innerHTML =
            '<div class="table-wrap"><table>' +
                '<thead><tr><th>Name</th><th>Size</th><th>Version</th><th>Modified</th></tr></thead>' +
                '<tbody>' + rows + '</tbody>' +
            '</table></div>';
    }).catch(function() {
        document.getElementById('files-table').innerHTML =
            '<div class="alert alert-error">Failed to load files</div>';
    });
}
