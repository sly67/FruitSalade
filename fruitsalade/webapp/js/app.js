// SPA hash router
(function() {
    var routes = {
        'login':            renderLogin,
        'browser':          renderBrowser,
        'versions':         renderVersions,
        'shares':           renderShares,
        'viewer':           renderViewer,
        'dashboard':        renderDashboard,
        'users':            renderUsers,
        'admin-shares':     renderAdminShares,
        'groups':           renderGroups,
        'settings':         renderSettings,
        'storage':          renderStorage,
        'gallery':          renderGallery,
        'gallery-plugins':  renderGalleryPlugins,
        'share':            renderShareDownload,
        'trash':            renderTrash,
        'search':           renderSearch,
        'favorites':        renderFavorites
    };

    // Admin-only routes
    var adminRoutes = ['users', 'groups', 'admin-shares', 'settings', 'storage', 'gallery-plugins'];

    // Mobile state
    var isMobile = false;
    var moreMenuOpen = false;

    function isAdmin() {
        return sessionStorage.getItem('is_admin') === 'true';
    }

    function getRoute() {
        var hash = window.location.hash.replace('#', '') || 'browser';
        var base = hash.split('/')[0];
        return base;
    }

    function navigate() {
        var route = getRoute();

        // Auth guard
        if (route !== 'login' && route !== 'share' && !API.isAuthenticated()) {
            window.location.hash = '#login';
            return;
        }

        // Admin guard
        if (adminRoutes.indexOf(route) !== -1 && !isAdmin()) {
            window.location.hash = '#browser';
            return;
        }

        // Close more menu and detail panel on navigate
        closeMoreMenu();
        var detailPanel = document.getElementById('detail-panel');
        if (detailPanel) {
            detailPanel.classList.remove('open');
            detailPanel.classList.add('hidden');
            detailPanel.innerHTML = '';
        }

        // Show/hide topbar, layout, tab bar
        var topbar = document.getElementById('topbar');
        var layout = document.getElementById('layout');
        var tabBar = document.getElementById('tab-bar');
        if (route === 'login') {
            topbar.classList.add('hidden');
            layout.classList.add('hidden');
            tabBar.classList.add('hidden');
            document.getElementById('login-container').classList.remove('hidden');
        } else if (route === 'share') {
            topbar.classList.add('hidden');
            tabBar.classList.add('hidden');
            var loginContainer = document.getElementById('login-container');
            if (loginContainer) loginContainer.classList.add('hidden');
            layout.classList.remove('hidden');
            document.getElementById('sidebar').classList.add('hidden');
            var dp = document.getElementById('detail-panel');
            if (dp) { dp.classList.add('hidden'); dp.innerHTML = ''; }
            var treeArea = document.getElementById('sidebar-tree');
            if (treeArea) treeArea.classList.add('hidden');
        } else {
            topbar.classList.remove('hidden');
            layout.classList.remove('hidden');
            document.getElementById('sidebar').classList.remove('hidden');
            if (isMobile) {
                tabBar.classList.remove('hidden');
            }
            var loginContainer = document.getElementById('login-container');
            if (loginContainer) loginContainer.classList.add('hidden');

            document.getElementById('topbar-username').textContent =
                sessionStorage.getItem('username') || '';

            // Show/hide admin sidebar section
            var adminSection = document.getElementById('sidebar-admin');
            if (adminSection) {
                if (isAdmin()) {
                    adminSection.classList.remove('hidden');
                } else {
                    adminSection.classList.add('hidden');
                }
            }

            // Show sidebar tree on file-related routes
            var treeArea = document.getElementById('sidebar-tree');
            var treeRoutes = ['browser', 'viewer', 'versions'];
            if (treeRoutes.indexOf(route) !== -1) {
                treeArea.classList.remove('hidden');
                // On mobile, sidebar starts closed
                if (isMobile) {
                    var sidebar = document.getElementById('sidebar');
                    sidebar.classList.remove('mobile-open');
                    closeSidebarBackdrop();
                }
                TreeView.renderTree();
            } else {
                treeArea.classList.add('hidden');
                closeSidebarBackdrop();
            }
        }

        // Highlight active sidebar-nav link
        var navLinks = document.querySelectorAll('.sidebar-nav a[data-route]');
        for (var i = 0; i < navLinks.length; i++) {
            var linkRoute = navLinks[i].getAttribute('data-route');
            if (linkRoute === route) {
                navLinks[i].classList.add('active');
            } else {
                navLinks[i].classList.remove('active');
            }
        }

        // Highlight active tab-bar link
        updateTabBar(route);

        // Update tree highlight
        if (route === 'browser' || route === 'viewer') {
            var hash = window.location.hash.replace('#' + route, '').replace(/^\//, '');
            var path = hash ? '/' + decodeURIComponent(hash) : '/';
            TreeView.setCurrentPath(path);
        }

        // Render view
        var handler = routes[route];
        if (handler) {
            handler();
        } else {
            document.getElementById('app').innerHTML =
                '<div class="alert alert-error">Page not found</div>';
        }
    }

    // ── Tab Bar ─────────────────────────────────────────────────────────────

    function updateTabBar(route) {
        var tabLinks = document.querySelectorAll('#tab-bar a[data-route]');
        for (var i = 0; i < tabLinks.length; i++) {
            var tr = tabLinks[i].getAttribute('data-route');
            if (tr === route) {
                tabLinks[i].classList.add('active');
            } else {
                tabLinks[i].classList.remove('active');
            }
        }
        // Highlight More tab if current route is an admin/more route
        var moreTab = document.getElementById('tab-more');
        var moreRoutes = ['shares', 'trash', 'favorites', 'search', 'users', 'groups', 'storage', 'gallery-plugins', 'admin-shares', 'settings'];
        if (moreRoutes.indexOf(route) !== -1) {
            moreTab.classList.add('active');
        } else {
            moreTab.classList.remove('active');
        }
    }

    // ── More Menu ───────────────────────────────────────────────────────────

    function openMoreMenu() {
        if (moreMenuOpen) { closeMoreMenu(); return; }
        moreMenuOpen = true;

        var overlay = document.createElement('div');
        overlay.className = 'more-menu-overlay';
        overlay.id = 'more-menu-overlay';
        overlay.addEventListener('click', closeMoreMenu);

        var menu = document.createElement('div');
        menu.className = 'more-menu';
        menu.id = 'more-menu';

        var items = [
            { route: 'favorites', icon: '&#11088;', label: 'Favorites' },
            { route: 'shares', icon: '&#128279;', label: 'My Shares' },
            { route: 'trash', icon: '&#128465;', label: 'Trash' },
            { route: 'search', icon: '&#128269;', label: 'Search' }
        ];

        if (isAdmin()) {
            items.push({ route: 'users', icon: '&#128100;', label: 'Users' });
            items.push({ route: 'groups', icon: '&#128101;', label: 'Groups' });
            items.push({ route: 'storage', icon: '&#128190;', label: 'Storage' });
            items.push({ route: 'gallery-plugins', icon: '&#127912;', label: 'Gallery Plugins' });
            items.push({ route: 'admin-shares', icon: '&#128279;', label: 'Admin Shares' });
            items.push({ route: 'settings', icon: '&#9881;', label: 'Settings' });
        }

        var html = '';
        for (var i = 0; i < items.length; i++) {
            html += '<a href="#' + items[i].route + '"><span class="more-icon">' +
                items[i].icon + '</span>' + items[i].label + '</a>';
        }
        html += '<a href="#" id="more-menu-logout"><span class="more-icon">&#128682;</span>Logout</a>';
        menu.innerHTML = html;

        document.body.appendChild(overlay);
        document.body.appendChild(menu);

        document.getElementById('more-menu-logout').addEventListener('click', function(e) {
            e.preventDefault();
            closeMoreMenu();
            API.clearToken();
            window.location.hash = '#login';
        });
    }

    function closeMoreMenu() {
        moreMenuOpen = false;
        var overlay = document.getElementById('more-menu-overlay');
        var menu = document.getElementById('more-menu');
        if (overlay) overlay.remove();
        if (menu) menu.remove();
    }

    document.getElementById('tab-more').addEventListener('click', function(e) {
        e.preventDefault();
        openMoreMenu();
    });

    // ── User Dropdown ───────────────────────────────────────────────────────

    var userMenuBtn = document.getElementById('user-menu-btn');
    var userDropdown = document.getElementById('user-dropdown');

    userMenuBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        userDropdown.classList.toggle('hidden');
    });

    document.addEventListener('click', function(e) {
        if (!userDropdown.classList.contains('hidden') && !userDropdown.contains(e.target)) {
            userDropdown.classList.add('hidden');
        }
    });

    document.getElementById('topbar-logout').addEventListener('click', function(e) {
        e.preventDefault();
        userDropdown.classList.add('hidden');
        API.clearToken();
        window.location.hash = '#login';
    });

    // ── Dark Mode ───────────────────────────────────────────────────────────

    function initTheme() {
        var saved = localStorage.getItem('theme');
        if (saved) {
            document.documentElement.setAttribute('data-theme', saved);
        } else if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
            document.documentElement.setAttribute('data-theme', 'dark');
        }
        updateThemeLabel();
    }

    function toggleTheme() {
        var current = document.documentElement.getAttribute('data-theme');
        var next = current === 'dark' ? 'light' : 'dark';
        document.documentElement.setAttribute('data-theme', next);
        localStorage.setItem('theme', next);
        updateThemeLabel();
    }

    function updateThemeLabel() {
        var btn = document.getElementById('theme-toggle');
        var isDark = document.documentElement.getAttribute('data-theme') === 'dark';
        if (btn) {
            btn.innerHTML = '<span class="dropdown-icon">' + (isDark ? '&#9728;' : '&#127769;') + '</span> ' +
                (isDark ? 'Light Mode' : 'Dark Mode');
        }
    }

    document.getElementById('theme-toggle').addEventListener('click', function(e) {
        e.preventDefault();
        toggleTheme();
        userDropdown.classList.add('hidden');
    });

    initTheme();

    // ── Global Search ────────────────────────────────────────────────────────

    var globalSearchInput = document.getElementById('global-search');
    var globalSearchTimer = null;

    globalSearchInput.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
            e.preventDefault();
            var q = globalSearchInput.value.trim();
            if (!q) return;
            window.location.hash = '#search/' + encodeURIComponent(q);
            globalSearchInput.blur();
        }
    });

    // Ctrl+K / Cmd+K shortcut to focus search
    document.addEventListener('keydown', function(e) {
        if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
            e.preventDefault();
            if (API.isAuthenticated()) {
                window.location.hash = '#search';
            }
        }
    });

    // ── Sidebar Overlay (mobile) ────────────────────────────────────────────

    function openSidebarMobile() {
        var sidebar = document.getElementById('sidebar');
        sidebar.classList.add('mobile-open');

        // Create backdrop
        if (!document.querySelector('.sidebar-backdrop')) {
            var backdrop = document.createElement('div');
            backdrop.className = 'sidebar-backdrop';
            backdrop.id = 'sidebar-backdrop';
            backdrop.addEventListener('click', closeSidebarMobile);
            document.body.appendChild(backdrop);
        }
    }

    function closeSidebarMobile() {
        var sidebar = document.getElementById('sidebar');
        sidebar.classList.remove('mobile-open');
        closeSidebarBackdrop();
    }

    function closeSidebarBackdrop() {
        var backdrop = document.getElementById('sidebar-backdrop');
        if (backdrop) backdrop.remove();
    }

    // ── Media Query Listener ────────────────────────────────────────────────

    var mql = window.matchMedia('(max-width: 768px)');

    function onMediaChange(e) {
        isMobile = e.matches;
        var tabBar = document.getElementById('tab-bar');
        var route = getRoute();

        if (isMobile) {
            // Show tab bar (unless on login)
            if (route !== 'login') {
                tabBar.classList.remove('hidden');
            }
            // Close sidebar on switch to mobile
            var sidebar = document.getElementById('sidebar');
            sidebar.classList.remove('mobile-open');
            closeSidebarBackdrop();
        } else {
            // Desktop: hide tab bar, close more menu
            tabBar.classList.add('hidden');
            closeMoreMenu();
            closeSidebarBackdrop();
            // Reset sidebar for desktop
            var sidebar = document.getElementById('sidebar');
            sidebar.classList.remove('mobile-open');
        }
    }

    // Initial check
    isMobile = mql.matches;
    mql.addEventListener('change', onMediaChange);

    // ── Service Worker / PWA ─────────────────────────────────────────────────

    var deferredInstallPrompt = null;

    if ('serviceWorker' in navigator) {
        navigator.serviceWorker.register('service-worker.js', { scope: '/app/' })
            .catch(function(err) {
                console.warn('SW registration failed:', err);
            });
    }

    window.addEventListener('beforeinstallprompt', function(e) {
        e.preventDefault();
        deferredInstallPrompt = e;
        // Show install banner in topbar
        var existing = document.getElementById('pwa-install-btn');
        if (!existing) {
            var btn = document.createElement('button');
            btn.id = 'pwa-install-btn';
            btn.className = 'btn btn-sm pwa-install-btn';
            btn.textContent = 'Install App';
            btn.addEventListener('click', function() {
                if (deferredInstallPrompt) {
                    deferredInstallPrompt.prompt();
                    deferredInstallPrompt.userChoice.then(function() {
                        deferredInstallPrompt = null;
                        btn.remove();
                    });
                }
            });
            var topbarUser = document.querySelector('.topbar-user');
            if (topbarUser) {
                topbarUser.parentNode.insertBefore(btn, topbarUser);
            }
        }
    });

    window.addEventListener('appinstalled', function() {
        deferredInstallPrompt = null;
        var btn = document.getElementById('pwa-install-btn');
        if (btn) btn.remove();
    });

    // ── Event Listeners ─────────────────────────────────────────────────────

    window.addEventListener('hashchange', navigate);

    // Initial route
    navigate();
})();
