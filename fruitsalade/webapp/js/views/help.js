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
    { id: 'shortcuts',       label: 'Shortcuts' },
    { id: 'webdav',          label: 'WebDAV' },
    { id: 'monitoring',      label: 'Monitoring' },
    { id: 'troubleshooting', label: 'Troubleshooting' },
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

    // ── Keyboard Shortcuts ─────────────────────────────────────────────
    {
        id: 'shortcuts-global',
        category: 'shortcuts',
        title: 'Global Shortcuts',
        body:
            '<p>These shortcuts work from anywhere in the app:</p>' +
            '<table>' +
                '<tr><th>Shortcut</th><th>Action</th></tr>' +
                '<tr><td><kbd>Ctrl</kbd>+<kbd>K</kbd></td><td>Focus global search bar</td></tr>' +
                '<tr><td><kbd>Cmd</kbd>+<kbd>K</kbd></td><td>Focus global search bar (macOS)</td></tr>' +
                '<tr><td><kbd>Escape</kbd></td><td>Close any open modal or overlay</td></tr>' +
            '</table>'
    },
    {
        id: 'shortcuts-browser',
        category: 'shortcuts',
        title: 'File Browser',
        body:
            '<p>Shortcuts available in the file browser view:</p>' +
            '<table>' +
                '<tr><th>Shortcut</th><th>Action</th></tr>' +
                '<tr><td><kbd>Enter</kbd></td><td>Confirm rename when editing a file or folder name</td></tr>' +
                '<tr><td><kbd>Escape</kbd></td><td>Cancel rename and revert to original name</td></tr>' +
            '</table>'
    },
    {
        id: 'shortcuts-gallery',
        category: 'shortcuts',
        title: 'Gallery &amp; Lightbox',
        body:
            '<p>Shortcuts for the gallery lightbox (fullscreen photo viewer):</p>' +
            '<table>' +
                '<tr><th>Shortcut</th><th>Action</th></tr>' +
                '<tr><td><kbd>&larr;</kbd> Arrow Left</td><td>Previous photo</td></tr>' +
                '<tr><td><kbd>&rarr;</kbd> Arrow Right</td><td>Next photo</td></tr>' +
                '<tr><td><kbd>Escape</kbd></td><td>Close the lightbox</td></tr>' +
                '<tr><td><kbd>Enter</kbd></td><td>Submit tag in the tag input field</td></tr>' +
            '</table>' +
            '<div class="wiki-tip">On touch devices, swipe left/right to navigate between photos.</div>'
    },
    {
        id: 'shortcuts-conflicts',
        category: 'shortcuts',
        title: 'Conflict Viewer',
        body:
            '<p>Shortcuts in the fullscreen conflict comparison overlay:</p>' +
            '<table>' +
                '<tr><th>Shortcut</th><th>Action</th></tr>' +
                '<tr><td><kbd>&larr;</kbd> Arrow Left</td><td>Previous conflict</td></tr>' +
                '<tr><td><kbd>&rarr;</kbd> Arrow Right</td><td>Next conflict</td></tr>' +
                '<tr><td><kbd>Escape</kbd></td><td>Close the comparison overlay</td></tr>' +
            '</table>'
    },
    {
        id: 'shortcuts-search',
        category: 'shortcuts',
        title: 'Search',
        body:
            '<p>Shortcuts on the search page:</p>' +
            '<table>' +
                '<tr><th>Shortcut</th><th>Action</th></tr>' +
                '<tr><td><kbd>Enter</kbd></td><td>Execute search query</td></tr>' +
            '</table>' +
            '<p>The search input auto-focuses when you navigate to the search page.</p>'
    },

    // ── WebDAV ───────────────────────────────────────────────────────────
    {
        id: 'webdav-overview',
        category: 'webdav',
        title: 'WebDAV Overview',
        body:
            '<p>FruitSalade exposes a WebDAV endpoint at <code>/webdav</code>, allowing you to mount your files as a network drive from any WebDAV-compatible client.</p>' +
            '<h4>Supported Operations</h4>' +
            '<table>' +
                '<tr><th>WebDAV Method</th><th>Description</th></tr>' +
                '<tr><td><code>GET</code> / <code>PUT</code></td><td>Download and upload files</td></tr>' +
                '<tr><td><code>MKCOL</code></td><td>Create directories</td></tr>' +
                '<tr><td><code>DELETE</code></td><td>Delete files and directories</td></tr>' +
                '<tr><td><code>MOVE</code> / <code>COPY</code></td><td>Rename, move, or copy files</td></tr>' +
                '<tr><td><code>PROPFIND</code></td><td>List directory contents and file properties</td></tr>' +
                '<tr><td><code>LOCK</code> / <code>UNLOCK</code></td><td>In-memory file locking for concurrent editing</td></tr>' +
            '</table>' +
            '<div class="wiki-note">WebDAV writes go through the same storage router as the API, so multi-backend storage and versioning are fully supported.</div>'
    },
    {
        id: 'webdav-auth',
        category: 'webdav',
        title: 'Authentication',
        body:
            '<p>The WebDAV endpoint supports two authentication methods:</p>' +
            '<h4>HTTP Basic Auth</h4>' +
            '<p>Most WebDAV clients use Basic Auth. Provide your FruitSalade username and password when prompted.</p>' +
            '<pre>https://your-server:48000/webdav</pre>' +
            '<h4>Bearer Token</h4>' +
            '<p>Programmatic clients can pass a JWT token via the <code>Authorization</code> header:</p>' +
            '<pre>Authorization: Bearer &lt;token&gt;</pre>' +
            '<div class="wiki-warning">Basic Auth sends credentials in base64 (not encrypted). Always use HTTPS when connecting via WebDAV.</div>'
    },
    {
        id: 'webdav-clients',
        category: 'webdav',
        title: 'Client Setup',
        body:
            '<h4>Windows (Map Network Drive)</h4>' +
            '<ol>' +
                '<li>Open File Explorer &rarr; right-click "This PC" &rarr; "Map network drive"</li>' +
                '<li>Enter: <code>https://your-server:48000/webdav</code></li>' +
                '<li>Check "Connect using different credentials"</li>' +
                '<li>Enter your FruitSalade username and password</li>' +
            '</ol>' +
            '<h4>macOS (Finder)</h4>' +
            '<ol>' +
                '<li>Finder &rarr; Go &rarr; Connect to Server (<kbd>Cmd</kbd>+<kbd>K</kbd>)</li>' +
                '<li>Enter: <code>https://your-server:48000/webdav</code></li>' +
                '<li>Authenticate with your credentials</li>' +
            '</ol>' +
            '<h4>Linux (GNOME Files / Nautilus)</h4>' +
            '<ol>' +
                '<li>Open Files &rarr; Other Locations</li>' +
                '<li>Enter: <code>davs://your-server:48000/webdav</code></li>' +
                '<li>Enter your credentials</li>' +
            '</ol>' +
            '<h4>Command Line (curl)</h4>' +
            '<pre># List files\ncurl -u admin:admin https://your-server:48000/webdav/ -X PROPFIND\n\n# Upload a file\ncurl -u admin:admin -T myfile.txt https://your-server:48000/webdav/myfile.txt\n\n# Download a file\ncurl -u admin:admin -o output.txt https://your-server:48000/webdav/myfile.txt</pre>' +
            '<div class="wiki-tip">For the best experience with large file collections, use the native FUSE or Windows client instead of WebDAV. They support on-demand placeholders and local caching.</div>'
    },
    {
        id: 'webdav-limitations',
        category: 'webdav',
        title: 'Limitations',
        body:
            '<p>WebDAV is a convenient fallback but has trade-offs compared to the native clients:</p>' +
            '<ul>' +
                '<li><strong>No placeholders</strong> &mdash; all files appear as fully downloaded; no on-demand hydration</li>' +
                '<li><strong>No local cache</strong> &mdash; every read hits the server (no LRU cache or pinning)</li>' +
                '<li><strong>Performance</strong> &mdash; listing large directories can be slow due to PROPFIND overhead</li>' +
                '<li><strong>Locking</strong> &mdash; locks are in-memory only and lost on server restart</li>' +
            '</ul>' +
            '<p>For these reasons, the FUSE client (Linux) or CfAPI client (Windows) are recommended for daily use.</p>'
    },

    // ── Monitoring ───────────────────────────────────────────────────────
    {
        id: 'monitoring-overview',
        category: 'monitoring',
        title: 'Monitoring Overview',
        body:
            '<p>FruitSalade exports Prometheus metrics and includes a pre-built Grafana dashboard for real-time monitoring.</p>' +
            '<h4>Endpoints</h4>' +
            '<table>' +
                '<tr><th>Endpoint</th><th>Description</th></tr>' +
                '<tr><td><code>/metrics</code></td><td>Prometheus metrics (port 9090 in Docker)</td></tr>' +
                '<tr><td><code>/health</code></td><td>Health check (returns 200 when server is ready)</td></tr>' +
            '</table>'
    },
    {
        id: 'monitoring-metrics',
        category: 'monitoring',
        title: 'Prometheus Metrics',
        body:
            '<p>Key metrics exported by FruitSalade:</p>' +
            '<h4>HTTP</h4>' +
            '<table>' +
                '<tr><th>Metric</th><th>Type</th><th>Description</th></tr>' +
                '<tr><td><code>fruitsalade_http_requests_total</code></td><td>Counter</td><td>Total HTTP requests by method, path, status</td></tr>' +
                '<tr><td><code>fruitsalade_http_request_duration_seconds</code></td><td>Histogram</td><td>Request latency (p50/p95/p99)</td></tr>' +
            '</table>' +
            '<h4>Content Transfer</h4>' +
            '<table>' +
                '<tr><th>Metric</th><th>Type</th><th>Description</th></tr>' +
                '<tr><td><code>fruitsalade_content_bytes_downloaded_total</code></td><td>Counter</td><td>Total bytes downloaded</td></tr>' +
                '<tr><td><code>fruitsalade_content_bytes_uploaded_total</code></td><td>Counter</td><td>Total bytes uploaded</td></tr>' +
                '<tr><td><code>fruitsalade_content_downloads_total</code></td><td>Counter</td><td>Download count by status</td></tr>' +
                '<tr><td><code>fruitsalade_content_uploads_total</code></td><td>Counter</td><td>Upload count by status</td></tr>' +
            '</table>' +
            '<h4>Storage &amp; Database</h4>' +
            '<table>' +
                '<tr><th>Metric</th><th>Type</th><th>Description</th></tr>' +
                '<tr><td><code>fruitsalade_metadata_tree_size</code></td><td>Gauge</td><td>Number of entries in the file tree</td></tr>' +
                '<tr><td><code>fruitsalade_db_connections_open</code></td><td>Gauge</td><td>Open PostgreSQL connections</td></tr>' +
                '<tr><td><code>fruitsalade_db_query_duration_seconds</code></td><td>Histogram</td><td>Database query latency</td></tr>' +
                '<tr><td><code>fruitsalade_s3_operation_duration_seconds</code></td><td>Histogram</td><td>S3 operation latency by type</td></tr>' +
                '<tr><td><code>fruitsalade_s3_operations_total</code></td><td>Counter</td><td>S3 operations by type and status</td></tr>' +
            '</table>' +
            '<h4>Auth &amp; Rate Limiting</h4>' +
            '<table>' +
                '<tr><th>Metric</th><th>Type</th><th>Description</th></tr>' +
                '<tr><td><code>fruitsalade_auth_attempts_total</code></td><td>Counter</td><td>Login attempts (success/failure)</td></tr>' +
                '<tr><td><code>fruitsalade_active_tokens</code></td><td>Gauge</td><td>Currently active JWT tokens</td></tr>' +
                '<tr><td><code>fruitsalade_rate_limit_hits_total</code></td><td>Counter</td><td>Rate limit rejections (HTTP 429)</td></tr>' +
                '<tr><td><code>fruitsalade_quota_exceeded_total</code></td><td>Counter</td><td>Quota exceeded events</td></tr>' +
            '</table>' +
            '<h4>Real-time &amp; Sharing</h4>' +
            '<table>' +
                '<tr><th>Metric</th><th>Type</th><th>Description</th></tr>' +
                '<tr><td><code>fruitsalade_sse_connections_active</code></td><td>Gauge</td><td>Active SSE connections</td></tr>' +
                '<tr><td><code>fruitsalade_sse_events_total</code></td><td>Counter</td><td>SSE events sent by type</td></tr>' +
                '<tr><td><code>fruitsalade_share_links_active</code></td><td>Gauge</td><td>Active share links</td></tr>' +
                '<tr><td><code>fruitsalade_share_downloads_total</code></td><td>Counter</td><td>Share link downloads</td></tr>' +
                '<tr><td><code>fruitsalade_permission_checks_total</code></td><td>Counter</td><td>Permission checks (allowed/denied)</td></tr>' +
            '</table>'
    },
    {
        id: 'monitoring-grafana',
        category: 'monitoring',
        title: 'Grafana Dashboard',
        body:
            '<p>A pre-built Grafana dashboard is included at <code>deploy/grafana-dashboard.json</code>.</p>' +
            '<h4>Import Steps</h4>' +
            '<ol>' +
                '<li>Open Grafana &rarr; Dashboards &rarr; Import</li>' +
                '<li>Upload <code>grafana-dashboard.json</code> or paste its contents</li>' +
                '<li>Select your Prometheus data source</li>' +
                '<li>Click Import</li>' +
            '</ol>' +
            '<h4>Dashboard Panels</h4>' +
            '<table>' +
                '<tr><th>Section</th><th>Panels</th></tr>' +
                '<tr><td><strong>Overview</strong></td><td>Request rate by method, error rate (4xx/5xx), latency percentiles (p50/p95/p99), top 10 endpoints by rate and latency</td></tr>' +
                '<tr><td><strong>Content</strong></td><td>Download/upload throughput (Bps), operation rates by status, cumulative totals</td></tr>' +
                '<tr><td><strong>Storage</strong></td><td>Metadata tree size, DB connections, S3 operation latency, S3 error rate, SSE connections, SSE event rate</td></tr>' +
                '<tr><td><strong>Auth</strong></td><td>Login success/failure rate, active tokens, rate limit hits, quota exceeded events</td></tr>' +
                '<tr><td><strong>Sharing</strong></td><td>Active share links, share download rate, permission check rate</td></tr>' +
                '<tr><td><strong>Database</strong></td><td>Query latency percentiles, query rate by type, S3 latency by operation</td></tr>' +
            '</table>' +
            '<div class="wiki-tip">The dashboard auto-refreshes every 30 seconds and defaults to a 1-hour time range.</div>'
    },
    {
        id: 'monitoring-prometheus',
        category: 'monitoring',
        title: 'Prometheus Setup',
        body:
            '<p>Add FruitSalade as a scrape target in your <code>prometheus.yml</code>:</p>' +
            '<pre>scrape_configs:\n  - job_name: fruitsalade\n    scrape_interval: 15s\n    static_configs:\n      - targets: [\'your-server:9090\']</pre>' +
            '<div class="wiki-note">In Docker Compose, the metrics port is mapped to <code>48001</code> on the host. Use <code>localhost:48001</code> if Prometheus runs on the same machine.</div>' +
            '<h4>Alerting Examples</h4>' +
            '<p>Useful PromQL alert rules:</p>' +
            '<pre># High error rate (>5% of requests are 5xx)\nrate(fruitsalade_http_requests_total{status=~"5.."}[5m])\n  / rate(fruitsalade_http_requests_total[5m]) > 0.05\n\n# High request latency (p95 > 2s)\nhistogram_quantile(0.95,\n  rate(fruitsalade_http_request_duration_seconds_bucket[5m])\n) > 2\n\n# S3 errors\nrate(fruitsalade_s3_operations_total{status="error"}[5m]) > 0</pre>'
    },
    {
        id: 'monitoring-systemd',
        category: 'monitoring',
        title: 'Systemd Services',
        body:
            '<p>For bare-metal deployments, systemd service files are provided in <code>deploy/</code>:</p>' +
            '<h4>Server</h4>' +
            '<pre># Install\nsudo cp deploy/fruitsalade-server.service /etc/systemd/system/\nsudo systemctl daemon-reload\nsudo systemctl enable --now fruitsalade-server</pre>' +
            '<h4>FUSE Client (per-user)</h4>' +
            '<pre># Install (template unit)\nsudo cp deploy/fruitsalade-fuse@.service /etc/systemd/system/\nsudo systemctl daemon-reload\n\n# Start for user "alice"\nsudo systemctl enable --now fruitsalade-fuse@alice</pre>' +
            '<div class="wiki-note">The FUSE client uses a template unit (<code>@.service</code>) so each user gets an isolated mount with their own credentials.</div>'
    },

    // ── Troubleshooting ──────────────────────────────────────────────────
    {
        id: 'troubleshoot-login',
        category: 'troubleshooting',
        title: 'Login Issues',
        body:
            '<h4>Wrong credentials</h4>' +
            '<p>The default admin account is <code>admin</code> / <code>admin</code>. If the password was changed and forgotten, an admin must reset it via the Users page or directly in the database.</p>' +
            '<h4>Token expired</h4>' +
            '<p>JWT tokens have a finite lifetime. If you see a 401 error, the app will automatically redirect you to the login page. Simply log in again.</p>' +
            '<h4>2FA locked out</h4>' +
            '<p>If you lost access to your authenticator app:</p>' +
            '<ol>' +
                '<li>Use one of your <span class="wiki-link" data-wiki-link="backup-codes">backup codes</span> to log in</li>' +
                '<li>Disable 2FA from the Dashboard Security section</li>' +
                '<li>Re-enable 2FA with a new device</li>' +
            '</ol>' +
            '<div class="wiki-warning">If you have no backup codes and no authenticator access, ask an admin to disable 2FA for your account directly in the database.</div>'
    },
    {
        id: 'troubleshoot-upload',
        category: 'troubleshooting',
        title: 'Upload Problems',
        body:
            '<h4>File too large</h4>' +
            '<p>The server rejects uploads exceeding <code>MAX_UPLOAD_SIZE</code> (default 100 MB). Ask your admin to increase this value.</p>' +
            '<h4>Quota exceeded</h4>' +
            '<p>If your storage quota is reached, you will see a 413 or 403 error. Free up space by deleting files or emptying the trash, or ask an admin to increase your quota.</p>' +
            '<h4>Upload stalls or fails</h4>' +
            '<ul>' +
                '<li>Check your network connection</li>' +
                '<li>Try a smaller file to rule out timeout issues</li>' +
                '<li>Check the browser console for error details</li>' +
                '<li>Verify the storage backend is healthy (S3 reachable, disk space available)</li>' +
            '</ul>'
    },
    {
        id: 'troubleshoot-fuse',
        category: 'troubleshooting',
        title: 'FUSE Client Issues',
        body:
            '<h4>"Transport endpoint is not connected"</h4>' +
            '<p>The FUSE mount was interrupted. Unmount and remount:</p>' +
            '<pre>sudo umount /mnt/fruitsalade\n./bin/fruitsalade-fuse -server https://your-server:48000 -mount /mnt/fruitsalade</pre>' +
            '<h4>"fusermount: permission denied"</h4>' +
            '<p>Ensure your user is in the <code>fuse</code> group:</p>' +
            '<pre>sudo usermod -aG fuse $USER\n# Log out and back in</pre>' +
            '<h4>"fuse: device not found"</h4>' +
            '<p>The FUSE kernel module is not loaded:</p>' +
            '<pre>sudo modprobe fuse</pre>' +
            '<h4>Files show 0 bytes</h4>' +
            '<p>This is normal. FruitSalade uses on-demand placeholders. Files only download their content when you open them. Metadata (name, size, mtime) is always available via <code>ls -l</code>.</p>' +
            '<h4>Slow directory listings</h4>' +
            '<ul>' +
                '<li>The first listing after mount fetches the metadata tree from the server</li>' +
                '<li>Subsequent listings use the cached tree</li>' +
                '<li>If still slow, check your network latency to the server</li>' +
            '</ul>'
    },
    {
        id: 'troubleshoot-sync',
        category: 'troubleshooting',
        title: 'Sync &amp; Conflict Issues',
        body:
            '<h4>Changes not appearing</h4>' +
            '<ul>' +
                '<li>Ensure SSE (Server-Sent Events) is connected &mdash; check the browser console for SSE connection messages</li>' +
                '<li>Hard refresh the page (<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>R</kbd>)</li>' +
                '<li>FUSE clients periodically poll for metadata updates; wait a few seconds</li>' +
            '</ul>' +
            '<h4>Conflict files appearing</h4>' +
            '<p>Conflicts occur when two clients edit the same file simultaneously. Go to <strong>File Management &rarr; Conflicts</strong> to resolve them.</p>' +
            '<div class="wiki-note">Conflicts are detected via version headers. The server never silently overwrites &mdash; it always preserves both versions.</div>' +
            '<h4>Version mismatch errors</h4>' +
            '<p>If you see a 409 response when saving:</p>' +
            '<ol>' +
                '<li>Someone else modified the file since you opened it</li>' +
                '<li>Reload the file to get the latest version</li>' +
                '<li>Re-apply your changes and save again</li>' +
            '</ol>'
    },
    {
        id: 'troubleshoot-docker',
        category: 'troubleshooting',
        title: 'Docker Issues',
        body:
            '<h4>Server won\'t start</h4>' +
            '<p>Check the logs:</p>' +
            '<pre>make docker-logs</pre>' +
            '<p>Common causes:</p>' +
            '<ul>' +
                '<li><strong>Port conflict</strong> &mdash; Another service is using port 48000. Check with <code>ss -tlnp | grep 48000</code></li>' +
                '<li><strong>PostgreSQL init failure</strong> &mdash; Corrupt data volume. Run <code>make docker-down</code> (removes volumes) and <code>make docker-up</code></li>' +
                '<li><strong>S3/MinIO unreachable</strong> &mdash; Ensure the MinIO container is healthy before the server starts</li>' +
            '</ul>' +
            '<h4>DOCKER_API_VERSION mismatch</h4>' +
            '<p>If you see "client version X is too new" errors:</p>' +
            '<pre>export DOCKER_API_VERSION=1.41</pre>' +
            '<div class="wiki-note">This is common with Docker 20.10 and Compose v5. Set this variable in your shell profile.</div>' +
            '<h4>FUSE clients crash in Docker</h4>' +
            '<p>FUSE containers require:</p>' +
            '<ul>' +
                '<li><code>--cap-add SYS_ADMIN</code></li>' +
                '<li><code>--device /dev/fuse</code></li>' +
                '<li><code>--security-opt apparmor:unconfined</code> (on systems with AppArmor)</li>' +
            '</ul>' +
            '<p>If <code>/dev/fuse</code> doesn\'t exist on the host, the FUSE clients cannot run.</p>'
    },
    {
        id: 'troubleshoot-storage',
        category: 'troubleshooting',
        title: 'Storage Backend Issues',
        body:
            '<h4>S3: "access denied" or "no such bucket"</h4>' +
            '<ul>' +
                '<li>Verify <code>S3_ACCESS_KEY</code>, <code>S3_SECRET_KEY</code>, and <code>S3_BUCKET</code> environment variables</li>' +
                '<li>Ensure the bucket exists: <code>mc ls local/fruitsalade</code></li>' +
                '<li>Check the MinIO console at <code>http://localhost:48003</code></li>' +
            '</ul>' +
            '<h4>Local: "permission denied"</h4>' +
            '<p>The server process must have read/write access to <code>LOCAL_STORAGE_PATH</code> (default <code>/data/storage</code>):</p>' +
            '<pre>sudo chown -R $(whoami) /data/storage</pre>' +
            '<h4>SMB: mount failures</h4>' +
            '<ul>' +
                '<li>Ensure <code>cifs-utils</code> is installed on the server</li>' +
                '<li>Verify network connectivity to the SMB share</li>' +
                '<li>Check credentials and share permissions</li>' +
            '</ul>' +
            '<h4>Disk full</h4>' +
            '<p>If uploads fail with I/O errors:</p>' +
            '<ol>' +
                '<li>Check disk usage: <code>df -h</code></li>' +
                '<li>Empty the trash to reclaim space</li>' +
                '<li>Delete old file versions if not needed</li>' +
                '<li>Consider migrating to an S3 backend for elastic storage</li>' +
            '</ol>'
    },
    {
        id: 'troubleshoot-performance',
        category: 'troubleshooting',
        title: 'Performance Tips',
        body:
            '<h4>Slow page loads</h4>' +
            '<ul>' +
                '<li>Enable the service worker (PWA) for cached static assets</li>' +
                '<li>Check the Prometheus dashboard for high request latencies</li>' +
                '<li>Ensure PostgreSQL has enough memory and connections</li>' +
            '</ul>' +
            '<h4>Gallery slow to load</h4>' +
            '<ul>' +
                '<li>Thumbnails are generated on first access; subsequent loads use cached versions</li>' +
                '<li>Large photo libraries benefit from the S3 backend with CDN caching</li>' +
            '</ul>' +
            '<h4>High database query latency</h4>' +
            '<ul>' +
                '<li>Monitor <code>fruitsalade_db_query_duration_seconds</code> in Grafana</li>' +
                '<li>Large file trees (&gt;100k files) may need PostgreSQL tuning</li>' +
                '<li>Ensure indexes exist on <code>path</code>, <code>parent_path</code>, and <code>id</code> columns</li>' +
            '</ul>' +
            '<h4>S3 latency</h4>' +
            '<ul>' +
                '<li>Use a regional endpoint close to your server</li>' +
                '<li>For MinIO, deploy it on the same network as the server</li>' +
                '<li>Monitor <code>fruitsalade_s3_operation_duration_seconds</code> for spikes</li>' +
            '</ul>' +
            '<div class="wiki-tip">The FUSE client\'s LRU cache significantly reduces repeated downloads. Pin frequently accessed files for instant offline access.</div>'
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
