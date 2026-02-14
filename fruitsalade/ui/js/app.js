// SPA hash router
(function() {
    var routes = {
        'login':     renderLogin,
        'dashboard': renderDashboard,
        'users':     renderUsers,
        'files':     renderFiles,
        'shares':    renderShares
    };

    function getRoute() {
        var hash = window.location.hash.replace('#', '') || 'dashboard';
        // Handle sub-routes like #files/docs/readme.md
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

        // Show/hide nav
        var nav = document.getElementById('nav');
        if (route === 'login') {
            nav.classList.add('hidden');
        } else {
            nav.classList.remove('hidden');
            document.getElementById('nav-username').textContent =
                sessionStorage.getItem('username') || '';
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

        // Render view
        var handler = routes[route];
        if (handler) {
            handler();
        } else {
            document.getElementById('app').innerHTML =
                '<div class="alert alert-error">Page not found</div>';
        }
    }

    window.addEventListener('hashchange', navigate);

    // Logout
    document.getElementById('btn-logout').addEventListener('click', function() {
        API.clearToken();
        window.location.hash = '#login';
    });

    // Initial route
    navigate();
})();
