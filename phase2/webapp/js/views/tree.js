// Sidebar folder tree component
var TreeView = (function() {
    var treeData = null;
    var currentPath = '/';

    function renderTree() {
        var sidebar = document.getElementById('sidebar');
        if (!sidebar) return;
        sidebar.innerHTML = '<div class="tree-header"><strong>Explorer</strong></div><div id="tree-content">Loading...</div>';
        loadTree();
    }

    function loadTree() {
        API.get('/api/v1/tree').then(function(data) {
            treeData = data.root;
            var content = document.getElementById('tree-content');
            if (!content) return;
            if (!treeData || !treeData.children || treeData.children.length === 0) {
                content.innerHTML = '<div class="tree-empty">No files</div>';
                return;
            }
            content.innerHTML = '<ul class="tree-list">' + renderNode(treeData, true) + '</ul>';
            highlightCurrent();
        }).catch(function() {
            var content = document.getElementById('tree-content');
            if (content) content.innerHTML = '<div class="tree-empty">Failed to load</div>';
        });
    }

    function renderNode(node, isRoot) {
        if (!node) return '';
        var html = '';

        if (isRoot) {
            // Render root children directly
            var children = sortChildren(node.children || []);
            for (var i = 0; i < children.length; i++) {
                html += renderNode(children[i], false);
            }
            return html;
        }

        if (node.is_dir) {
            var children = sortChildren(node.children || []);
            var hasChildren = children.length > 0;
            html += '<li class="tree-dir">';
            html += '<div class="tree-item" data-path="' + esc(node.path) + '">';
            html += '<span class="tree-toggle">' + (hasChildren ? '&#9654;' : '&nbsp;') + '</span>';
            html += '<span class="tree-icon">&#128193;</span>';
            html += '<span class="tree-label">' + esc(node.name) + '</span>';
            html += '</div>';
            if (hasChildren) {
                html += '<ul class="tree-children hidden">';
                for (var i = 0; i < children.length; i++) {
                    html += renderNode(children[i], false);
                }
                html += '</ul>';
            }
            html += '</li>';
        } else {
            html += '<li class="tree-file">';
            html += '<div class="tree-item" data-path="' + esc(node.path) + '">';
            html += '<span class="tree-toggle">&nbsp;</span>';
            html += '<span class="tree-icon">&#128196;</span>';
            html += '<span class="tree-label">' + esc(node.name) + '</span>';
            html += '</div>';
            html += '</li>';
        }

        return html;
    }

    function sortChildren(children) {
        return children.slice().sort(function(a, b) {
            if (a.is_dir && !b.is_dir) return -1;
            if (!a.is_dir && b.is_dir) return 1;
            return a.name.localeCompare(b.name);
        });
    }

    function highlightCurrent() {
        var items = document.querySelectorAll('#sidebar .tree-item');
        for (var i = 0; i < items.length; i++) {
            items[i].classList.remove('active');
            if (items[i].getAttribute('data-path') === currentPath) {
                items[i].classList.add('active');
                // Expand parents
                expandParents(items[i]);
            }
        }
    }

    function expandParents(el) {
        var parent = el.parentElement;
        while (parent && parent.id !== 'sidebar') {
            if (parent.classList.contains('tree-children')) {
                parent.classList.remove('hidden');
                // Update toggle icon
                var prevItem = parent.previousElementSibling;
                if (prevItem) {
                    var toggle = prevItem.querySelector('.tree-toggle');
                    if (toggle) toggle.innerHTML = '&#9660;';
                }
            }
            parent = parent.parentElement;
        }
    }

    // Event delegation for tree clicks
    document.addEventListener('click', function(e) {
        var item = e.target.closest('.tree-item');
        if (!item) return;
        var path = item.getAttribute('data-path');
        if (!path) return;

        var li = item.parentElement;
        if (li.classList.contains('tree-dir')) {
            // Toggle expand/collapse
            var children = li.querySelector('.tree-children');
            var toggle = item.querySelector('.tree-toggle');
            if (children) {
                children.classList.toggle('hidden');
                if (toggle) {
                    toggle.innerHTML = children.classList.contains('hidden') ? '&#9654;' : '&#9660;';
                }
            }
            // Navigate to directory
            window.location.hash = '#browser' + path;
        } else {
            // Navigate to file viewer
            window.location.hash = '#viewer' + path;
        }
    });

    function setCurrentPath(path) {
        currentPath = path;
        highlightCurrent();
    }

    function refresh() {
        loadTree();
    }

    return {
        renderTree: renderTree,
        setCurrentPath: setCurrentPath,
        refresh: refresh
    };
})();
