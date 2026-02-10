// API client with Bearer token auth
var API = (function() {
    function getToken() {
        return sessionStorage.getItem('token');
    }

    function setToken(token) {
        sessionStorage.setItem('token', token);
    }

    function clearToken() {
        sessionStorage.removeItem('token');
        sessionStorage.removeItem('username');
        sessionStorage.removeItem('is_admin');
    }

    function isAuthenticated() {
        return !!getToken();
    }

    function request(method, path, body, rawBody) {
        var opts = {
            method: method,
            headers: {}
        };

        var token = getToken();
        if (token) {
            opts.headers['Authorization'] = 'Bearer ' + token;
        }

        if (rawBody) {
            // For file uploads — send body directly
            opts.body = rawBody;
        } else if (body !== undefined) {
            opts.headers['Content-Type'] = 'application/json';
            opts.body = JSON.stringify(body);
        }

        return fetch(path, opts).then(function(resp) {
            if (resp.status === 401) {
                clearToken();
                window.location.hash = '#login';
                return Promise.reject(new Error('Unauthorized'));
            }
            return resp;
        });
    }

    function get(path) {
        return request('GET', path).then(function(r) { return r.json(); });
    }

    function post(path, body) {
        return request('POST', path, body);
    }

    function put(path, body) {
        return request('PUT', path, body);
    }

    function del(path) {
        return request('DELETE', path);
    }

    // Upload a file to a path (raw binary POST)
    function upload(path, file) {
        return request('POST', '/api/v1/content/' + encodeURIPath(path), undefined, file);
    }

    // Get download URL with auth token
    function downloadUrl(path) {
        return '/api/v1/content/' + encodeURIPath(path) + '?token=' + encodeURIComponent(getToken());
    }

    // Encode path segments individually (preserving slashes)
    function encodeURIPath(p) {
        return p.split('/').map(function(seg) {
            return encodeURIComponent(seg);
        }).join('/');
    }

    return {
        getToken: getToken,
        setToken: setToken,
        clearToken: clearToken,
        isAuthenticated: isAuthenticated,
        request: request,
        get: get,
        post: post,
        put: put,
        del: del,
        upload: upload,
        downloadUrl: downloadUrl,
        encodeURIPath: encodeURIPath
    };
})();

// HTML escape helper to prevent XSS
function esc(str) {
    var div = document.createElement('div');
    div.appendChild(document.createTextNode(str == null ? '' : String(str)));
    return div.innerHTML;
}

// Format bytes to human-readable
function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    var units = ['B', 'KB', 'MB', 'GB', 'TB'];
    var i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
}

// Format date
function formatDate(dateStr) {
    if (!dateStr) return '-';
    var d = new Date(dateStr);
    return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'});
}
