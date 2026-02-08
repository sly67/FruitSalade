function renderLogin() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="login-wrap">' +
            '<div class="form-card login-card">' +
                '<h1>FruitSalade</h1>' +
                '<div id="login-error"></div>' +
                '<form id="login-form">' +
                    '<div class="form-group">' +
                        '<label for="username">Username</label>' +
                        '<input type="text" id="username" autocomplete="username" required>' +
                    '</div>' +
                    '<div class="form-group">' +
                        '<label for="password">Password</label>' +
                        '<input type="password" id="password" autocomplete="current-password" required>' +
                    '</div>' +
                    '<button type="submit" class="btn" style="width:100%">Login</button>' +
                '</form>' +
            '</div>' +
        '</div>';

    document.getElementById('login-form').addEventListener('submit', function(e) {
        e.preventDefault();
        var username = document.getElementById('username').value;
        var password = document.getElementById('password').value;
        var errDiv = document.getElementById('login-error');
        errDiv.innerHTML = '';

        API.post('/api/v1/auth/token', {
            username: username,
            password: password,
            device_name: 'admin-ui'
        }).then(function(resp) {
            return resp.json();
        }).then(function(data) {
            if (data.error) {
                errDiv.innerHTML = '<div class="alert alert-error">' + esc(data.error) + '</div>';
                return;
            }
            if (!data.user.is_admin) {
                errDiv.innerHTML = '<div class="alert alert-error">Admin access required</div>';
                return;
            }
            API.setToken(data.token);
            sessionStorage.setItem('username', data.user.username);
            window.location.hash = '#dashboard';
        }).catch(function() {
            errDiv.innerHTML = '<div class="alert alert-error">Login failed</div>';
        });
    });
}
