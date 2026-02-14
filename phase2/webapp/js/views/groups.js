// Admin groups management view - nested groups with roles
function renderGroups() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Groups</h2>' +
            '<div class="btn-group">' +
                '<button class="btn btn-sm btn-outline" id="btn-view-flat">List</button>' +
                '<button class="btn btn-sm" id="btn-view-tree">Tree</button>' +
            '</div>' +
            '<button class="btn" id="btn-show-create-group">Create Group</button>' +
        '</div>' +
        '<div id="group-form-area"></div>' +
        '<div id="groups-table">Loading...</div>';

    document.getElementById('btn-show-create-group').addEventListener('click', showGroupCreateForm);
    document.getElementById('btn-view-flat').addEventListener('click', function() {
        this.classList.add('btn-outline');
        document.getElementById('btn-view-tree').classList.remove('btn-outline');
        loadGroupList();
    });
    document.getElementById('btn-view-tree').addEventListener('click', function() {
        this.classList.remove('btn-outline');
        document.getElementById('btn-view-flat').classList.add('btn-outline');
        loadGroupTree();
    });

    loadGroupTree();
}

// ─── Group Tree View ────────────────────────────────────────────────────────

function loadGroupTree() {
    API.get('/api/v1/admin/groups/tree').then(function(tree) {
        if (!tree || tree.length === 0) {
            document.getElementById('groups-table').innerHTML = '<p>No groups found.</p>';
            return;
        }
        var html = '<div class="group-tree">' + renderTreeNodes(tree, 0) + '</div>';
        document.getElementById('groups-table').innerHTML = html;
        wireGroupTreeActions();
    }).catch(function() {
        Toast.info('Tree view unavailable, showing flat list');
        loadGroupList();
    });
}

function renderTreeNodes(nodes, depth) {
    var html = '';
    for (var i = 0; i < nodes.length; i++) {
        var g = nodes[i];
        var indent = depth * 24;
        var isTopLevel = depth === 0;
        html += '<div class="group-tree-row' + (isTopLevel ? ' group-tree-toplevel' : '') + '" style="padding-left:' + (indent + 12) + 'px">' +
            (depth > 0 ? '<span class="tree-connector"></span>' : '') +
            '<div class="group-tree-info">' +
                '<span class="group-tree-name' + (isTopLevel ? ' group-tree-org' : '') + '">' + esc(g.name) + '</span>' +
                (g.description ? '<span class="group-tree-desc">' + esc(g.description) + '</span>' : '') +
                '<span class="badge badge-blue">' + g.member_count + ' members</span>' +
            '</div>' +
            '<div class="btn-group">' +
                '<button class="btn btn-sm btn-outline" data-action="manage-group" data-id="' + g.id + '" data-name="' + esc(g.name) + '">Manage</button>' +
                '<button class="btn btn-sm btn-danger" data-action="delete-group" data-id="' + g.id + '" data-name="' + esc(g.name) + '">Delete</button>' +
            '</div>' +
        '</div>';
        if (g.children && g.children.length > 0) {
            html += renderTreeNodes(g.children, depth + 1);
        }
    }
    return html;
}

function wireGroupTreeActions() {
    document.getElementById('groups-table').querySelectorAll('[data-action]').forEach(function(btn) {
        btn.addEventListener('click', function(e) {
            var action = e.currentTarget.getAttribute('data-action');
            var id = parseInt(e.currentTarget.getAttribute('data-id'), 10);
            var name = e.currentTarget.getAttribute('data-name');
            if (action === 'manage-group') {
                showGroupDetail(id, name);
            } else if (action === 'delete-group') {
                deleteGroup(id, name);
            }
        });
    });
}

// ─── Group Flat List View ───────────────────────────────────────────────────

function loadGroupList() {
    API.get('/api/v1/admin/groups').then(function(groups) {
        if (!groups || groups.length === 0) {
            document.getElementById('groups-table').innerHTML = '<p>No groups found.</p>';
            return;
        }

        var rows = '';
        for (var i = 0; i < groups.length; i++) {
            var g = groups[i];
            rows += '<tr>' +
                '<td data-label="ID">' + esc(g.id) + '</td>' +
                '<td data-label="Name">' + esc(g.name) + '</td>' +
                '<td data-label="Description">' + esc(g.description || '-') + '</td>' +
                '<td data-label="Parent">' + esc(g.parent_name || '-') + '</td>' +
                '<td data-label="Members"><span class="badge badge-blue">' + esc(g.member_count) + '</span></td>' +
                '<td data-label="Creator">' + esc(g.creator_name || '-') + '</td>' +
                '<td data-label="Created">' + formatDate(g.created_at) + '</td>' +
                '<td data-label="">' +
                    '<div class="btn-group">' +
                        '<button class="btn btn-sm btn-outline" data-action="manage-group" data-id="' + g.id + '" data-name="' + esc(g.name) + '">Manage</button>' +
                        '<button class="btn btn-sm btn-danger" data-action="delete-group" data-id="' + g.id + '" data-name="' + esc(g.name) + '">Delete</button>' +
                    '</div>' +
                '</td>' +
            '</tr>';
        }

        document.getElementById('groups-table').innerHTML =
            '<div class="table-wrap"><table class="responsive-table">' +
                '<thead><tr><th>ID</th><th>Name</th><th>Description</th><th>Parent</th><th>Members</th><th>Creator</th><th>Created</th><th>Actions</th></tr></thead>' +
                '<tbody>' + rows + '</tbody>' +
            '</table></div>';

        wireGroupTreeActions();
    }).catch(function() {
        document.getElementById('groups-table').innerHTML =
            '<div class="alert alert-error">Failed to load groups</div>';
    });
}

// ─── Create Group Form ──────────────────────────────────────────────────────

function showGroupCreateForm() {
    var area = document.getElementById('group-form-area');

    // Load existing groups for parent selector
    API.get('/api/v1/admin/groups').then(function(groups) {
        var parentOptions = '<option value="">None (top-level)</option>';
        if (groups) {
            for (var i = 0; i < groups.length; i++) {
                parentOptions += '<option value="' + groups[i].id + '">' + esc(groups[i].name) + '</option>';
            }
        }

        area.innerHTML =
            '<div class="form-card" style="margin-bottom:1rem">' +
                '<form id="create-group-form">' +
                    '<div class="form-group">' +
                        '<label for="new-group-name">Group Name</label>' +
                        '<input type="text" id="new-group-name" required>' +
                    '</div>' +
                    '<div class="form-group">' +
                        '<label for="new-group-desc">Description</label>' +
                        '<input type="text" id="new-group-desc" placeholder="Optional">' +
                    '</div>' +
                    '<div class="form-group">' +
                        '<label for="new-group-parent">Parent Group</label>' +
                        '<select id="new-group-parent">' + parentOptions + '</select>' +
                    '</div>' +
                    '<button type="submit" class="btn">Create</button> ' +
                    '<button type="button" class="btn btn-outline" id="btn-cancel-create-group">Cancel</button>' +
                '</form>' +
            '</div>';

        document.getElementById('btn-cancel-create-group').addEventListener('click', function() {
            area.innerHTML = '';
        });

        document.getElementById('create-group-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var name = document.getElementById('new-group-name').value;
            var desc = document.getElementById('new-group-desc').value;
            var parentVal = document.getElementById('new-group-parent').value;
            var body = { name: name, description: desc };
            if (parentVal) body.parent_id = parseInt(parentVal, 10);

            API.post('/api/v1/admin/groups', body)
                .then(function(resp) {
                    return resp.json().then(function(data) {
                        if (resp.ok) {
                            area.innerHTML = '';
                            Toast.show('Group "' + name + '" created', 'success');
                            loadGroupTree();
                        } else {
                            Toast.show(data.error || 'Failed to create group', 'error');
                        }
                    });
                }).catch(function() {
                    Toast.show('Failed to create group', 'error');
                });
        });
    });
}

function deleteGroup(id, name) {
    if (!confirm('Delete group "' + name + '"? All members, permissions, and subgroups will be removed.')) return;

    API.del('/api/v1/admin/groups/' + id).then(function(resp) {
        return resp.json().then(function(data) {
            if (resp.ok) {
                Toast.show('Group deleted', 'success');
                loadGroupTree();
            } else {
                Toast.show(data.error || 'Failed to delete group', 'error');
            }
        });
    }).catch(function() {
        Toast.show('Failed to delete group', 'error');
    });
}

// ─── Group Detail Modal ─────────────────────────────────────────────────────

function showGroupDetail(groupID, groupName) {
    var contentDiv = document.createElement('div');
    contentDiv.innerHTML =
        '<div class="group-tabs">' +
            '<button class="btn btn-sm group-tab active" data-tab="members">Members</button>' +
            '<button class="btn btn-sm btn-outline group-tab" data-tab="permissions">Permissions</button>' +
            '<button class="btn btn-sm btn-outline group-tab" data-tab="subgroups">Subgroups</button>' +
            '<button class="btn btn-sm btn-outline group-tab" data-tab="storage">Storage</button>' +
        '</div>' +
        '<div id="group-tab-content"></div>';

    var modalEl = Modal.open({
        title: 'Group: ' + groupName,
        content: contentDiv,
        className: 'group-detail-modal'
    });

    // Tab switching
    modalEl.querySelectorAll('.group-tab').forEach(function(tab) {
        tab.addEventListener('click', function() {
            modalEl.querySelectorAll('.group-tab').forEach(function(t) {
                t.classList.remove('active');
                t.classList.add('btn-outline');
            });
            tab.classList.add('active');
            tab.classList.remove('btn-outline');
            var tabName = tab.getAttribute('data-tab');
            if (tabName === 'members') {
                loadGroupMembers(groupID);
            } else if (tabName === 'permissions') {
                loadGroupPermissions(groupID);
            } else if (tabName === 'subgroups') {
                loadGroupSubgroups(groupID, groupName);
            } else if (tabName === 'storage') {
                loadGroupStorage(groupID, groupName);
            }
        });
    });

    loadGroupMembers(groupID);
}

// ─── Members Tab ────────────────────────────────────────────────────────────

function loadGroupMembers(groupID) {
    var content = document.getElementById('group-tab-content');
    if (!content) return;
    content.innerHTML = '<p>Loading members...</p>';

    Promise.all([
        API.get('/api/v1/admin/groups/' + groupID + '/members'),
        API.get('/api/v1/admin/users')
    ]).then(function(results) {
        var members = results[0];
        var users = results[1];

        var html = '<div class="group-section">';

        // Add member form with role selector
        html += '<div class="group-add-form">' +
            '<select id="add-member-select">' +
                '<option value="">-- Select user --</option>';

        var memberIds = {};
        if (members) {
            for (var i = 0; i < members.length; i++) {
                memberIds[members[i].user_id] = true;
            }
        }
        if (users) {
            for (var j = 0; j < users.length; j++) {
                if (!memberIds[users[j].id]) {
                    html += '<option value="' + users[j].id + '">' + esc(users[j].username) + '</option>';
                }
            }
        }

        html += '</select>' +
            '<select id="add-member-role" style="width:auto">' +
                '<option value="viewer">Viewer</option>' +
                '<option value="editor">Editor</option>' +
                '<option value="admin">Admin</option>' +
            '</select>' +
            '<button class="btn btn-sm" id="btn-add-member">Add Member</button>' +
        '</div>';

        // Members list with roles
        if (!members || members.length === 0) {
            html += '<p style="color:var(--text-muted);padding:0.5rem 0">No members yet.</p>';
        } else {
            html += '<table class="responsive-table"><thead><tr><th>Username</th><th>Role</th><th>Added</th><th></th></tr></thead><tbody>';
            for (var k = 0; k < members.length; k++) {
                var m = members[k];
                var roleClass = m.role === 'admin' ? 'badge-red' : m.role === 'editor' ? 'badge-blue' : 'badge-green';
                html += '<tr>' +
                    '<td data-label="Username">' + esc(m.username) + '</td>' +
                    '<td data-label="Role">' +
                        '<select class="role-select" data-uid="' + m.user_id + '" style="padding:0.2rem 0.4rem;border-radius:4px;border:1px solid var(--border)">' +
                            '<option value="viewer"' + (m.role === 'viewer' ? ' selected' : '') + '>Viewer</option>' +
                            '<option value="editor"' + (m.role === 'editor' ? ' selected' : '') + '>Editor</option>' +
                            '<option value="admin"' + (m.role === 'admin' ? ' selected' : '') + '>Admin</option>' +
                        '</select>' +
                    '</td>' +
                    '<td data-label="Added">' + formatDate(m.added_at) + '</td>' +
                    '<td data-label=""><button class="btn btn-sm btn-danger" data-action="remove-member" data-uid="' + m.user_id + '">Remove</button></td>' +
                '</tr>';
            }
            html += '</tbody></table>';
        }

        html += '</div>';
        content.innerHTML = html;

        // Wire add member
        document.getElementById('btn-add-member').addEventListener('click', function() {
            var sel = document.getElementById('add-member-select');
            var uid = parseInt(sel.value, 10);
            var role = document.getElementById('add-member-role').value;
            if (!uid) return;
            API.post('/api/v1/admin/groups/' + groupID + '/members', { user_id: uid, role: role })
                .then(function(resp) {
                    if (resp.ok) {
                        loadGroupMembers(groupID);
                        loadGroupTree();
                    } else {
                        resp.json().then(function(d) { alert(d.error || 'Failed'); });
                    }
                });
        });

        // Wire role change
        content.querySelectorAll('.role-select').forEach(function(sel) {
            sel.addEventListener('change', function() {
                var uid = sel.getAttribute('data-uid');
                var newRole = sel.value;
                API.put('/api/v1/admin/groups/' + groupID + '/members/' + uid + '/role', { role: newRole })
                    .then(function(resp) {
                        if (!resp.ok) {
                            resp.json().then(function(d) { alert(d.error || 'Failed to update role'); });
                            loadGroupMembers(groupID);
                        }
                    });
            });
        });

        // Wire remove member
        content.querySelectorAll('[data-action="remove-member"]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var uid = btn.getAttribute('data-uid');
                API.del('/api/v1/admin/groups/' + groupID + '/members/' + uid)
                    .then(function(resp) {
                        if (resp.ok) {
                            loadGroupMembers(groupID);
                            loadGroupTree();
                        }
                    });
            });
        });
    });
}

// ─── Permissions Tab ────────────────────────────────────────────────────────

function loadGroupPermissions(groupID) {
    var content = document.getElementById('group-tab-content');
    if (!content) return;
    content.innerHTML = '<p>Loading permissions...</p>';

    API.get('/api/v1/admin/groups/' + groupID + '/permissions').then(function(perms) {
        var html = '<div class="group-section">';

        // Add permission form
        html += '<div class="group-add-form">' +
            '<input type="text" id="add-perm-path" placeholder="Path (e.g. /docs)" style="flex:1">' +
            '<select id="add-perm-level" style="width:auto">' +
                '<option value="read">Read</option>' +
                '<option value="write">Write</option>' +
                '<option value="owner">Owner</option>' +
            '</select>' +
            '<button class="btn btn-sm" id="btn-add-perm">Add</button>' +
        '</div>';

        // Permissions list
        if (!perms || perms.length === 0) {
            html += '<p style="color:var(--text-muted);padding:0.5rem 0">No permissions set.</p>';
        } else {
            html += '<table class="responsive-table"><thead><tr><th>Path</th><th>Permission</th><th></th></tr></thead><tbody>';
            for (var i = 0; i < perms.length; i++) {
                var p = perms[i];
                var badgeClass = p.permission === 'owner' ? 'badge-red' :
                                 p.permission === 'write' ? 'badge-blue' : 'badge-green';
                html += '<tr>' +
                    '<td data-label="Path"><code>' + esc(p.path) + '</code></td>' +
                    '<td data-label="Permission"><span class="badge ' + badgeClass + '">' + esc(p.permission) + '</span></td>' +
                    '<td data-label=""><button class="btn btn-sm btn-danger" data-action="remove-perm" data-path="' + esc(p.path) + '">Remove</button></td>' +
                '</tr>';
            }
            html += '</tbody></table>';
        }

        html += '</div>';
        content.innerHTML = html;

        // Wire add permission
        document.getElementById('btn-add-perm').addEventListener('click', function() {
            var path = document.getElementById('add-perm-path').value.trim();
            var level = document.getElementById('add-perm-level').value;
            if (!path) return;
            if (path[0] !== '/') path = '/' + path;

            API.put('/api/v1/admin/groups/' + groupID + '/permissions/' + API.encodeURIPath(path.replace(/^\//, '')),
                { permission: level })
                .then(function(resp) {
                    if (resp.ok) {
                        loadGroupPermissions(groupID);
                    } else {
                        resp.json().then(function(d) { alert(d.error || 'Failed'); });
                    }
                });
        });

        // Wire remove permission
        content.querySelectorAll('[data-action="remove-perm"]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var path = btn.getAttribute('data-path');
                API.del('/api/v1/admin/groups/' + groupID + '/permissions/' + API.encodeURIPath(path.replace(/^\//, '')))
                    .then(function(resp) {
                        if (resp.ok) {
                            loadGroupPermissions(groupID);
                        }
                    });
            });
        });
    });
}

// ─── Subgroups Tab ──────────────────────────────────────────────────────────

function loadGroupSubgroups(groupID, groupName) {
    var content = document.getElementById('group-tab-content');
    if (!content) return;
    content.innerHTML = '<p>Loading subgroups...</p>';

    API.get('/api/v1/admin/groups').then(function(groups) {
        var children = [];
        if (groups) {
            for (var i = 0; i < groups.length; i++) {
                if (groups[i].parent_id === groupID) {
                    children.push(groups[i]);
                }
            }
        }

        var html = '<div class="group-section">';

        // Create subgroup button
        html += '<div class="group-add-form">' +
            '<input type="text" id="new-subgroup-name" placeholder="Subgroup name">' +
            '<input type="text" id="new-subgroup-desc" placeholder="Description (optional)">' +
            '<button class="btn btn-sm" id="btn-create-subgroup">Create Subgroup</button>' +
        '</div>';

        if (children.length === 0) {
            html += '<p style="color:var(--text-muted);padding:0.5rem 0">No subgroups.</p>';
        } else {
            html += '<table class="responsive-table"><thead><tr><th>Name</th><th>Description</th><th>Members</th><th></th></tr></thead><tbody>';
            for (var j = 0; j < children.length; j++) {
                var c = children[j];
                html += '<tr>' +
                    '<td data-label="Name">' + esc(c.name) + '</td>' +
                    '<td data-label="Description">' + esc(c.description || '-') + '</td>' +
                    '<td data-label="Members"><span class="badge badge-blue">' + c.member_count + '</span></td>' +
                    '<td data-label="">' +
                        '<button class="btn btn-sm btn-outline" data-action="manage-group" data-id="' + c.id + '" data-name="' + esc(c.name) + '">Manage</button>' +
                    '</td>' +
                '</tr>';
            }
            html += '</tbody></table>';
        }

        html += '</div>';
        content.innerHTML = html;

        // Wire create subgroup
        document.getElementById('btn-create-subgroup').addEventListener('click', function() {
            var name = document.getElementById('new-subgroup-name').value.trim();
            var desc = document.getElementById('new-subgroup-desc').value.trim();
            if (!name) return;

            API.post('/api/v1/admin/groups', { name: name, description: desc, parent_id: groupID })
                .then(function(resp) {
                    if (resp.ok) {
                        loadGroupSubgroups(groupID, groupName);
                        loadGroupTree();
                    } else {
                        resp.json().then(function(d) { alert(d.error || 'Failed to create subgroup'); });
                    }
                });
        });

        // Wire manage subgroup
        content.querySelectorAll('[data-action="manage-group"]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var id = parseInt(btn.getAttribute('data-id'), 10);
                var name = btn.getAttribute('data-name');
                showGroupDetail(id, name);
            });
        });
    });
}

// ─── Storage Tab ────────────────────────────────────────────────────────────

function loadGroupStorage(groupID, groupName) {
    var content = document.getElementById('group-tab-content');
    if (!content) return;
    content.innerHTML = '<p>Loading storage locations...</p>';

    API.get('/api/v1/admin/storage').then(function(locations) {
        var groupLocations = [];
        if (locations) {
            for (var i = 0; i < locations.length; i++) {
                if (locations[i].group_id === groupID) {
                    groupLocations.push(locations[i]);
                }
            }
        }

        var html = '<div class="group-section">';

        html += '<div style="margin-bottom:1rem">' +
            '<button class="btn btn-sm" id="btn-add-group-storage">Add Storage Location for this Group</button>' +
        '</div>';

        if (groupLocations.length === 0) {
            html += '<p style="color:var(--text-muted);padding:0.5rem 0">No storage locations assigned. Files use the default backend.</p>';
        } else {
            html += '<table class="responsive-table"><thead><tr>' +
                '<th>Name</th><th>Type</th><th>Priority</th><th>Default</th><th></th>' +
            '</tr></thead><tbody>';

            for (var j = 0; j < groupLocations.length; j++) {
                var loc = groupLocations[j];
                html += '<tr>' +
                    '<td data-label="Name">' + esc(loc.name) + '</td>' +
                    '<td data-label="Type"><span class="badge badge-' + backendBadgeColor(loc.backend_type) + '">' +
                        esc(loc.backend_type.toUpperCase()) + '</span></td>' +
                    '<td data-label="Priority">' + loc.priority + '</td>' +
                    '<td data-label="Default">' + (loc.is_default ? 'Yes' : '-') + '</td>' +
                    '<td data-label=""><button class="btn btn-sm btn-outline" data-action="test-group-storage" data-id="' + loc.id + '">Test</button></td>' +
                '</tr>';
            }
            html += '</tbody></table>';
        }

        html += '</div>';
        content.innerHTML = html;

        // Wire add button — navigate to storage view
        document.getElementById('btn-add-group-storage').addEventListener('click', function() {
            Modal.close();
            window.location.hash = '#storage';
        });

        // Wire test buttons
        content.querySelectorAll('[data-action="test-group-storage"]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var id = parseInt(btn.getAttribute('data-id'), 10);
                var origText = btn.textContent;
                btn.textContent = 'Testing...';
                btn.disabled = true;

                API.post('/api/v1/admin/storage/' + id + '/test').then(function(resp) {
                    return resp.json().then(function(data) {
                        btn.textContent = origText;
                        btn.disabled = false;
                        if (data.success) {
                            Toast.success('Connection test passed!');
                        } else {
                            Toast.error('Test failed: ' + (data.error || 'Unknown error'));
                        }
                    });
                }).catch(function() {
                    btn.textContent = origText;
                    btn.disabled = false;
                    Toast.error('Test request failed');
                });
            });
        });
    }).catch(function() {
        content.innerHTML = '<div class="alert alert-error">Failed to load storage locations</div>';
    });
}

// backendBadgeColor is defined in storage.js (loaded after this file)
