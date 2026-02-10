// File viewer/editor component
function renderViewer() {
    var hash = window.location.hash.replace('#viewer', '').replace(/^\//, '');
    if (!hash) {
        document.getElementById('app').innerHTML =
            '<div class="alert alert-error">No file path specified.</div>';
        return;
    }

    var filePath = '/' + decodeURIComponent(hash);
    var fileName = filePath.split('/').pop();
    var ext = (fileName.indexOf('.') !== -1) ? '.' + fileName.split('.').pop().toLowerCase() : '';
    var fileType = detectFileType(ext);

    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="viewer-toolbar">' +
            '<a href="#browser' + esc(filePath.substring(0, filePath.lastIndexOf('/'))) + '" class="btn btn-sm btn-outline">Back</a>' +
            '<span class="viewer-filename">' + esc(fileName) + '</span>' +
            '<div class="viewer-actions" id="viewer-actions"></div>' +
        '</div>' +
        '<div class="viewer-meta" id="viewer-meta"></div>' +
        '<div class="viewer-content" id="viewer-content">Loading...</div>';

    // Load metadata for info bar
    var apiPath = '/api/v1/tree/' + API.encodeURIPath(filePath.replace(/^\//, ''));
    API.get(apiPath).then(function(data) {
        var node = data.root;
        if (node) {
            var meta = document.getElementById('viewer-meta');
            meta.innerHTML =
                '<span>Size: ' + formatBytes(node.size) + '</span>' +
                '<span>Version: v' + (node.version || 1) + '</span>' +
                '<span>Modified: ' + formatDate(node.mod_time) + '</span>';
        }
    }).catch(function() {});

    if (fileType === 'text') {
        renderTextViewer(filePath);
    } else if (fileType === 'image') {
        renderImageViewer(filePath);
    } else if (fileType === 'pdf') {
        renderPdfViewer(filePath);
    } else {
        renderGenericViewer(filePath, fileName);
    }
}

function detectFileType(ext) {
    var textExts = ['.txt', '.md', '.json', '.js', '.css', '.html', '.py', '.go', '.sh',
        '.yml', '.yaml', '.xml', '.csv', '.log', '.ini', '.conf', '.toml', '.env',
        '.sql', '.rs', '.ts', '.tsx', '.jsx', '.c', '.cpp', '.h', '.java', '.rb',
        '.php', '.lua', '.makefile', '.dockerfile', '.gitignore', '.mod', '.sum',
        '.cfg', '.properties', '.bat', '.ps1', '.r', '.swift', '.kt', '.scala'];
    var imageExts = ['.png', '.jpg', '.jpeg', '.gif', '.svg', '.webp', '.bmp', '.ico'];
    var pdfExts = ['.pdf'];

    if (textExts.indexOf(ext) !== -1 || ext === '') return 'text';
    if (imageExts.indexOf(ext) !== -1) return 'image';
    if (pdfExts.indexOf(ext) !== -1) return 'pdf';
    return 'other';
}

function renderTextViewer(filePath) {
    var content = document.getElementById('viewer-content');
    var actions = document.getElementById('viewer-actions');
    var isEditing = false;
    var originalText = '';

    actions.innerHTML =
        '<button class="btn btn-sm" id="btn-edit">Edit</button>' +
        '<a class="btn btn-sm btn-outline" href="' + esc(API.downloadUrl(filePath.replace(/^\//, ''))) + '" download>Download</a>';

    // Fetch text content
    API.request('GET', '/api/v1/content/' + API.encodeURIPath(filePath.replace(/^\//, '')))
        .then(function(resp) { return resp.text(); })
        .then(function(text) {
            originalText = text;
            content.innerHTML = '<pre class="viewer-pre">' + esc(text) + '</pre>';

            document.getElementById('btn-edit').addEventListener('click', function() {
                if (!isEditing) {
                    enterEditMode(text);
                }
            });
        })
        .catch(function() {
            content.innerHTML = '<div class="alert alert-error">Failed to load file content</div>';
        });

    function enterEditMode(text) {
        isEditing = true;
        content.innerHTML = '<textarea class="viewer-textarea" id="editor-textarea">' +
            esc(text) + '</textarea>';
        actions.innerHTML =
            '<button class="btn btn-sm btn-success" id="btn-save">Save</button>' +
            '<button class="btn btn-sm btn-outline" id="btn-cancel">Cancel</button>' +
            '<a class="btn btn-sm btn-outline" href="' + esc(API.downloadUrl(filePath.replace(/^\//, ''))) + '" download>Download</a>';

        document.getElementById('btn-save').addEventListener('click', function() {
            var newText = document.getElementById('editor-textarea').value;
            saveFile(filePath, newText);
        });

        document.getElementById('btn-cancel').addEventListener('click', function() {
            exitEditMode(originalText);
        });
    }

    function exitEditMode(text) {
        isEditing = false;
        content.innerHTML = '<pre class="viewer-pre">' + esc(text) + '</pre>';
        actions.innerHTML =
            '<button class="btn btn-sm" id="btn-edit">Edit</button>' +
            '<a class="btn btn-sm btn-outline" href="' + esc(API.downloadUrl(filePath.replace(/^\//, ''))) + '" download>Download</a>';

        document.getElementById('btn-edit').addEventListener('click', function() {
            enterEditMode(text);
        });
    }

    function saveFile(path, text) {
        var blob = new Blob([text], { type: 'text/plain' });
        API.upload(path.replace(/^\//, ''), blob)
            .then(function(resp) {
                if (resp.ok) {
                    originalText = text;
                    exitEditMode(text);
                    // Refresh metadata
                    API.get('/api/v1/tree/' + API.encodeURIPath(path.replace(/^\//, ''))).then(function(data) {
                        var node = data.root;
                        if (node) {
                            var meta = document.getElementById('viewer-meta');
                            meta.innerHTML =
                                '<span>Size: ' + formatBytes(node.size) + '</span>' +
                                '<span>Version: v' + (node.version || 1) + '</span>' +
                                '<span>Modified: ' + formatDate(node.mod_time) + '</span>';
                        }
                    });
                    TreeView.refresh();
                } else {
                    resp.json().then(function(d) { alert(d.error || 'Save failed'); });
                }
            })
            .catch(function() { alert('Save failed'); });
    }
}

function renderImageViewer(filePath) {
    var content = document.getElementById('viewer-content');
    var actions = document.getElementById('viewer-actions');
    actions.innerHTML =
        '<a class="btn btn-sm btn-outline" href="' + esc(API.downloadUrl(filePath.replace(/^\//, ''))) + '" download>Download</a>';
    content.innerHTML =
        '<div class="viewer-image-wrap">' +
            '<img class="viewer-image" src="' + esc(API.downloadUrl(filePath.replace(/^\//, ''))) + '" alt="' + esc(filePath) + '">' +
        '</div>';
}

function renderPdfViewer(filePath) {
    var content = document.getElementById('viewer-content');
    var actions = document.getElementById('viewer-actions');
    actions.innerHTML =
        '<a class="btn btn-sm btn-outline" href="' + esc(API.downloadUrl(filePath.replace(/^\//, ''))) + '" download>Download</a>';
    content.innerHTML =
        '<iframe class="viewer-pdf" src="' + esc(API.downloadUrl(filePath.replace(/^\//, ''))) + '"></iframe>';
}

function renderGenericViewer(filePath, fileName) {
    var content = document.getElementById('viewer-content');
    var actions = document.getElementById('viewer-actions');
    actions.innerHTML =
        '<a class="btn btn-sm" href="' + esc(API.downloadUrl(filePath.replace(/^\//, ''))) + '" download>Download</a>';
    content.innerHTML =
        '<div class="viewer-generic">' +
            '<p>Preview not available for this file type.</p>' +
            '<p>File: <strong>' + esc(fileName) + '</strong></p>' +
        '</div>';
}
