// Notification Center — SSE-powered real-time notifications
var Notifications = (function() {
    var eventSource = null;
    var notifications = [];
    var maxNotifications = 50;
    var unreadCount = 0;
    var panelOpen = false;

    // ── SSE Connection ────────────────────────────────────────────────────────

    function connect() {
        disconnect();
        var token = API.getToken();
        if (!token) return;

        try {
            eventSource = new EventSource('/api/v1/events?token=' + encodeURIComponent(token));

            eventSource.addEventListener('file_created', function(e) {
                onEvent('create', JSON.parse(e.data));
            });
            eventSource.addEventListener('file_modified', function(e) {
                onEvent('modify', JSON.parse(e.data));
            });
            eventSource.addEventListener('file_deleted', function(e) {
                onEvent('delete', JSON.parse(e.data));
            });
            eventSource.addEventListener('file_versioned', function(e) {
                onEvent('version', JSON.parse(e.data));
            });
            // Generic message fallback
            eventSource.onmessage = function(e) {
                try {
                    var data = JSON.parse(e.data);
                    var type = data.type || 'modify';
                    onEvent(type, data);
                } catch(_) {}
            };
            eventSource.onerror = function() {
                // EventSource auto-reconnects; nothing to do
            };
        } catch(_) {}
    }

    function disconnect() {
        if (eventSource) {
            eventSource.close();
            eventSource = null;
        }
    }

    // ── Event Handling ────────────────────────────────────────────────────────

    function onEvent(type, data) {
        var n = {
            id: Date.now() + '-' + Math.random().toString(36).substr(2, 6),
            type: type,
            path: data.path || data.file_path || '',
            filename: extractFilename(data.path || data.file_path || ''),
            user: data.user || data.username || '',
            time: new Date(),
            read: false,
            data: data
        };

        notifications.unshift(n);
        if (notifications.length > maxNotifications) {
            notifications = notifications.slice(0, maxNotifications);
        }
        unreadCount++;
        updateBadge();
        showNotificationToast(n);
        if (panelOpen) renderPanel();
    }

    function extractFilename(path) {
        if (!path) return 'Unknown file';
        var parts = path.split('/');
        return parts[parts.length - 1] || path;
    }

    // ── Toasts ────────────────────────────────────────────────────────────────

    function showNotificationToast(n) {
        var container = document.getElementById('notif-toast-container');
        if (!container) {
            container = document.createElement('div');
            container.id = 'notif-toast-container';
            document.body.appendChild(container);
        }

        // Limit visible toasts
        var existing = container.querySelectorAll('.notif-toast');
        if (existing.length >= 3) {
            dismissToast(existing[0]);
        }

        var toast = document.createElement('div');
        toast.className = 'notif-toast notif-toast-' + n.type;
        toast.setAttribute('role', 'status');
        toast.setAttribute('aria-live', 'polite');
        toast.innerHTML =
            '<span class="notif-toast-icon">' + eventIcon(n.type) + '</span>' +
            '<div class="notif-toast-body">' +
                '<div class="notif-toast-title">' + esc(eventLabel(n.type)) + '</div>' +
                '<div class="notif-toast-text">' + esc(n.filename) + '</div>' +
            '</div>' +
            '<button class="notif-toast-close" aria-label="Dismiss">&times;</button>';

        container.appendChild(toast);
        requestAnimationFrame(function() { toast.classList.add('notif-toast-visible'); });

        var timer = setTimeout(function() { dismissToast(toast); }, 5000);

        toast.querySelector('.notif-toast-close').addEventListener('click', function() {
            clearTimeout(timer);
            dismissToast(toast);
        });

        // Click toast to navigate
        toast.addEventListener('click', function(e) {
            if (e.target.classList.contains('notif-toast-close')) return;
            clearTimeout(timer);
            dismissToast(toast);
            navigateToFile(n);
        });
    }

    function dismissToast(toast) {
        toast.classList.remove('notif-toast-visible');
        toast.classList.add('notif-toast-out');
        setTimeout(function() { if (toast.parentNode) toast.parentNode.removeChild(toast); }, 300);
    }

    // ── Panel ─────────────────────────────────────────────────────────────────

    function togglePanel() {
        if (panelOpen) {
            closePanel();
        } else {
            openPanel();
        }
    }

    function openPanel() {
        panelOpen = true;
        var panel = document.getElementById('notif-panel');
        var btn = document.getElementById('notif-bell-btn');
        if (panel) {
            panel.classList.remove('hidden');
            renderPanel();
        }
        if (btn) btn.setAttribute('aria-expanded', 'true');
    }

    function closePanel() {
        panelOpen = false;
        var panel = document.getElementById('notif-panel');
        var btn = document.getElementById('notif-bell-btn');
        if (panel) panel.classList.add('hidden');
        if (btn) btn.setAttribute('aria-expanded', 'false');
    }

    function renderPanel() {
        var panel = document.getElementById('notif-panel');
        if (!panel) return;

        if (notifications.length === 0) {
            panel.innerHTML =
                '<div class="notif-panel-header">' +
                    '<span class="notif-panel-title">Notifications</span>' +
                '</div>' +
                '<div class="notif-panel-empty">No notifications yet</div>';
            return;
        }

        var html =
            '<div class="notif-panel-header">' +
                '<span class="notif-panel-title">Notifications</span>' +
                '<div class="notif-panel-actions">' +
                    '<button class="notif-panel-action" id="notif-mark-read">Mark all read</button>' +
                    '<button class="notif-panel-action" id="notif-clear-all">Clear all</button>' +
                '</div>' +
            '</div>' +
            '<div class="notif-panel-list">';

        for (var i = 0; i < notifications.length; i++) {
            var n = notifications[i];
            html += '<div class="notif-item' + (n.read ? '' : ' unread') + '" data-notif-idx="' + i + '">' +
                '<span class="notif-item-icon notif-icon-' + n.type + '">' + eventIcon(n.type) + '</span>' +
                '<div class="notif-item-body">' +
                    '<div class="notif-item-title">' + esc(eventLabel(n.type)) + '</div>' +
                    '<div class="notif-item-text">' + esc(n.filename) + '</div>' +
                    '<div class="notif-item-time">' + relativeTime(n.time) + '</div>' +
                '</div>' +
            '</div>';
        }

        html += '</div>';
        panel.innerHTML = html;

        // Wire actions
        var markBtn = document.getElementById('notif-mark-read');
        if (markBtn) markBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            markAllRead();
        });

        var clearBtn = document.getElementById('notif-clear-all');
        if (clearBtn) clearBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            clearAll();
        });

        // Wire notification item clicks
        panel.querySelectorAll('.notif-item').forEach(function(item) {
            item.addEventListener('click', function() {
                var idx = parseInt(item.getAttribute('data-notif-idx'), 10);
                var n = notifications[idx];
                if (n) {
                    if (!n.read) {
                        n.read = true;
                        unreadCount = Math.max(0, unreadCount - 1);
                        updateBadge();
                    }
                    closePanel();
                    navigateToFile(n);
                }
            });
        });
    }

    function markAllRead() {
        for (var i = 0; i < notifications.length; i++) {
            notifications[i].read = true;
        }
        unreadCount = 0;
        updateBadge();
        renderPanel();
    }

    function clearAll() {
        notifications = [];
        unreadCount = 0;
        updateBadge();
        renderPanel();
    }

    // ── Badge ─────────────────────────────────────────────────────────────────

    function updateBadge() {
        var badge = document.getElementById('notif-badge');
        if (!badge) return;
        if (unreadCount > 0) {
            badge.textContent = unreadCount > 99 ? '99+' : unreadCount;
            badge.classList.remove('hidden');
        } else {
            badge.classList.add('hidden');
        }
    }

    // ── Navigation ────────────────────────────────────────────────────────────

    function navigateToFile(n) {
        if (!n.path) return;
        if (n.type === 'delete') {
            window.location.hash = '#trash';
        } else {
            // Check if directory by trailing slash or type hint
            var isDir = n.path.endsWith('/') || (n.data && n.data.is_dir);
            if (isDir) {
                window.location.hash = '#browser' + n.path;
            } else {
                window.location.hash = '#viewer' + n.path;
            }
        }
    }

    // ── Helpers ───────────────────────────────────────────────────────────────

    function relativeTime(date) {
        var now = new Date();
        var diff = Math.floor((now - date) / 1000);
        if (diff < 5) return 'just now';
        if (diff < 60) return diff + 's ago';
        if (diff < 3600) return Math.floor(diff / 60) + 'm ago';
        if (diff < 86400) return Math.floor(diff / 3600) + 'h ago';
        return Math.floor(diff / 86400) + 'd ago';
    }

    function eventIcon(type) {
        switch (type) {
            case 'create': return '&#10010;';
            case 'modify': return '&#9998;';
            case 'delete': return '&#128465;';
            case 'version': return '&#128338;';
            default: return '&#128276;';
        }
    }

    function eventLabel(type) {
        switch (type) {
            case 'create': return 'File Created';
            case 'modify': return 'File Modified';
            case 'delete': return 'File Deleted';
            case 'version': return 'New Version';
            default: return 'Notification';
        }
    }

    return {
        connect: connect,
        disconnect: disconnect,
        togglePanel: togglePanel,
        closePanel: closePanel,
        markAllRead: markAllRead,
        clearAll: clearAll
    };
})();
