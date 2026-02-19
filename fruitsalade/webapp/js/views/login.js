function renderLogin() {
    var container = document.getElementById('login-container');
    container.innerHTML =
        '<div class="login-wrap">' +
            '<div class="form-card login-card">' +
                '<h1>FruitSalade</h1>' +
                '<p class="login-tagline">Self-hosted file sync</p>' +
                '<div id="login-error"></div>' +
                '<form id="login-form">' +
                    '<div class="form-group">' +
                        '<label for="username">Username</label>' +
                        '<input type="text" id="username" autocomplete="username" required>' +
                    '</div>' +
                    '<div class="form-group">' +
                        '<label for="password">Password</label>' +
                        '<div class="password-wrap">' +
                            '<input type="password" id="password" autocomplete="current-password" required>' +
                            '<button type="button" class="password-toggle" id="pw-toggle" title="Show password">&#128065;</button>' +
                        '</div>' +
                    '</div>' +
                    '<button type="submit" class="btn login-submit">Login</button>' +
                '</form>' +
            '</div>' +
        '</div>';

    // Password visibility toggle
    var pwToggle = document.getElementById('pw-toggle');
    var pwInput = document.getElementById('password');
    pwToggle.addEventListener('click', function() {
        var isPassword = pwInput.type === 'password';
        pwInput.type = isPassword ? 'text' : 'password';
        pwToggle.innerHTML = isPassword ? '&#128064;' : '&#128065;';
        pwToggle.title = isPassword ? 'Hide password' : 'Show password';
    });

    document.getElementById('login-form').addEventListener('submit', function(e) {
        e.preventDefault();
        var username = document.getElementById('username').value;
        var password = document.getElementById('password').value;
        var errDiv = document.getElementById('login-error');
        errDiv.innerHTML = '';

        API.post('/api/v1/auth/token', {
            username: username,
            password: password,
            device_name: 'web-app'
        }).then(function(resp) {
            return resp.json();
        }).then(function(data) {
            if (data.error) {
                errDiv.innerHTML = '<div class="alert alert-error">' + esc(data.error) + '</div>';
                return;
            }
            API.setToken(data.token);
            sessionStorage.setItem('username', data.user.username);
            sessionStorage.setItem('is_admin', data.user.is_admin ? 'true' : 'false');
            window.location.hash = '#browser';
        }).catch(function() {
            errDiv.innerHTML = '<div class="alert alert-error">Login failed</div>';
        });
    });
}
