// Admin user management view with group memberships
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

    document.getElementById('btn-show-create').addEventListener('click', showUserCreateForm);
    loadUserList();
}

function loadUserList() {
    // Load users and all groups in parallel
    Promise.all([
        API.get('/api/v1/admin/users'),
        API.get('/api/v1/admin/groups')
    ]).then(function(results) {
        var users = results[0];
        var groups = results[1];

        if (!users || users.length === 0) {
            document.getElementById('users-table').innerHTML = '<p>No users found.</p>';
            return;
        }

        var rows = '';
        for (var i = 0; i < users.length; i++) {
            var u = users[i];
            rows += '<tr>' +
                '<td data-label="ID">' + esc(u.id) + '</td>' +
                '<td data-label="Username">' + esc(u.username) + '</td>' +
                '<td data-label="Role">' + (u.is_admin ? '<span class="badge badge-blue">Admin</span>' : 'User') + '</td>' +
                '<td data-label="Groups"><div class="user-groups-cell" id="user-groups-' + u.id + '"><span class="props-muted">loading...</span></div></td>' +
                '<td data-label="Created">' + formatDate(u.created_at) + '</td>' +
                '<td data-label="">' +
                    '<div class="btn-group">' +
                        '<button class="btn btn-sm btn-outline" data-action="manage-groups" data-id="' + u.id + '" data-name="' + esc(u.username) + '">Groups</button>' +
                        '<button class="btn btn-sm btn-outline" data-action="password" data-id="' + u.id + '">Password</button>' +
                        '<button class="btn btn-sm btn-danger" data-action="delete-user" data-id="' + u.id + '" data-name="' + esc(u.username) + '">Delete</button>' +
                    '</div>' +
                '</td>' +
            '</tr>';
        }

        document.getElementById('users-table').innerHTML =
            '<div class="table-wrap"><table class="responsive-table">' +
                '<thead><tr><th>ID</th><th>Username</th><th>Role</th><th>Groups</th><th>Created</th><th>Actions</th></tr></thead>' +
                '<tbody>' + rows + '</tbody>' +
            '</table></div>';

        // Load group memberships for each user
        for (var j = 0; j < users.length; j++) {
            loadUserGroupBadges(users[j].id);
        }

        // Wire action buttons
        document.getElementById('users-table').querySelectorAll('[data-action]').forEach(function(btn) {
            btn.addEventListener('click', function(e) {
                var action = e.currentTarget.getAttribute('data-action');
                var id = parseInt(e.currentTarget.getAttribute('data-id'), 10);
                var name = e.currentTarget.getAttribute('data-name');
                if (action === 'password') {
                    showUserPasswordDialog(id);
                } else if (action === 'delete-user') {
                    deleteUserById(id, name);
                } else if (action === 'manage-groups') {
                    showUserGroupsModal(id, name, groups || []);
                }
            });
        });
    }).catch(function() {
        document.getElementById('users-table').innerHTML =
            '<div class="alert alert-error">Failed to load users</div>';
    });
}

function loadUserGroupBadges(userID) {
    var cell = document.getElementById('user-groups-' + userID);
    if (!cell) return;

    API.get('/api/v1/admin/users/' + userID + '/groups').then(function(memberships) {
        if (!memberships || memberships.length === 0) {
            cell.innerHTML = '<span class="props-muted">none</span>';
            return;
        }
        var badges = '';
        for (var i = 0; i < memberships.length; i++) {
            var m = memberships[i];
            var roleClass = 'role-' + m.role;
            badges += '<span class="user-group-badge">' +
                esc(m.group_name) +
                '<span class="role-badge ' + roleClass + '">' + esc(m.role) + '</span>' +
                '</span>';
        }
        cell.innerHTML = badges;
    }).catch(function() {
        cell.innerHTML = '<span class="props-muted">-</span>';
    });
}

function showUserGroupsModal(userID, username, allGroups) {
    var modalArea = document.getElementById('user-form-area');
    modalArea.innerHTML =
        '<div class="modal-overlay" id="user-groups-overlay">' +
            '<div class="modal" style="max-width:550px">' +
                '<button class="modal-close" id="user-groups-close">&times;</button>' +
                '<h3>Groups for ' + esc(username) + '</h3>' +
                '<div id="user-groups-list">Loading...</div>' +
                '<div id="user-groups-add" style="margin-top:1rem"></div>' +
            '</div>' +
        '</div>';

    document.getElementById('user-groups-close').addEventListener('click', function() {
        modalArea.innerHTML = '';
    });
    document.getElementById('user-groups-overlay').addEventListener('click', function(e) {
        if (e.target === e.currentTarget) modalArea.innerHTML = '';
    });

    loadUserGroupsList(userID, username, allGroups, modalArea);
}

function loadUserGroupsList(userID, username, allGroups, modalArea) {
    API.get('/api/v1/admin/users/' + userID + '/groups').then(function(memberships) {
        var listEl = document.getElementById('user-groups-list');
        var addEl = document.getElementById('user-groups-add');
        if (!listEl || !addEl) return;

        memberships = memberships || [];

        if (memberships.length === 0) {
            listEl.innerHTML = '<div class="props-empty">Not a member of any group</div>';
        } else {
            var html = '<table class="props-table"><thead><tr><th>Group</th><th>Role</th><th></th></tr></thead><tbody>';
            for (var i = 0; i < memberships.length; i++) {
                var m = memberships[i];
                var roleClass = 'role-' + m.role;
                html += '<tr>' +
                    '<td>' + esc(m.group_name) + '</td>' +
                    '<td>' +
                        '<select class="role-select" data-gid="' + m.group_id + '">' +
                            '<option value="viewer"' + (m.role === 'viewer' ? ' selected' : '') + '>Viewer</option>' +
                            '<option value="editor"' + (m.role === 'editor' ? ' selected' : '') + '>Editor</option>' +
                            '<option value="admin"' + (m.role === 'admin' ? ' selected' : '') + '>Admin</option>' +
                        '</select>' +
                    '</td>' +
                    '<td><button class="btn btn-sm btn-danger" data-remove-gid="' + m.group_id + '">Remove</button></td>' +
                    '</tr>';
            }
            html += '</tbody></table>';
            listEl.innerHTML = html;

            // Wire role change
            listEl.querySelectorAll('.role-select').forEach(function(sel) {
                sel.addEventListener('change', function() {
                    var gid = sel.getAttribute('data-gid');
                    API.put('/api/v1/admin/groups/' + gid + '/members/' + userID + '/role', { role: sel.value })
                        .then(function(resp) {
                            if (!resp.ok) {
                                resp.json().then(function(d) { alert(d.error || 'Failed to update role'); });
                            } else {
                                loadUserGroupBadges(userID);
                            }
                        });
                });
            });

            // Wire remove buttons
            listEl.querySelectorAll('[data-remove-gid]').forEach(function(btn) {
                btn.addEventListener('click', function() {
                    var gid = btn.getAttribute('data-remove-gid');
                    if (!confirm('Remove ' + username + ' from this group?')) return;
                    API.del('/api/v1/admin/groups/' + gid + '/members/' + userID).then(function(resp) {
                        if (resp.ok) {
                            loadUserGroupsList(userID, username, allGroups, modalArea);
                            loadUserGroupBadges(userID);
                        } else {
                            resp.json().then(function(d) { alert(d.error || 'Failed to remove'); });
                        }
                    });
                });
            });
        }

        // Add to group form
        var memberGroupIDs = {};
        for (var j = 0; j < memberships.length; j++) {
            memberGroupIDs[memberships[j].group_id] = true;
        }

        var availableGroups = (allGroups || []).filter(function(g) {
            return !memberGroupIDs[g.id];
        });

        if (availableGroups.length > 0) {
            var opts = '';
            for (var k = 0; k < availableGroups.length; k++) {
                opts += '<option value="' + availableGroups[k].id + '">' + esc(availableGroups[k].name) + '</option>';
            }
            addEl.innerHTML =
                '<div class="group-add-form">' +
                    '<select id="add-group-select">' + opts + '</select>' +
                    '<select id="add-group-role">' +
                        '<option value="viewer">Viewer</option>' +
                        '<option value="editor">Editor</option>' +
                        '<option value="admin">Admin</option>' +
                    '</select>' +
                    '<button class="btn btn-sm" id="btn-add-to-group">Add</button>' +
                '</div>';

            document.getElementById('btn-add-to-group').addEventListener('click', function() {
                var gid = document.getElementById('add-group-select').value;
                var role = document.getElementById('add-group-role').value;
                API.post('/api/v1/admin/groups/' + gid + '/members', { user_id: userID, role: role })
                    .then(function(resp) {
                        if (resp.ok) {
                            loadUserGroupsList(userID, username, allGroups, modalArea);
                            loadUserGroupBadges(userID);
                        } else {
                            resp.json().then(function(d) { alert(d.error || 'Failed to add'); });
                        }
                    });
            });
        } else {
            addEl.innerHTML = '';
        }
    }).catch(function() {
        var el = document.getElementById('user-groups-list');
        if (el) el.innerHTML = '<div class="alert alert-error">Failed to load groups</div>';
    });
}

function showUserCreateForm() {
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
                    showUserAlert('User created successfully', 'success');
                    loadUserList();
                } else {
                    showUserAlert(data.error || 'Failed to create user', 'error');
                }
            });
        }).catch(function() {
            showUserAlert('Failed to create user', 'error');
        });
    });
}

function deleteUserById(id, username) {
    if (!confirm('Delete user "' + username + '"? This cannot be undone.')) return;

    API.del('/api/v1/admin/users/' + id).then(function(resp) {
        return resp.json().then(function(data) {
            if (resp.ok) {
                showUserAlert('User deleted', 'success');
                loadUserList();
            } else {
                showUserAlert(data.error || 'Failed to delete user', 'error');
            }
        });
    }).catch(function() {
        showUserAlert('Failed to delete user', 'error');
    });
}

function showUserPasswordDialog(id) {
    var newPass = prompt('Enter new password for user #' + id + ':');
    if (!newPass) return;

    API.put('/api/v1/admin/users/' + id + '/password', { password: newPass })
    .then(function(resp) {
        return resp.json().then(function(data) {
            if (resp.ok) {
                showUserAlert('Password changed', 'success');
            } else {
                showUserAlert(data.error || 'Failed to change password', 'error');
            }
        });
    }).catch(function() {
        showUserAlert('Failed to change password', 'error');
    });
}

function showUserAlert(message, type) {
    var el = document.getElementById('user-alert');
    if (!el) return;
    el.innerHTML = '<div class="alert alert-' + type + '">' + esc(message) + '</div>';
    setTimeout(function() { if (el) el.innerHTML = ''; }, 4000);
}
