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

        // Show/hide nav and sidebar
        var nav = document.getElementById('nav');
        var layout = document.getElementById('layout');
        if (route === 'login') {
            nav.classList.add('hidden');
            layout.classList.add('hidden');
            document.getElementById('login-container').classList.remove('hidden');
        } else {
            nav.classList.remove('hidden');
            layout.classList.remove('hidden');
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
                TreeView.renderTree();
            } else {
                sidebar.classList.add('hidden');
            }
        }

        // Highlight active nav link
        var links = document.querySelectorAll('.nav-links a');
        for (var i = 0; i < links.length; i++) {
            var linkRoute = links[i].getAttribute('data-route');
            if (linkRoute === route) {
                links[i].classList.add('active');
            } else {
                links[i].classList.remove('active');
            }
        }

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
            sidebar.classList.toggle('collapsed');
        });
    }

    // Initial route
    navigate();
})();
