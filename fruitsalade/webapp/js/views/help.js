// ─── Help / Wiki View ─────────────────────────────────────────────────────
// Embedded documentation wiki with 12 categories, ToC sidebar, search,
// scroll-spy, and cross-article navigation.

var HELP_CATEGORIES = [
    { id: 'getting-started', label: 'Getting Started' },
    { id: 'files',           label: 'Files' },
    { id: 'versioning',      label: 'Versioning' },
    { id: 'sharing',         label: 'Sharing' },
    { id: 'gallery',         label: 'Gallery' },
    { id: 'favorites-trash', label: 'Favorites & Trash' },
    { id: 'groups',          label: 'Groups' },
    { id: 'security',        label: 'Security' },
    { id: 'clients',         label: 'Clients' },
    { id: 'admin',           label: 'Admin' },
    { id: 'docker',          label: 'Docker' },
    { id: 'api-reference',   label: 'API Reference' },
    { id: 'credits',         label: 'Credits' }
];

var HELP_ARTICLES = [
    // ── Getting Started ──────────────────────────────────────────────────
    {
        id: 'first-login',
        category: 'getting-started',
        title: 'First Login',
        body:
            '<p>After deploying FruitSalade, open your browser and navigate to your server address (e.g. <code>https://your-server:48000/app/</code>).</p>' +
            '<h4>Default Credentials</h4>' +
            '<p>The seed tool creates an initial admin account:</p>' +
            '<ul>' +
                '<li>Username: <code>admin</code></li>' +
                '<li>Password: <code>admin</code></li>' +
            '</ul>' +
            '<div class="wiki-warning">Change the default password immediately after your first login via the <span class="wiki-link" data-wiki-link="dashboard-overview">Dashboard</span> Security section.</div>' +
            '<h4>Login Flow</h4>' +
            '<ol>' +
                '<li>Enter your username and password</li>' +
                '<li>If <span class="wiki-link" data-wiki-link="totp-2fa">2FA</span> is enabled, enter your TOTP code</li>' +
                '<li>You will be redirected to the file browser</li>' +
            '</ol>'
    },
    {
        id: 'dashboard-overview',
        category: 'getting-started',
        title: 'Dashboard Overview',
        body:
            '<p>The Dashboard is your personal home page. It shows key metrics and quick actions.</p>' +
            '<h4>Sections</h4>' +
            '<ul>' +
                '<li><strong>Quick Stats</strong> &mdash; Total files, storage used, recent uploads</li>' +
                '<li><strong>Recent Activity</strong> &mdash; Last uploaded or modified files</li>' +
                '<li><strong>Security</strong> &mdash; 2FA status, active sessions, password change</li>' +
                '<li><strong>Storage Analytics</strong> &mdash; Charts showing usage by type, growth over time (admin)</li>' +
            '</ul>' +
            '<div class="wiki-tip">Admin users see additional analytics charts and system-wide statistics on the Storage tab.</div>'
    },
    {
        id: 'file-browser-basics',
        category: 'getting-started',
        title: 'File Browser Basics',
        body:
            '<p>The <strong>Files</strong> view is the main interface for browsing your synced files.</p>' +
            '<h4>Navigation</h4>' +
            '<ul>' +
                '<li>Click a folder to enter it; use the breadcrumb bar to navigate back</li>' +
                '<li>The sidebar tree on the left provides quick folder access</li>' +
                '<li>Use <kbd>Ctrl</kbd>+<kbd>K</kbd> to open the search bar</li>' +
            '</ul>' +
            '<h4>File Actions</h4>' +
            '<p>Right-click a file or use the action menu (<code>...</code>) to access:</p>' +
            '<ul>' +
                '<li>Download, Rename, Move, Copy, Delete</li>' +
                '<li>Share, Properties, Version History</li>' +
                '<li>Add to Favorites, View in Gallery</li>' +
            '</ul>'
    },

    // ── Files ────────────────────────────────────────────────────────────
    {
        id: 'upload-files',
        category: 'files',
        title: 'Uploading Files',
        body:
            '<p>There are several ways to upload files to FruitSalade:</p>' +
            '<h4>Drag &amp; Drop</h4>' +
            '<p>Drag files or folders directly from your desktop onto the file browser. A drop zone overlay will appear.</p>' +
            '<h4>Upload Button</h4>' +
            '<p>Click the <strong>Upload</strong> button in the toolbar to open a file picker. You can select multiple files.</p>' +
            '<h4>Folder Upload</h4>' +
            '<p>Click the <strong>Upload Folder</strong> button to upload an entire directory structure, preserving subfolder hierarchy.</p>' +
            '<div class="wiki-note">Upload progress is shown in a progress bar. Large files are uploaded sequentially to avoid memory issues.</div>' +
            '<h4>Upload Limits</h4>' +
            '<p>The maximum upload size is configured server-side (default: 100 MB). Admins can adjust this via the <code>MAX_UPLOAD_SIZE</code> environment variable.</p>'
    },
    {
        id: 'folders',
        category: 'files',
        title: 'Creating Folders',
        body:
            '<p>Click the <strong>New Folder</strong> button in the toolbar to create a new folder in the current directory.</p>' +
            '<ul>' +
                '<li>Enter a name in the dialog and click Create</li>' +
                '<li>Folder names must be unique within the same directory</li>' +
                '<li>Nested folder creation is supported via folder upload</li>' +
            '</ul>'
    },
    {
        id: 'rename-move-copy',
        category: 'files',
        title: 'Rename, Move &amp; Copy',
        body:
            '<h4>Rename</h4>' +
            '<p>Select a file or folder, then choose <strong>Rename</strong> from the action menu. Enter the new name and confirm.</p>' +
            '<h4>Move</h4>' +
            '<p>Use the <strong>Move</strong> action to relocate files to a different folder. A folder picker dialog lets you choose the destination.</p>' +
            '<h4>Copy</h4>' +
            '<p>The <strong>Copy</strong> action creates a duplicate in the selected destination folder.</p>' +
            '<div class="wiki-tip">You can also use bulk operations to move or copy multiple files at once. See <span class="wiki-link" data-wiki-link="bulk-operations">Bulk Operations</span>.</div>'
    },
    {
        id: 'download-files',
        category: 'files',
        title: 'Downloading Files',
        body:
            '<p>Click the <strong>Download</strong> button in a file\'s action menu or the detail panel to download it.</p>' +
            '<ul>' +
                '<li>Single files download directly</li>' +
                '<li>The server supports <code>Range</code> headers for resumable downloads</li>' +
            '</ul>'
    },
    {
        id: 'bulk-operations',
        category: 'files',
        title: 'Bulk Operations',
        body:
            '<p>Select multiple files using checkboxes, then use the bulk action toolbar that appears.</p>' +
            '<h4>Available Bulk Actions</h4>' +
            '<ul>' +
                '<li><strong>Move</strong> &mdash; Move selected items to a chosen folder</li>' +
                '<li><strong>Copy</strong> &mdash; Copy selected items</li>' +
                '<li><strong>Delete</strong> &mdash; Send selected items to trash</li>' +
                '<li><strong>Favorite</strong> &mdash; Add or remove from favorites</li>' +
            '</ul>' +
            '<div class="wiki-note">Hold <kbd>Shift</kbd> and click to select a range of files.</div>'
    },
    {
        id: 'file-properties',
        category: 'files',
        title: 'File Properties',
        body:
            '<p>Click on a file to open the detail panel on the right side. This shows:</p>' +
            '<ul>' +
                '<li><strong>Name, Path, Size</strong></li>' +
                '<li><strong>Created / Modified</strong> timestamps</li>' +
                '<li><strong>Content Hash</strong> (SHA-256)</li>' +
                '<li><strong>Version</strong> number</li>' +
                '<li><strong>Storage Backend</strong> location</li>' +
            '</ul>'
    },
    {
        id: 'search-files',
        category: 'files',
        title: 'Search',
        body:
            '<p>Use the global search bar (<kbd>Ctrl</kbd>+<kbd>K</kbd>) or navigate to the <strong>Search</strong> page.</p>' +
            '<h4>Search Features</h4>' +
            '<ul>' +
                '<li>Search by file name</li>' +
                '<li>Filter by type: All, Documents, Images, Videos, Audio</li>' +
                '<li>Results show file path and size</li>' +
                '<li>Click a result to navigate to the file</li>' +
            '</ul>' +
            '<p>Recent searches are saved locally for quick re-access.</p>'
    },
    {
        id: 'drag-and-drop',
        category: 'files',
        title: 'Drag &amp; Drop',
        body:
            '<p>FruitSalade supports drag-and-drop for file uploads:</p>' +
            '<ol>' +
                '<li>Drag files from your desktop or file manager</li>' +
                '<li>A blue overlay drop zone appears over the file browser</li>' +
                '<li>Release to start uploading to the current folder</li>' +
            '</ol>' +
            '<div class="wiki-tip">You can drag entire folders for recursive upload (in supported browsers).</div>'
    },

    // ── Versioning ───────────────────────────────────────────────────────
    {
        id: 'version-history',
        category: 'versioning',
        title: 'Version History',
        body:
            '<p>FruitSalade automatically tracks file versions. Every time a file is updated, the previous version is saved.</p>' +
            '<h4>Viewing History</h4>' +
            '<p>Go to <strong>File Management</strong> &rarr; <strong>Versions</strong> tab, or select <strong>Versions</strong> from a file\'s action menu.</p>' +
            '<ul>' +
                '<li>Each version shows the version number, date, and size</li>' +
                '<li>Versions are stored alongside the current file</li>' +
            '</ul>' +
            '<div class="wiki-note">S3 backends store versions at <code>_versions/{key}/{version}</code>.</div>'
    },
    {
        id: 'download-old-version',
        category: 'versioning',
        title: 'Download Old Versions',
        body:
            '<p>From the version history panel, click <strong>Download</strong> next to any version to retrieve that specific version of the file.</p>'
    },
    {
        id: 'rollback',
        category: 'versioning',
        title: 'Rollback',
        body:
            '<p>To restore a previous version:</p>' +
            '<ol>' +
                '<li>Open the version history for the file</li>' +
                '<li>Click <strong>Restore</strong> on the desired version</li>' +
                '<li>The file is replaced with the old version (creating a new version entry)</li>' +
            '</ol>' +
            '<div class="wiki-warning">Rollback creates a new version. The current file content is preserved as a version entry before being replaced.</div>'
    },
    {
        id: 'conflict-resolution',
        category: 'versioning',
        title: 'Conflict Resolution',
        body:
            '<p>When two clients modify the same file simultaneously, a <strong>conflict</strong> is detected via version headers.</p>' +
            '<h4>How It Works</h4>' +
            '<ul>' +
                '<li>Upload requests include <code>X-Expected-Version</code> or <code>If-Match</code> headers</li>' +
                '<li>If the server version doesn\'t match, a <strong>409 Conflict</strong> response is returned</li>' +
                '<li>The conflicting file is saved alongside the original</li>' +
            '</ul>' +
            '<h4>Resolving Conflicts</h4>' +
            '<p>Go to <strong>File Management</strong> &rarr; <strong>Conflicts</strong> tab to see all conflicts. Use the compare view to inspect differences and choose which version to keep.</p>'
    },

    // ── Sharing ──────────────────────────────────────────────────────────
    {
        id: 'share-links',
        category: 'sharing',
        title: 'Share Links',
        body:
            '<p>Create a share link to give anyone access to a file without requiring a login.</p>' +
            '<h4>Creating a Share Link</h4>' +
            '<ol>' +
                '<li>Select a file and click <strong>Share</strong></li>' +
                '<li>Configure options: password protection, expiry date, max downloads</li>' +
                '<li>Click <strong>Create Link</strong></li>' +
                '<li>Copy the generated URL</li>' +
            '</ol>' +
            '<div class="wiki-tip">Share links work with the <code>/share/{token}</code> page, which does not require authentication.</div>'
    },
    {
        id: 'managing-shares',
        category: 'sharing',
        title: 'Managing Shares',
        body:
            '<p>View and manage all your active share links on the <strong>My Shares</strong> page.</p>' +
            '<ul>' +
                '<li>See download count, expiry status, and creation date</li>' +
                '<li>Delete share links you no longer need</li>' +
                '<li>Admins can manage all share links via <strong>Admin &rarr; Share Links</strong></li>' +
            '</ul>'
    },
    {
        id: 'permissions',
        category: 'sharing',
        title: 'Permissions',
        body:
            '<p>File permissions control who can access specific files and folders.</p>' +
            '<h4>Permission Types</h4>' +
            '<table>' +
                '<tr><th>Permission</th><th>Description</th></tr>' +
                '<tr><td><code>read</code></td><td>View and download files</td></tr>' +
                '<tr><td><code>write</code></td><td>Upload, modify, and delete files</td></tr>' +
                '<tr><td><code>admin</code></td><td>Manage permissions and settings</td></tr>' +
            '</table>' +
            '<h4>Path Inheritance</h4>' +
            '<p>Permissions are inherited down the directory tree. A <code>write</code> permission on <code>/docs</code> applies to all files inside <code>/docs</code>.</p>'
    },
    {
        id: 'visibility',
        category: 'sharing',
        title: 'File Visibility',
        body:
            '<p>Files can have different visibility levels tied to group membership:</p>' +
            '<ul>' +
                '<li><strong>Public</strong> &mdash; Visible to all authenticated users</li>' +
                '<li><strong>Group</strong> &mdash; Only visible to members of the owning group</li>' +
                '<li><strong>Private</strong> &mdash; Only visible to the owner</li>' +
            '</ul>' +
            '<p>Visibility is set at the group level via the <code>visibility</code> field. See <span class="wiki-link" data-wiki-link="group-permissions">Group Permissions</span>.</p>'
    },

    // ── Gallery ──────────────────────────────────────────────────────────
    {
        id: 'gallery-browsing',
        category: 'gallery',
        title: 'Browsing the Gallery',
        body:
            '<p>The Gallery automatically indexes all image and video files in your library.</p>' +
            '<h4>View Modes</h4>' +
            '<ul>' +
                '<li><strong>Grid</strong> &mdash; Thumbnail grid layout (default)</li>' +
                '<li><strong>Timeline</strong> &mdash; Organized by date</li>' +
                '<li><strong>Map</strong> &mdash; Geolocated photos on a world map</li>' +
            '</ul>' +
            '<p>Use the tabs at the top to switch between modes. Filters for tags and albums help narrow results.</p>'
    },
    {
        id: 'gallery-lightbox',
        category: 'gallery',
        title: 'Lightbox',
        body:
            '<p>Click any thumbnail to open the lightbox (fullscreen viewer).</p>' +
            '<h4>Lightbox Controls</h4>' +
            '<ul>' +
                '<li><strong>Arrow keys</strong> or swipe to navigate between photos</li>' +
                '<li><strong>Toolbar</strong>: Download, Share, Favorite, Info panel</li>' +
                '<li><strong>EXIF panel</strong>: Camera model, settings, GPS coordinates</li>' +
                '<li><strong>Tags &amp; Albums</strong>: View and manage tag/album membership</li>' +
            '</ul>' +
            '<p>Press <kbd>Escape</kbd> to close the lightbox.</p>'
    },
    {
        id: 'gallery-exif',
        category: 'gallery',
        title: 'EXIF Data',
        body:
            '<p>Photos with embedded EXIF metadata display additional information:</p>' +
            '<ul>' +
                '<li>Camera make and model</li>' +
                '<li>Exposure, ISO, aperture, focal length</li>' +
                '<li>GPS coordinates (shown on a mini-map in the lightbox)</li>' +
                '<li>Date taken</li>' +
            '</ul>' +
            '<div class="wiki-note">EXIF extraction requires a gallery plugin to be configured. See <span class="wiki-link" data-wiki-link="admin-gallery-plugins">Gallery Plugins</span>.</div>'
    },
    {
        id: 'gallery-tags',
        category: 'gallery',
        title: 'Tags',
        body:
            '<p>Tags help organize your photos. There are two types:</p>' +
            '<h4>Manual Tags</h4>' +
            '<p>Added by users via the lightbox tag input. You can add, remove, and rename your own tags from <strong>Settings &rarr; My Tags</strong>.</p>' +
            '<h4>Auto Tags</h4>' +
            '<p>Generated by gallery plugins (e.g., AI classification). Auto tags have a source label and cannot be edited directly.</p>' +
            '<p>Click any tag pill in the gallery to filter by that tag.</p>'
    },
    {
        id: 'gallery-albums',
        category: 'gallery',
        title: 'Albums',
        body:
            '<p>Albums are personal collections of photos.</p>' +
            '<h4>Managing Albums</h4>' +
            '<ul>' +
                '<li>Create albums in <strong>Settings &rarr; My Albums</strong></li>' +
                '<li>Add photos to albums from the lightbox</li>' +
                '<li>Browse album contents from the Gallery Albums tab</li>' +
            '</ul>' +
            '<div class="wiki-tip">Album names must be unique per user.</div>'
    },
    {
        id: 'gallery-map',
        category: 'gallery',
        title: 'Photo Map',
        body:
            '<p>The Map view shows geolocated photos on an interactive world map.</p>' +
            '<ul>' +
                '<li>Photos with GPS EXIF data appear as markers</li>' +
                '<li>Nearby photos are clustered; zoom in to expand clusters</li>' +
                '<li>Click a marker to see a thumbnail popup; click through to the lightbox</li>' +
            '</ul>'
    },
    {
        id: 'gallery-sharing',
        category: 'gallery',
        title: 'Gallery Sharing',
        body:
            '<p>Share individual gallery photos directly from the lightbox:</p>' +
            '<ol>' +
                '<li>Open a photo in the lightbox</li>' +
                '<li>Click the share button in the toolbar</li>' +
                '<li>Configure share options (password, expiry, max downloads)</li>' +
                '<li>Copy the generated share link</li>' +
            '</ol>'
    },

    // ── Favorites & Trash ────────────────────────────────────────────────
    {
        id: 'favorites',
        category: 'favorites-trash',
        title: 'Favorites',
        body:
            '<p>Mark frequently accessed files as favorites for quick access.</p>' +
            '<h4>Adding Favorites</h4>' +
            '<ul>' +
                '<li>Click the star icon next to any file in the browser</li>' +
                '<li>Or use the action menu &rarr; <strong>Favorite</strong></li>' +
            '</ul>' +
            '<h4>Viewing Favorites</h4>' +
            '<p>Navigate to the <strong>Favorites</strong> page from the sidebar to see all your starred files in one place.</p>'
    },
    {
        id: 'trash',
        category: 'favorites-trash',
        title: 'Trash',
        body:
            '<p>Deleted files are moved to the Trash instead of being permanently removed.</p>' +
            '<ul>' +
                '<li>Trashed files are kept for a configurable retention period</li>' +
                '<li>You can restore files from the Trash at any time</li>' +
                '<li>Use <strong>Empty Trash</strong> to permanently delete all trashed files</li>' +
            '</ul>' +
            '<div class="wiki-warning">Emptying the trash is irreversible. Files cannot be recovered after this action.</div>'
    },
    {
        id: 'trash-management',
        category: 'favorites-trash',
        title: 'Trash Management',
        body:
            '<p>The Trash page shows all deleted items with their original paths and deletion dates.</p>' +
            '<h4>Actions</h4>' +
            '<ul>' +
                '<li><strong>Restore</strong> &mdash; Moves the file back to its original location</li>' +
                '<li><strong>Delete Permanently</strong> &mdash; Removes the file forever</li>' +
                '<li><strong>Empty Trash</strong> &mdash; Purges all trashed items</li>' +
            '</ul>'
    },

    // ── Groups ───────────────────────────────────────────────────────────
    {
        id: 'group-membership',
        category: 'groups',
        title: 'Group Membership',
        body:
            '<p>Users can belong to one or more groups. Groups control file access and storage locations.</p>' +
            '<ul>' +
                '<li>Admins create groups and add members</li>' +
                '<li>Each group can have its own storage backend and root directory</li>' +
                '<li>Members inherit the group\'s permissions</li>' +
            '</ul>'
    },
    {
        id: 'roles',
        category: 'groups',
        title: 'Roles (RBAC)',
        body:
            '<p>FruitSalade uses role-based access control with three roles:</p>' +
            '<table>' +
                '<tr><th>Role</th><th>Capabilities</th></tr>' +
                '<tr><td><code>admin</code></td><td>Full control: manage users, groups, settings, all files</td></tr>' +
                '<tr><td><code>editor</code></td><td>Upload, modify, and delete files within permitted areas</td></tr>' +
                '<tr><td><code>viewer</code></td><td>Read-only access to files within permitted areas</td></tr>' +
            '</table>' +
            '<h4>Effective Role</h4>' +
            '<p>A user\'s effective role is determined by walking up the group hierarchy. The highest role found in any ancestor group applies.</p>'
    },
    {
        id: 'nested-groups',
        category: 'groups',
        title: 'Nested Groups',
        body:
            '<p>Groups can be nested to create organizational hierarchies (e.g., Company &rarr; Department &rarr; Team).</p>' +
            '<ul>' +
                '<li>Child groups inherit parent permissions</li>' +
                '<li>A cycle-prevention mechanism ensures no circular nesting</li>' +
                '<li>The <code>Provisioner</code> auto-creates directory structures for new groups</li>' +
            '</ul>'
    },
    {
        id: 'group-permissions',
        category: 'groups',
        title: 'Group Permissions',
        body:
            '<p>Group permissions define what members can do with files in the group\'s storage location.</p>' +
            '<ul>' +
                '<li>Set at the group level and inherited by all members</li>' +
                '<li>Combined with per-file permissions for fine-grained control</li>' +
                '<li>The <code>visibility</code> field controls whether files are visible outside the group</li>' +
            '</ul>'
    },

    // ── Security ─────────────────────────────────────────────────────────
    {
        id: 'totp-2fa',
        category: 'security',
        title: '2FA / TOTP Setup',
        body:
            '<p>Two-factor authentication adds an extra layer of security using time-based one-time passwords.</p>' +
            '<h4>Enabling 2FA</h4>' +
            '<ol>' +
                '<li>Go to <strong>Dashboard &rarr; Security</strong></li>' +
                '<li>Click <strong>Enable 2FA</strong></li>' +
                '<li>Scan the QR code with an authenticator app (Google Authenticator, Authy, etc.)</li>' +
                '<li>Enter the 6-digit code to verify</li>' +
            '</ol>' +
            '<div class="wiki-warning">Save your backup codes in a safe place. They are the only way to recover access if you lose your authenticator device.</div>' +
            '<h4>Disabling 2FA</h4>' +
            '<p>Click <strong>Disable 2FA</strong> in the Security section and confirm.</p>'
    },
    {
        id: 'backup-codes',
        category: 'security',
        title: 'Backup Codes',
        body:
            '<p>When you enable 2FA, a set of single-use backup codes is generated.</p>' +
            '<ul>' +
                '<li>Each code can be used exactly once in place of a TOTP code</li>' +
                '<li>Store them securely offline (e.g., printed or in a password manager)</li>' +
                '<li>You can regenerate codes from the Security section (invalidating old ones)</li>' +
            '</ul>'
    },
    {
        id: 'sessions',
        category: 'security',
        title: 'Sessions',
        body:
            '<p>FruitSalade uses JWT tokens for session management.</p>' +
            '<ul>' +
                '<li>Tokens are stored in <code>sessionStorage</code> and cleared on tab close</li>' +
                '<li>Expired or invalid tokens trigger an automatic redirect to login</li>' +
                '<li>OIDC authentication is also supported for SSO environments</li>' +
            '</ul>'
    },

    // ── Clients ──────────────────────────────────────────────────────────
    {
        id: 'linux-fuse',
        category: 'clients',
        title: 'Linux FUSE Client',
        body:
            '<p>The FUSE client mounts FruitSalade as a local filesystem on Linux.</p>' +
            '<h4>Installation</h4>' +
            '<pre>make fuse\nsudo ./bin/fruitsalade-fuse -server https://your-server:48000 -mount /mnt/fruitsalade</pre>' +
            '<h4>Features</h4>' +
            '<ul>' +
                '<li><strong>On-demand files</strong>: Content is fetched only when you open a file</li>' +
                '<li><strong>LRU cache</strong>: Recently accessed files are cached locally</li>' +
                '<li><strong>Pinning</strong>: Pin files to keep them always available offline</li>' +
                '<li><strong>Write support</strong>: Create, edit, and delete files through the mount</li>' +
            '</ul>' +
            '<div class="wiki-note">The FUSE client requires <code>libfuse</code> to be installed. On Debian/Ubuntu: <code>apt install fuse3</code>.</div>' +
            '<h4>Systemd Service</h4>' +
            '<p>A systemd service file is provided in <code>deploy/</code> for running the FUSE client as a system service.</p>'
    },
    {
        id: 'windows-client',
        category: 'clients',
        title: 'Windows Client',
        body:
            '<p>The Windows client integrates with File Explorer using cloud file placeholders (similar to OneDrive).</p>' +
            '<h4>Backends</h4>' +
            '<table>' +
                '<tr><th>Backend</th><th>Description</th></tr>' +
                '<tr><td>CfAPI</td><td>Native Windows Cloud Files API (recommended)</td></tr>' +
                '<tr><td>CgoFUSE</td><td>WinFsp-based FUSE mount (cross-platform)</td></tr>' +
            '</table>' +
            '<h4>Features</h4>' +
            '<ul>' +
                '<li>File placeholders: files appear in Explorer but are downloaded on demand</li>' +
                '<li>Sync status overlay icons</li>' +
                '<li>Runs as a Windows Service</li>' +
            '</ul>' +
            '<div class="wiki-note">The CfAPI backend requires a C++ shim (<code>cfapi_shim.cpp</code>) and CGO for building.</div>'
    },

    // ── Admin ────────────────────────────────────────────────────────────
    {
        id: 'admin-users',
        category: 'admin',
        title: 'User Management',
        body:
            '<p>Admins can manage users from the <strong>Admin &rarr; Users</strong> page.</p>' +
            '<h4>Actions</h4>' +
            '<ul>' +
                '<li><strong>Create User</strong> &mdash; Add new users with username, password, and role</li>' +
                '<li><strong>Edit User</strong> &mdash; Change role, reset password</li>' +
                '<li><strong>Delete User</strong> &mdash; Remove a user account</li>' +
            '</ul>' +
            '<div class="wiki-warning">Deleting a user does not automatically delete their files. Reassign or clean up storage manually.</div>'
    },
    {
        id: 'admin-groups',
        category: 'admin',
        title: 'Group Administration',
        body:
            '<p>Manage groups from <strong>Admin &rarr; Groups</strong>.</p>' +
            '<ul>' +
                '<li>Create groups with a name, optional parent group, and storage location</li>' +
                '<li>Add or remove members</li>' +
                '<li>Set group-level visibility and roles</li>' +
                '<li>Nested groups are displayed as a tree structure</li>' +
            '</ul>'
    },
    {
        id: 'admin-storage',
        category: 'admin',
        title: 'Storage Backends',
        body:
            '<p>FruitSalade supports multiple storage backends that can be assigned to different groups.</p>' +
            '<h4>Backend Types</h4>' +
            '<table>' +
                '<tr><th>Backend</th><th>Config</th></tr>' +
                '<tr><td><strong>Local</strong></td><td>Path on the server filesystem</td></tr>' +
                '<tr><td><strong>S3</strong></td><td>S3-compatible (AWS, MinIO, Wasabi, etc.)</td></tr>' +
                '<tr><td><strong>SMB</strong></td><td>Windows file shares / CIFS mounts</td></tr>' +
            '</table>' +
            '<p>Configure storage locations from <strong>Admin &rarr; Storage</strong>. The storage router automatically directs uploads to the correct backend based on group membership.</p>'
    },
    {
        id: 'admin-quotas',
        category: 'admin',
        title: 'Quotas &amp; Rate Limiting',
        body:
            '<p>Control resource usage with per-user quotas and rate limits.</p>' +
            '<h4>Storage Quotas</h4>' +
            '<ul>' +
                '<li>Set a maximum storage size per user</li>' +
                '<li>The server rejects uploads that would exceed the quota</li>' +
            '</ul>' +
            '<h4>Rate Limiting</h4>' +
            '<ul>' +
                '<li>In-memory token bucket rate limiter</li>' +
                '<li>Daily bandwidth tracking per user</li>' +
                '<li>Configurable via environment variables</li>' +
            '</ul>'
    },
    {
        id: 'admin-config',
        category: 'admin',
        title: 'Configuration',
        body:
            '<p>Server configuration is managed via environment variables. Key settings:</p>' +
            '<table>' +
                '<tr><th>Variable</th><th>Description</th><th>Default</th></tr>' +
                '<tr><td><code>STORAGE_BACKEND</code></td><td>Default storage backend</td><td><code>local</code></td></tr>' +
                '<tr><td><code>LOCAL_STORAGE_PATH</code></td><td>Local storage directory</td><td><code>/data/storage</code></td></tr>' +
                '<tr><td><code>MAX_UPLOAD_SIZE</code></td><td>Max upload file size</td><td><code>100MB</code></td></tr>' +
                '<tr><td><code>JWT_SECRET</code></td><td>JWT signing key</td><td>(required)</td></tr>' +
                '<tr><td><code>DATABASE_URL</code></td><td>PostgreSQL connection string</td><td>(required)</td></tr>' +
            '</table>' +
            '<p>See <strong>Admin &rarr; Settings</strong> for runtime-editable settings.</p>'
    },
    {
        id: 'admin-analytics',
        category: 'admin',
        title: 'Analytics',
        body:
            '<p>The Dashboard provides storage analytics charts for admins:</p>' +
            '<ul>' +
                '<li><strong>Storage by Type</strong> &mdash; Donut chart of file types</li>' +
                '<li><strong>Upload Activity</strong> &mdash; Daily upload counts over time</li>' +
                '<li><strong>Top Users</strong> &mdash; Storage consumption by user</li>' +
                '<li><strong>Storage Growth</strong> &mdash; Cumulative storage over time</li>' +
            '</ul>' +
            '<p>Prometheus metrics are also exported at <code>/metrics</code> for external monitoring with Grafana.</p>'
    },
    {
        id: 'admin-gallery-plugins',
        category: 'admin',
        title: 'Gallery Plugins',
        body:
            '<p>Gallery plugins add automatic processing to uploaded media files.</p>' +
            '<ul>' +
                '<li><strong>EXIF Extraction</strong> &mdash; Reads camera metadata from photos</li>' +
                '<li><strong>Thumbnail Generation</strong> &mdash; Creates preview thumbnails</li>' +
                '<li><strong>AI Tagging</strong> &mdash; Auto-classify images with tags</li>' +
            '</ul>' +
            '<p>Manage plugins from <strong>Admin &rarr; Gallery Plugins</strong>. Each plugin can be enabled or disabled independently.</p>'
    },

    // ── Docker ───────────────────────────────────────────────────────────
    {
        id: 'docker-compose',
        category: 'docker',
        title: 'Docker Compose Setup',
        body:
            '<p>The recommended deployment method uses Docker Compose.</p>' +
            '<pre>make docker      # Build images\nmake docker-up   # Start all services</pre>' +
            '<h4>Services</h4>' +
            '<table>' +
                '<tr><th>Service</th><th>Description</th><th>Port</th></tr>' +
                '<tr><td><code>server</code></td><td>FruitSalade server (embedded PostgreSQL)</td><td>48000</td></tr>' +
                '<tr><td><code>minio</code></td><td>S3-compatible object storage</td><td>48001</td></tr>' +
                '<tr><td><code>client-a</code></td><td>FUSE client instance</td><td>48002</td></tr>' +
                '<tr><td><code>client-b</code></td><td>FUSE client instance</td><td>48003</td></tr>' +
            '</table>' +
            '<div class="wiki-tip">Run <code>make docker-logs</code> to follow logs from all containers.</div>'
    },
    {
        id: 'docker-standalone',
        category: 'docker',
        title: 'Standalone Docker',
        body:
            '<p>For a simpler setup without S3 or FUSE clients:</p>' +
            '<pre>make docker-run</pre>' +
            '<p>This starts a single container with local filesystem storage and an embedded PostgreSQL database.</p>' +
            '<ul>' +
                '<li>API: <code>http://localhost:48000</code></li>' +
                '<li>Web app: <code>http://localhost:48000/app/</code></li>' +
            '</ul>'
    },
    {
        id: 'docker-env-vars',
        category: 'docker',
        title: 'Environment Variables',
        body:
            '<p>Key environment variables for Docker deployment:</p>' +
            '<table>' +
                '<tr><th>Variable</th><th>Description</th></tr>' +
                '<tr><td><code>STORAGE_BACKEND</code></td><td>Storage type: <code>local</code> or <code>s3</code></td></tr>' +
                '<tr><td><code>LOCAL_STORAGE_PATH</code></td><td>Path for local storage (default: <code>/data/storage</code>)</td></tr>' +
                '<tr><td><code>S3_BUCKET</code></td><td>S3 bucket name</td></tr>' +
                '<tr><td><code>S3_ENDPOINT</code></td><td>S3 endpoint URL (for MinIO/custom S3)</td></tr>' +
                '<tr><td><code>S3_REGION</code></td><td>S3 region</td></tr>' +
                '<tr><td><code>JWT_SECRET</code></td><td>Secret key for JWT signing</td></tr>' +
                '<tr><td><code>DATABASE_URL</code></td><td>PostgreSQL connection string</td></tr>' +
                '<tr><td><code>DOCKER_API_VERSION</code></td><td>Set to <code>1.41</code> for Docker 20.10 compatibility</td></tr>' +
            '</table>'
    },
    {
        id: 'docker-ports',
        category: 'docker',
        title: 'Port Reference',
        body:
            '<p>Default port mappings in Docker Compose:</p>' +
            '<table>' +
                '<tr><th>Port</th><th>Service</th></tr>' +
                '<tr><td><code>48000</code></td><td>FruitSalade API + Web App</td></tr>' +
                '<tr><td><code>48001</code></td><td>MinIO Console / Standalone API</td></tr>' +
                '<tr><td><code>48002</code></td><td>FUSE Client A</td></tr>' +
                '<tr><td><code>48003</code></td><td>FUSE Client B</td></tr>' +
            '</table>' +
            '<div class="wiki-note">These ports are chosen to avoid conflicts with common services. Adjust in <code>docker-compose.yml</code> as needed.</div>'
    },

    // ── API Reference ────────────────────────────────────────────────────
    {
        id: 'api-auth',
        category: 'api-reference',
        title: 'Authentication',
        body:
            '<p>The API uses JWT bearer tokens for authentication.</p>' +
            '<h4>Login</h4>' +
            '<pre>POST /api/v1/login\nContent-Type: application/json\n\n{"username": "admin", "password": "admin"}</pre>' +
            '<p>Returns a JSON object with a <code>token</code> field. Include this token in subsequent requests:</p>' +
            '<pre>Authorization: Bearer &lt;token&gt;</pre>' +
            '<h4>OIDC</h4>' +
            '<p>For SSO environments, the server supports OpenID Connect via the <code>go-oidc/v3</code> library. Configure OIDC providers via environment variables.</p>'
    },
    {
        id: 'api-file-ops',
        category: 'api-reference',
        title: 'File Operations',
        body:
            '<h4>Metadata</h4>' +
            '<pre>GET /api/v1/tree              # Full file tree\nGET /api/v1/tree/{path}       # Subtree at path</pre>' +
            '<h4>Content</h4>' +
            '<pre>GET /api/v1/content/{id}      # Download file (supports Range header)\nPOST /api/v1/upload           # Upload file (multipart/form-data)\nDELETE /api/v1/files/{id}     # Delete file (moves to trash)</pre>' +
            '<h4>Versioning</h4>' +
            '<pre>GET /api/v1/versions/{id}     # List versions\nPOST /api/v1/versions/{id}/restore/{v}  # Restore version</pre>' +
            '<div class="wiki-tip">Include <code>X-Expected-Version</code> in upload requests for conflict detection.</div>'
    },
    {
        id: 'api-endpoints',
        category: 'api-reference',
        title: 'Endpoint Reference',
        body:
            '<p>Key API endpoint groups:</p>' +
            '<table>' +
                '<tr><th>Path Prefix</th><th>Description</th></tr>' +
                '<tr><td><code>/api/v1/tree</code></td><td>File tree metadata</td></tr>' +
                '<tr><td><code>/api/v1/content</code></td><td>File content (download/stream)</td></tr>' +
                '<tr><td><code>/api/v1/upload</code></td><td>File upload</td></tr>' +
                '<tr><td><code>/api/v1/versions</code></td><td>Version management</td></tr>' +
                '<tr><td><code>/api/v1/shares</code></td><td>Share link management</td></tr>' +
                '<tr><td><code>/api/v1/trash</code></td><td>Trash operations</td></tr>' +
                '<tr><td><code>/api/v1/favorites</code></td><td>Favorites</td></tr>' +
                '<tr><td><code>/api/v1/gallery</code></td><td>Gallery (tags, albums, EXIF)</td></tr>' +
                '<tr><td><code>/api/v1/groups</code></td><td>Group management</td></tr>' +
                '<tr><td><code>/api/v1/users</code></td><td>User management (admin)</td></tr>' +
                '<tr><td><code>/api/v1/events</code></td><td>SSE real-time events</td></tr>' +
                '<tr><td><code>/api/v1/totp</code></td><td>2FA / TOTP management</td></tr>' +
                '<tr><td><code>/health</code></td><td>Health check</td></tr>' +
                '<tr><td><code>/metrics</code></td><td>Prometheus metrics</td></tr>' +
            '</table>'
    },

    // ── Credits ──────────────────────────────────────────────────────────
    {
        id: 'credits-libraries',
        category: 'credits',
        title: 'Go Libraries',
        body:
            '<p>FruitSalade is built on these excellent open-source Go libraries:</p>' +
            '<h4>Core Dependencies</h4>' +
            '<table>' +
                '<tr><th>Library</th><th>Used For</th></tr>' +
                '<tr><td><code>github.com/hanwen/go-fuse/v2</code></td><td>Linux FUSE filesystem &mdash; mounts FruitSalade as a local directory with on-demand file hydration</td></tr>' +
                '<tr><td><code>github.com/aws/aws-sdk-go-v2</code></td><td>S3 storage backend &mdash; file storage on AWS S3, MinIO, Wasabi, and other S3-compatible services</td></tr>' +
                '<tr><td><code>github.com/lib/pq</code></td><td>PostgreSQL driver &mdash; all metadata, users, groups, permissions, versions, and gallery data</td></tr>' +
                '<tr><td><code>github.com/golang-jwt/jwt/v5</code></td><td>JWT authentication &mdash; stateless session tokens for API and web app</td></tr>' +
                '<tr><td><code>github.com/coreos/go-oidc/v3</code></td><td>OpenID Connect &mdash; SSO integration with external identity providers</td></tr>' +
                '<tr><td><code>github.com/pquerna/otp</code></td><td>TOTP / 2FA &mdash; time-based one-time passwords with QR code generation</td></tr>' +
                '<tr><td><code>github.com/prometheus/client_golang</code></td><td>Prometheus metrics &mdash; request counts, latencies, storage usage exported at <code>/metrics</code></td></tr>' +
                '<tr><td><code>go.uber.org/zap</code></td><td>Structured logging &mdash; high-performance JSON/console logging throughout the server</td></tr>' +
                '<tr><td><code>golang.org/x/crypto</code></td><td>Password hashing &mdash; bcrypt for secure credential storage</td></tr>' +
                '<tr><td><code>golang.org/x/net</code></td><td>WebDAV handler &mdash; standards-compliant WebDAV file access</td></tr>' +
                '<tr><td><code>github.com/winfsp/cgofuse</code></td><td>Windows FUSE &mdash; cross-platform FUSE via WinFsp for the Windows client backend</td></tr>' +
            '</table>' +
            '<h4>Media Processing</h4>' +
            '<table>' +
                '<tr><th>Library</th><th>Used For</th></tr>' +
                '<tr><td><code>github.com/rwcarlsen/goexif</code></td><td>EXIF extraction &mdash; reads camera metadata, GPS coordinates from photos</td></tr>' +
                '<tr><td><code>github.com/disintegration/imaging</code></td><td>Image processing &mdash; thumbnail generation and image resizing</td></tr>' +
                '<tr><td><code>github.com/boombuler/barcode</code></td><td>QR code generation &mdash; TOTP setup QR codes for authenticator apps</td></tr>' +
            '</table>' +
            '<h4>Indirect / Supporting</h4>' +
            '<table>' +
                '<tr><th>Library</th><th>Used For</th></tr>' +
                '<tr><td><code>github.com/go-jose/go-jose/v4</code></td><td>JOSE / JWK &mdash; JSON Web Key handling for OIDC token verification</td></tr>' +
                '<tr><td><code>golang.org/x/oauth2</code></td><td>OAuth 2.0 flows &mdash; token exchange for OIDC authentication</td></tr>' +
                '<tr><td><code>golang.org/x/image</code></td><td>Extended image formats &mdash; additional codec support for imaging library</td></tr>' +
                '<tr><td><code>golang.org/x/sys</code></td><td>OS-level syscalls &mdash; FUSE mount operations and signal handling</td></tr>' +
                '<tr><td><code>golang.org/x/term</code></td><td>Terminal handling &mdash; password prompts in CLI clients</td></tr>' +
                '<tr><td><code>github.com/aws/smithy-go</code></td><td>AWS SDK foundation &mdash; serialization and protocol support for S3</td></tr>' +
                '<tr><td><code>github.com/klauspost/compress</code></td><td>Compression &mdash; fast zstd/gzip support used by Prometheus client</td></tr>' +
                '<tr><td><code>google.golang.org/protobuf</code></td><td>Protocol Buffers &mdash; Prometheus metric serialization format</td></tr>' +
                '<tr><td><code>go.uber.org/multierr</code></td><td>Error aggregation &mdash; combines multiple errors in deferred cleanup paths</td></tr>' +
            '</table>'
    },
    {
        id: 'credits-project',
        category: 'credits',
        title: 'What We Built',
        body:
            '<p>FruitSalade is a <strong>self-hosted, Docker-deployable file synchronization system</strong> with on-demand file placeholders &mdash; similar to OneDrive or Dropbox "Files On-Demand" but fully under your control.</p>' +
            '<h4>Server</h4>' +
            '<ul>' +
                '<li><strong>REST API</strong> &mdash; 127+ endpoints for files, metadata, versioning, sharing, gallery, groups, quotas, 2FA, and admin operations</li>' +
                '<li><strong>Multi-backend storage</strong> &mdash; S3, local filesystem, and SMB/CIFS with per-group routing</li>' +
                '<li><strong>PostgreSQL metadata</strong> &mdash; 12 migrations covering files, versions, permissions, groups, RBAC, quotas, gallery, TOTP, and trash</li>' +
                '<li><strong>File versioning</strong> &mdash; automatic version history with conflict detection via <code>X-Expected-Version</code> and ETag headers</li>' +
                '<li><strong>Real-time sync</strong> &mdash; Server-Sent Events (SSE) broadcaster for live file change notifications</li>' +
                '<li><strong>Sharing</strong> &mdash; ACL-based permissions with path inheritance, plus public share links with password protection, expiry, and download limits</li>' +
                '<li><strong>Groups &amp; RBAC</strong> &mdash; nested groups with admin/editor/viewer roles, cycle-prevention, and automatic directory provisioning</li>' +
                '<li><strong>Quotas &amp; rate limiting</strong> &mdash; per-user storage quotas, daily bandwidth tracking, and in-memory token bucket rate limiter</li>' +
                '<li><strong>Authentication</strong> &mdash; JWT sessions, OIDC/SSO support, TOTP 2FA with backup codes</li>' +
                '<li><strong>Monitoring</strong> &mdash; Prometheus metrics, Grafana dashboard, structured zap logging</li>' +
                '<li><strong>Gallery</strong> &mdash; photo/video indexing with EXIF extraction, manual and auto tags, albums, geolocated photo map, and plugin system</li>' +
            '</ul>' +
            '<h4>Web Application</h4>' +
            '<ul>' +
                '<li><strong>18 views</strong> &mdash; file browser, dashboard, gallery, versions, sharing, trash, favorites, search, groups, users, storage, settings, and more</li>' +
                '<li><strong>Vanilla JS</strong> &mdash; no framework, no build step, embedded via <code>go:embed</code> and served at <code>/app/</code></li>' +
                '<li><strong>Gallery lightbox</strong> &mdash; fullscreen viewer with EXIF panel, tags, albums, mini-map, and sharing</li>' +
                '<li><strong>Conflict resolution UI</strong> &mdash; fullscreen compare view with folder grouping and multi-type preview</li>' +
                '<li><strong>Dark mode</strong> &mdash; system preference detection with manual toggle</li>' +
                '<li><strong>PWA</strong> &mdash; installable progressive web app with offline shell caching</li>' +
                '<li><strong>Responsive</strong> &mdash; mobile-first with bottom tab bar, collapsible sidebar, and touch-friendly controls</li>' +
            '</ul>' +
            '<h4>Desktop Clients</h4>' +
            '<ul>' +
                '<li><strong>Linux FUSE client</strong> &mdash; mounts as a local filesystem with on-demand content fetching, LRU cache, and file pinning</li>' +
                '<li><strong>Windows client</strong> &mdash; dual backend: native Cloud Files API (CfAPI) with C++ shim, or cross-platform WinFsp FUSE; runs as a Windows Service</li>' +
            '</ul>' +
            '<h4>Infrastructure</h4>' +
            '<ul>' +
                '<li><strong>Docker</strong> &mdash; two images (server with embedded PostgreSQL, client with FUSE); Docker Compose with MinIO S3</li>' +
                '<li><strong>CI/CD</strong> &mdash; GitHub Actions pipeline for build, test, and lint</li>' +
                '<li><strong>Systemd</strong> &mdash; service files for server and FUSE client deployment</li>' +
                '<li><strong>TLS 1.3</strong> &mdash; built-in HTTPS with automatic certificate handling</li>' +
            '</ul>'
    },
    {
        id: 'credits-license',
        category: 'credits',
        title: 'License &amp; Acknowledgments',
        body:
            '<p>FruitSalade is open-source software.</p>' +
            '<p>All third-party Go libraries are used under their respective open-source licenses (Apache 2.0, MIT, BSD). We are grateful to the maintainers and contributors of every dependency listed above.</p>' +
            '<h4>Special Thanks</h4>' +
            '<ul>' +
                '<li>The <strong>Go team</strong> at Google for an incredible language and standard library</li>' +
                '<li><strong>hanwen/go-fuse</strong> for making FUSE filesystems in Go practical</li>' +
                '<li>The <strong>AWS SDK for Go</strong> team for a well-designed S3 client</li>' +
                '<li><strong>Uber</strong> for the <code>zap</code> logger &mdash; fast, structured, and a joy to use</li>' +
                '<li>The <strong>Prometheus</strong> community for the monitoring ecosystem</li>' +
                '<li><strong>WinFsp / cgofuse</strong> for bridging FUSE to Windows</li>' +
                '<li>The <strong>Leaflet</strong> project for the interactive photo map</li>' +
            '</ul>' +
            '<div class="wiki-tip">Built with Go ' + '1.24' + ', vanilla HTML/CSS/JS, and a lot of coffee.</div>'
    }
];

// ── State ────────────────────────────────────────────────────────────────
var _helpActiveCategory = 'getting-started';
var _helpScrollSpyObserver = null;
var _helpSearchTimer = null;

// ── Entry Point ──────────────────────────────────────────────────────────
function renderHelp() {
    var app = document.getElementById('app');
    app.innerHTML = buildHelpShell();
    wireHelpTabs();
    wireHelpSearch();
    renderHelpCategory(_helpActiveCategory);
}

// ── Shell (toolbar + search + tabs + layout) ─────────────────────────────
function buildHelpShell() {
    var tabs = '';
    for (var i = 0; i < HELP_CATEGORIES.length; i++) {
        var c = HELP_CATEGORIES[i];
        tabs += '<button class="fm-tab' + (c.id === _helpActiveCategory ? ' active' : '') +
            '" data-help-cat="' + c.id + '">' + c.label + '</button>';
    }

    return '' +
        '<div class="toolbar">' +
            '<h2>Help</h2>' +
            '<div class="toolbar-actions">' +
                '<input type="text" id="help-search-input" class="form-input" ' +
                    'placeholder="Search documentation..." style="width:260px;max-width:100%">' +
            '</div>' +
        '</div>' +
        '<div class="fm-tab-nav" id="help-tab-nav">' + tabs + '</div>' +
        '<div class="wiki-layout" id="wiki-layout">' +
            '<nav class="wiki-toc" id="wiki-toc">' +
                '<button class="wiki-toc-toggle" id="wiki-toc-toggle">Table of Contents</button>' +
                '<div class="wiki-toc-body" id="wiki-toc-body"></div>' +
            '</nav>' +
            '<div class="wiki-content" id="wiki-content"></div>' +
        '</div>';
}

// ── Render Category ──────────────────────────────────────────────────────
function renderHelpCategory(catId) {
    _helpActiveCategory = catId;

    // Update tab highlight
    var tabs = document.querySelectorAll('#help-tab-nav .fm-tab');
    for (var i = 0; i < tabs.length; i++) {
        if (tabs[i].getAttribute('data-help-cat') === catId) {
            tabs[i].classList.add('active');
        } else {
            tabs[i].classList.remove('active');
        }
    }

    var articles = [];
    for (var i = 0; i < HELP_ARTICLES.length; i++) {
        if (HELP_ARTICLES[i].category === catId) articles.push(HELP_ARTICLES[i]);
    }

    // Build ToC
    var tocHtml = '<div class="wiki-toc-title">Contents</div>';
    for (var i = 0; i < articles.length; i++) {
        tocHtml += '<a class="wiki-toc-link" data-toc-target="' + articles[i].id + '">' +
            articles[i].title + '</a>';
    }
    document.getElementById('wiki-toc-body').innerHTML = tocHtml;

    // Build content
    var html = '';
    for (var i = 0; i < articles.length; i++) {
        var a = articles[i];
        html += '<section class="wiki-article" id="wiki-art-' + a.id + '">' +
            '<h3 class="wiki-article-title">' + a.title + '</h3>' +
            '<div class="wiki-article-body">' + a.body + '</div>' +
        '</section>';
    }
    document.getElementById('wiki-content').innerHTML = html;

    // Wire ToC links
    var tocLinks = document.querySelectorAll('#wiki-toc-body .wiki-toc-link');
    for (var i = 0; i < tocLinks.length; i++) {
        tocLinks[i].addEventListener('click', (function(link) {
            return function() {
                var target = document.getElementById('wiki-art-' + link.getAttribute('data-toc-target'));
                if (target) target.scrollIntoView({ behavior: 'smooth', block: 'start' });
                // On mobile close ToC
                var body = document.getElementById('wiki-toc-body');
                if (window.innerWidth <= 768) body.classList.remove('open');
            };
        })(tocLinks[i]));
    }

    // Wire cross-article links
    var wikiLinks = document.querySelectorAll('#wiki-content .wiki-link');
    for (var i = 0; i < wikiLinks.length; i++) {
        wikiLinks[i].addEventListener('click', (function(link) {
            return function(e) {
                e.preventDefault();
                navigateToHelpArticle(link.getAttribute('data-wiki-link'));
            };
        })(wikiLinks[i]));
    }

    // Setup scroll spy
    setupHelpScrollSpy(articles);

    // Clear search
    var searchInput = document.getElementById('help-search-input');
    if (searchInput) searchInput.value = '';
}

// ── Search ───────────────────────────────────────────────────────────────
function wireHelpSearch() {
    var input = document.getElementById('help-search-input');
    if (!input) return;

    input.addEventListener('input', function() {
        clearTimeout(_helpSearchTimer);
        var q = input.value.trim();
        _helpSearchTimer = setTimeout(function() {
            if (q.length < 2) {
                renderHelpCategory(_helpActiveCategory);
                return;
            }
            renderHelpSearchResults(q);
        }, 250);
    });
}

function renderHelpSearchResults(query) {
    var lower = query.toLowerCase();
    var results = [];

    for (var i = 0; i < HELP_ARTICLES.length; i++) {
        var a = HELP_ARTICLES[i];
        var plain = a.title + ' ' + helpStripHtml(a.body);
        if (plain.toLowerCase().indexOf(lower) !== -1) {
            results.push(a);
        }
    }

    // Build ToC from results
    var tocHtml = '<div class="wiki-toc-title">Search Results (' + results.length + ')</div>';
    for (var i = 0; i < results.length; i++) {
        var catLabel = '';
        for (var j = 0; j < HELP_CATEGORIES.length; j++) {
            if (HELP_CATEGORIES[j].id === results[i].category) {
                catLabel = HELP_CATEGORIES[j].label;
                break;
            }
        }
        tocHtml += '<a class="wiki-toc-link" data-toc-target="' + results[i].id + '">' +
            '<span class="wiki-toc-category">' + catLabel + '</span>' +
            results[i].title + '</a>';
    }
    document.getElementById('wiki-toc-body').innerHTML = tocHtml;

    // Build content
    var html = '';
    if (results.length === 0) {
        html = '<div class="wiki-no-results">' +
            '<span style="font-size:2rem;display:block;margin-bottom:0.5rem">&#128269;</span>' +
            'No results for "<strong>' + escapeHtml(query) + '</strong>"' +
        '</div>';
    } else {
        html = '<div class="wiki-search-results-header">' +
            results.length + ' result' + (results.length !== 1 ? 's' : '') +
            ' for "<strong>' + escapeHtml(query) + '</strong>"</div>';
        for (var i = 0; i < results.length; i++) {
            var a = results[i];
            html += '<section class="wiki-article" id="wiki-art-' + a.id + '">' +
                '<h3 class="wiki-article-title">' + highlightMatch(a.title, query) + '</h3>' +
                '<div class="wiki-article-body">' + highlightHtmlContent(a.body, query) + '</div>' +
            '</section>';
        }
    }
    document.getElementById('wiki-content').innerHTML = html;

    // Deactivate tabs
    var tabs = document.querySelectorAll('#help-tab-nav .fm-tab');
    for (var i = 0; i < tabs.length; i++) tabs[i].classList.remove('active');

    // Wire ToC links
    var tocLinks = document.querySelectorAll('#wiki-toc-body .wiki-toc-link');
    for (var i = 0; i < tocLinks.length; i++) {
        tocLinks[i].addEventListener('click', (function(link) {
            return function() {
                var target = document.getElementById('wiki-art-' + link.getAttribute('data-toc-target'));
                if (target) target.scrollIntoView({ behavior: 'smooth', block: 'start' });
            };
        })(tocLinks[i]));
    }

    // Wire cross-article links
    var wikiLinks = document.querySelectorAll('#wiki-content .wiki-link');
    for (var i = 0; i < wikiLinks.length; i++) {
        wikiLinks[i].addEventListener('click', (function(link) {
            return function(e) {
                e.preventDefault();
                navigateToHelpArticle(link.getAttribute('data-wiki-link'));
            };
        })(wikiLinks[i]));
    }
}

// ── Tabs ─────────────────────────────────────────────────────────────────
function wireHelpTabs() {
    var nav = document.getElementById('help-tab-nav');
    if (!nav) return;
    nav.addEventListener('click', function(e) {
        var btn = e.target.closest('.fm-tab');
        if (!btn) return;
        var catId = btn.getAttribute('data-help-cat');
        if (catId) {
            document.getElementById('help-search-input').value = '';
            renderHelpCategory(catId);
        }
    });

    // Mobile ToC toggle
    var toggle = document.getElementById('wiki-toc-toggle');
    if (toggle) {
        toggle.addEventListener('click', function() {
            var body = document.getElementById('wiki-toc-body');
            body.classList.toggle('open');
        });
    }
}

// ── Scroll Spy ───────────────────────────────────────────────────────────
function setupHelpScrollSpy(articles) {
    if (_helpScrollSpyObserver) {
        _helpScrollSpyObserver.disconnect();
        _helpScrollSpyObserver = null;
    }
    if (!articles || articles.length === 0) return;
    if (!('IntersectionObserver' in window)) return;

    _helpScrollSpyObserver = new IntersectionObserver(function(entries) {
        for (var i = 0; i < entries.length; i++) {
            if (entries[i].isIntersecting) {
                var artId = entries[i].target.id.replace('wiki-art-', '');
                var tocLinks = document.querySelectorAll('#wiki-toc-body .wiki-toc-link');
                for (var j = 0; j < tocLinks.length; j++) {
                    if (tocLinks[j].getAttribute('data-toc-target') === artId) {
                        tocLinks[j].classList.add('active');
                    } else {
                        tocLinks[j].classList.remove('active');
                    }
                }
                break; // only highlight the first intersecting
            }
        }
    }, {
        rootMargin: '-80px 0px -60% 0px',
        threshold: 0
    });

    for (var i = 0; i < articles.length; i++) {
        var el = document.getElementById('wiki-art-' + articles[i].id);
        if (el) _helpScrollSpyObserver.observe(el);
    }
}

// ── Cross-Article Navigation ─────────────────────────────────────────────
function navigateToHelpArticle(articleId) {
    // Find the article and its category
    var article = null;
    for (var i = 0; i < HELP_ARTICLES.length; i++) {
        if (HELP_ARTICLES[i].id === articleId) {
            article = HELP_ARTICLES[i];
            break;
        }
    }
    if (!article) return;

    // Switch to category if different
    if (article.category !== _helpActiveCategory) {
        renderHelpCategory(article.category);
    }

    // Scroll to article
    setTimeout(function() {
        var el = document.getElementById('wiki-art-' + articleId);
        if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }, 50);
}

// ── Helpers ──────────────────────────────────────────────────────────────
function helpStripHtml(html) {
    var tmp = document.createElement('div');
    tmp.innerHTML = html;
    return tmp.textContent || tmp.innerText || '';
}

function escapeHtml(str) {
    var div = document.createElement('div');
    div.appendChild(document.createTextNode(str));
    return div.innerHTML;
}

function highlightMatch(text, query) {
    if (!query) return text;
    var escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    var re = new RegExp('(' + escaped + ')', 'gi');
    return text.replace(re, '<mark>$1</mark>');
}

function highlightHtmlContent(html, query) {
    if (!query) return html;
    // Only highlight text nodes (not inside tags)
    var escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    var re = new RegExp('(' + escaped + ')', 'gi');
    // Split on HTML tags, highlight only non-tag parts
    return html.replace(/(>[^<]*)/gi, function(match) {
        return match.replace(re, '<mark>$1</mark>');
    });
}
