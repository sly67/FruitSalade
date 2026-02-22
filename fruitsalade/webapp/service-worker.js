// FruitSalade Service Worker — offline-first caching
var CACHE_VERSION = 'fs-v1';
var SHELL_CACHE = 'app-shell-' + CACHE_VERSION;
var CDN_CACHE = 'cdn-' + CACHE_VERSION;
var API_CACHE = 'api-' + CACHE_VERSION;
var CONTENT_CACHE = 'content-' + CACHE_VERSION;

var ALL_CACHES = [SHELL_CACHE, CDN_CACHE, API_CACHE, CONTENT_CACHE];

// App shell resources to pre-cache on install
var SHELL_URLS = [
    '/app/',
    '/app/index.html',
    '/app/css/style.css',
    '/app/manifest.json',
    '/app/js/api.js',
    '/app/js/utils.js',
    '/app/js/app.js',
    '/app/js/views/login.js',
    '/app/js/views/tree.js',
    '/app/js/views/browser.js',
    '/app/js/views/viewer.js',
    '/app/js/views/versions.js',
    '/app/js/views/shares.js',
    '/app/js/views/dashboard.js',
    '/app/js/views/users.js',
    '/app/js/views/admin-shares.js',
    '/app/js/views/groups.js',
    '/app/js/views/storage.js',
    '/app/js/views/gallery.js',
    '/app/js/views/gallery-plugins.js',
    '/app/js/views/settings.js',
    '/app/js/views/trash.js',
    '/app/js/views/search.js',
    '/app/js/views/share-download.js',
    '/app/js/views/map.js'
];

var OFFLINE_HTML = '<!DOCTYPE html><html><head><meta charset="UTF-8">' +
    '<meta name="viewport" content="width=device-width,initial-scale=1">' +
    '<title>FruitSalade — Offline</title>' +
    '<style>body{font-family:-apple-system,sans-serif;display:flex;align-items:center;' +
    'justify-content:center;min-height:100vh;margin:0;background:#F8FAFC;color:#0F172A;text-align:center}' +
    '.box{padding:2rem}h1{font-size:1.5rem;margin-bottom:.5rem}p{color:#475569}</style></head>' +
    '<body><div class="box"><h1>You are offline</h1>' +
    '<p>FruitSalade requires a network connection for this action.<br>' +
    'Cached pages and files remain available.</p></div></body></html>';

// ─── Install ───────────────────────────────────────────────────────────────

self.addEventListener('install', function(event) {
    event.waitUntil(
        caches.open(SHELL_CACHE).then(function(cache) {
            return cache.addAll(SHELL_URLS);
        }).then(function() {
            return self.skipWaiting();
        })
    );
});

// ─── Activate ──────────────────────────────────────────────────────────────

self.addEventListener('activate', function(event) {
    event.waitUntil(
        caches.keys().then(function(keys) {
            return Promise.all(
                keys.filter(function(key) {
                    return ALL_CACHES.indexOf(key) === -1;
                }).map(function(key) {
                    return caches.delete(key);
                })
            );
        }).then(function() {
            return self.clients.claim();
        })
    );
});

// ─── Fetch ─────────────────────────────────────────────────────────────────

self.addEventListener('fetch', function(event) {
    var url = new URL(event.request.url);
    var method = event.request.method;

    // Only handle GET requests for caching
    if (method !== 'GET') return;

    // Auth endpoints — network-only
    if (url.pathname.indexOf('/api/v1/auth/') !== -1) return;

    // CDN resources (Leaflet etc.) — cache-first
    if (url.hostname === 'unpkg.com') {
        event.respondWith(cacheFirst(event.request, CDN_CACHE));
        return;
    }

    // App shell assets — cache-first
    if (url.pathname.indexOf('/app/') === 0 &&
        (url.pathname.indexOf('/css/') !== -1 ||
         url.pathname.indexOf('/js/') !== -1 ||
         url.pathname.indexOf('/icons/') !== -1 ||
         url.pathname.endsWith('/index.html') ||
         url.pathname.endsWith('/manifest.json') ||
         url.pathname.endsWith('/service-worker.js') ||
         url.pathname === '/app/')) {
        event.respondWith(cacheFirst(event.request, SHELL_CACHE));
        return;
    }

    // Content downloads (immutable by hash) — cache-first
    if (url.pathname.indexOf('/api/v1/content/') === 0 ||
        url.pathname.indexOf('/api/v1/gallery/thumb/') === 0) {
        event.respondWith(cacheFirst(event.request, CONTENT_CACHE));
        return;
    }

    // API GET metadata — network-first with cache fallback
    if (url.pathname.indexOf('/api/v1/') === 0) {
        event.respondWith(networkFirst(event.request, API_CACHE));
        return;
    }
});

// ─── Strategies ────────────────────────────────────────────────────────────

function cacheFirst(request, cacheName) {
    return caches.open(cacheName).then(function(cache) {
        return cache.match(request).then(function(cached) {
            if (cached) return cached;
            return fetch(request).then(function(response) {
                if (response.ok) {
                    cache.put(request, response.clone());
                }
                return response;
            }).catch(function() {
                return offlineFallback(request);
            });
        });
    });
}

function networkFirst(request, cacheName) {
    return caches.open(cacheName).then(function(cache) {
        return fetch(request).then(function(response) {
            if (response.ok) {
                cache.put(request, response.clone());
            }
            return response;
        }).catch(function() {
            return cache.match(request).then(function(cached) {
                return cached || offlineFallback(request);
            });
        });
    });
}

function offlineFallback(request) {
    var url = new URL(request.url);
    // For navigation requests, return offline HTML
    if (request.mode === 'navigate' || url.pathname.indexOf('/app/') === 0) {
        return new Response(OFFLINE_HTML, {
            headers: { 'Content-Type': 'text/html' }
        });
    }
    return new Response('Offline', { status: 503 });
}
