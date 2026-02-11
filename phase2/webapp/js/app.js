// SPA hash router
(function() {
    var routes = {
        'login':        renderLogin,
        'browser':      renderBrowser,
        'versions':     renderVersions,
        'shares':       renderShares,
        'viewer':       renderViewer,
        'dashboard':    renderDashboard,
        'users':        renderUsers,
        'admin-shares': renderAdminShares,
        'groups':       renderGroups,
        'settings':     renderSettings
    };

    // Admin-only routes
    var adminRoutes = ['users', 'groups', 'admin-shares', 'settings'];

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
        if (route !== 'login' && !API.isAuthenticated()) {
            window.location.hash = '#login';
            return;
        }

        // Admin guard
        if (adminRoutes.indexOf(route) !== -1 && !isAdmin()) {
            window.location.hash = '#browser';
            return;
        }

        // Close more menu on navigate
        closeMoreMenu();

        // Show/hide nav and sidebar
        var nav = document.getElementById('nav');
        var layout = document.getElementById('layout');
        var tabBar = document.getElementById('tab-bar');
        if (route === 'login') {
            nav.classList.add('hidden');
            layout.classList.add('hidden');
            tabBar.classList.add('hidden');
            document.getElementById('login-container').classList.remove('hidden');
        } else {
            nav.classList.remove('hidden');
            layout.classList.remove('hidden');
            if (isMobile) {
                tabBar.classList.remove('hidden');
            }
            var loginContainer = document.getElementById('login-container');
            if (loginContainer) loginContainer.classList.add('hidden');

            document.getElementById('nav-username').textContent =
                sessionStorage.getItem('username') || '';

            // Update nav links visibility based on admin status
            updateNavLinks();

            // Show sidebar tree only on file-related routes
            var sidebar = document.getElementById('sidebar');
            var treeRoutes = ['browser', 'viewer', 'versions'];
            if (treeRoutes.indexOf(route) !== -1) {
                sidebar.classList.remove('hidden');
                // On mobile, sidebar starts closed
                if (isMobile) {
                    sidebar.classList.remove('mobile-open');
                    closeSidebarBackdrop();
                }
                TreeView.renderTree();
            } else {
                sidebar.classList.add('hidden');
                closeSidebarBackdrop();
            }
        }

        // Highlight active nav link (top nav)
        var links = document.querySelectorAll('.nav-links a');
        for (var i = 0; i < links.length; i++) {
            var linkRoute = links[i].getAttribute('data-route');
            if (linkRoute === route) {
                links[i].classList.add('active');
            } else {
                links[i].classList.remove('active');
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

    function updateNavLinks() {
        var adminLinks = document.querySelectorAll('.nav-links [data-admin]');
        for (var i = 0; i < adminLinks.length; i++) {
            if (isAdmin()) {
                adminLinks[i].classList.remove('hidden');
            } else {
                adminLinks[i].classList.add('hidden');
            }
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
        var moreRoutes = ['shares', 'users', 'groups', 'admin-shares', 'settings'];
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
            { route: 'shares', icon: '&#128279;', label: 'My Shares' }
        ];

        if (isAdmin()) {
            items.push({ route: 'users', icon: '&#128100;', label: 'Users' });
            items.push({ route: 'groups', icon: '&#128101;', label: 'Groups' });
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

    // ── Sidebar Overlay (mobile) ────────────────────────────────────────────

    function openSidebarMobile() {
        var sidebar = document.getElementById('sidebar');
        if (sidebar.classList.contains('hidden')) return;
        sidebar.classList.add('mobile-open');
        sidebar.classList.remove('collapsed');

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

    // ── Swipe Gestures ──────────────────────────────────────────────────────

    (function() {
        var touchStartX = 0;
        var touchStartY = 0;
        var tracking = false;
        var swipeType = ''; // 'open' or 'close'

        var layout = document.getElementById('layout');

        layout.addEventListener('touchstart', function(e) {
            if (!isMobile) return;
            var touch = e.touches[0];
            touchStartX = touch.clientX;
            touchStartY = touch.clientY;

            var sidebar = document.getElementById('sidebar');
            if (sidebar.classList.contains('hidden')) return;

            if (touchStartX < 30) {
                // Near left edge — potential open swipe
                swipeType = 'open';
                tracking = true;
            } else if (sidebar.classList.contains('mobile-open')) {
                // Sidebar is open — potential close swipe
                swipeType = 'close';
                tracking = true;
            } else {
                tracking = false;
            }
        }, { passive: true });

        layout.addEventListener('touchend', function(e) {
            if (!tracking || !isMobile) return;
            tracking = false;

            var touch = e.changedTouches[0];
            var dx = touch.clientX - touchStartX;
            var dy = Math.abs(touch.clientY - touchStartY);

            // Must swipe at least 50px horizontal with max 30px vertical drift
            if (dy > 30) return;

            if (swipeType === 'open' && dx > 50) {
                openSidebarMobile();
            } else if (swipeType === 'close' && dx < -50) {
                closeSidebarMobile();
            }
        }, { passive: true });
    })();

    // ── Pull-to-Refresh ─────────────────────────────────────────────────────

    (function() {
        var appEl = document.getElementById('app');
        var pulling = false;
        var pullStartY = 0;
        var pullDist = 0;
        var indicator = null;
        var threshold = 60;

        function ensureIndicator() {
            if (!indicator) {
                indicator = document.createElement('div');
                indicator.className = 'pull-indicator';
                indicator.textContent = 'Pull to refresh';
                appEl.appendChild(indicator);
            }
            return indicator;
        }

        appEl.addEventListener('touchstart', function(e) {
            if (!('ontouchstart' in window)) return;
            if (appEl.scrollTop !== 0) return;
            pullStartY = e.touches[0].clientY;
            pulling = true;
            pullDist = 0;
        }, { passive: true });

        appEl.addEventListener('touchmove', function(e) {
            if (!pulling) return;
            var currentY = e.touches[0].clientY;
            pullDist = currentY - pullStartY;
            if (pullDist < 0) { pulling = false; return; }

            var ind = ensureIndicator();
            if (pullDist > threshold) {
                ind.textContent = 'Release to refresh';
            } else {
                ind.textContent = 'Pull to refresh';
            }
            if (pullDist > 10) {
                ind.classList.add('visible');
            }
        }, { passive: true });

        appEl.addEventListener('touchend', function() {
            if (!pulling) return;
            pulling = false;

            if (indicator) {
                indicator.classList.remove('visible');
            }

            if (pullDist > threshold) {
                // Show spinner briefly
                var ind = ensureIndicator();
                ind.textContent = 'Refreshing...';
                ind.classList.add('visible');
                setTimeout(function() {
                    ind.classList.remove('visible');
                    ind.textContent = '';
                }, 600);
                navigate();
            }
            pullDist = 0;
        }, { passive: true });
    })();

    // ── Event Listeners ─────────────────────────────────────────────────────

    window.addEventListener('hashchange', navigate);

    // Logout
    document.getElementById('btn-logout').addEventListener('click', function() {
        API.clearToken();
        window.location.hash = '#login';
    });

    // Sidebar toggle
    var toggleBtn = document.getElementById('sidebar-toggle');
    if (toggleBtn) {
        toggleBtn.addEventListener('click', function() {
            var sidebar = document.getElementById('sidebar');
            if (isMobile) {
                if (sidebar.classList.contains('mobile-open')) {
                    closeSidebarMobile();
                } else {
                    openSidebarMobile();
                }
            } else {
                sidebar.classList.toggle('collapsed');
            }
        });
    }

    // Initial route
    navigate();
})();
