function renderUsers() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Users</h2>' +
            '<button class="btn" id="btn-show-create">Create User</button>' +
        '</div>' +
        '<div id="user-form-area"></div>' +
        '<div id="user-alert"></div>' +
        '<div id="users-table">Loading...</div>';

    document.getElementById('btn-show-create').addEventListener('click', showCreateForm);
    loadUsers();
}

function loadUsers() {
    API.get('/api/v1/admin/users').then(function(users) {
        if (!users || users.length === 0) {
            document.getElementById('users-table').innerHTML = '<p>No users found.</p>';
            return;
        }

        var rows = '';
        for (var i = 0; i < users.length; i++) {
            var u = users[i];
            rows += '<tr>' +
                '<td>' + esc(u.id) + '</td>' +
                '<td>' + esc(u.username) + '</td>' +
                '<td>' + (u.is_admin ? '<span class="badge badge-blue">Admin</span>' : 'User') + '</td>' +
                '<td>' + formatDate(u.created_at) + '</td>' +
                '<td>' +
                    '<button class="btn btn-sm btn-outline" onclick="showPasswordDialog(' + u.id + ')">Password</button> ' +
                    '<button class="btn btn-sm btn-danger" onclick="deleteUser(' + u.id + ', \'' + esc(u.username) + '\')">Delete</button>' +
                '</td>' +
            '</tr>';
        }

        document.getElementById('users-table').innerHTML =
            '<div class="table-wrap"><table>' +
                '<thead><tr><th>ID</th><th>Username</th><th>Role</th><th>Created</th><th>Actions</th></tr></thead>' +
                '<tbody>' + rows + '</tbody>' +
            '</table></div>';
    }).catch(function() {
        document.getElementById('users-table').innerHTML =
            '<div class="alert alert-error">Failed to load users</div>';
    });
}

function showCreateForm() {
    var area = document.getElementById('user-form-area');
    area.innerHTML =
        '<div class="form-card" style="margin-bottom:1rem">' +
            '<form id="create-user-form">' +
                '<div class="form-group">' +
                    '<label for="new-username">Username</label>' +
                    '<input type="text" id="new-username" required>' +
                '</div>' +
                '<div class="form-group">' +
                    '<label for="new-password">Password</label>' +
                    '<input type="password" id="new-password" required>' +
                '</div>' +
                '<div class="form-group checkbox-group">' +
                    '<input type="checkbox" id="new-admin">' +
                    '<label for="new-admin">Admin</label>' +
                '</div>' +
                '<button type="submit" class="btn">Create</button> ' +
                '<button type="button" class="btn btn-outline" id="btn-cancel-create">Cancel</button>' +
            '</form>' +
        '</div>';

    document.getElementById('btn-cancel-create').addEventListener('click', function() {
        area.innerHTML = '';
    });

    document.getElementById('create-user-form').addEventListener('submit', function(e) {
        e.preventDefault();
        var username = document.getElementById('new-username').value;
        var password = document.getElementById('new-password').value;
        var isAdmin = document.getElementById('new-admin').checked;

        API.post('/api/v1/admin/users', {
            username: username,
            password: password,
            is_admin: isAdmin
        }).then(function(resp) {
            return resp.json().then(function(data) {
                if (resp.ok) {
                    area.innerHTML = '';
                    showAlert('user-alert', 'User created successfully', 'success');
                    loadUsers();
                } else {
                    showAlert('user-alert', data.error || 'Failed to create user', 'error');
                }
            });
        }).catch(function() {
            showAlert('user-alert', 'Failed to create user', 'error');
        });
    });
}

function deleteUser(id, username) {
    if (!confirm('Delete user "' + username + '"? This cannot be undone.')) return;

    API.del('/api/v1/admin/users/' + id).then(function(resp) {
        return resp.json().then(function(data) {
            if (resp.ok) {
                showAlert('user-alert', 'User deleted', 'success');
                loadUsers();
            } else {
                showAlert('user-alert', data.error || 'Failed to delete user', 'error');
            }
        });
    }).catch(function() {
        showAlert('user-alert', 'Failed to delete user', 'error');
    });
}

function showPasswordDialog(id) {
    var newPass = prompt('Enter new password for user #' + id + ':');
    if (!newPass) return;

    API.put('/api/v1/admin/users/' + id + '/password', { password: newPass })
    .then(function(resp) {
        return resp.json().then(function(data) {
            if (resp.ok) {
                showAlert('user-alert', 'Password changed', 'success');
            } else {
                showAlert('user-alert', data.error || 'Failed to change password', 'error');
            }
        });
    }).catch(function() {
        showAlert('user-alert', 'Failed to change password', 'error');
    });
}

function showAlert(containerId, message, type) {
    var el = document.getElementById(containerId);
    el.innerHTML = '<div class="alert alert-' + type + '">' + esc(message) + '</div>';
    setTimeout(function() { el.innerHTML = ''; }, 4000);
}
