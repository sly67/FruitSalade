// Shared utilities — loaded after api.js, before views

// ─── Toast Notifications ────────────────────────────────────────────────────

var Toast = (function() {
    function getContainer() {
        var c = document.getElementById('toast-container');
        if (!c) {
            c = document.createElement('div');
            c.id = 'toast-container';
            c.setAttribute('role', 'status');
            c.setAttribute('aria-live', 'polite');
            document.body.appendChild(c);
        }
        return c;
    }

    function show(message, type, duration) {
        duration = duration || 4000;
        var container = getContainer();

        var toast = document.createElement('div');
        toast.className = 'toast toast-' + (type || 'info');
        toast.innerHTML = '<span class="toast-msg">' + esc(message) + '</span>' +
            '<button class="toast-close" aria-label="Dismiss">&times;</button>';

        container.appendChild(toast);

        // Trigger animation
        requestAnimationFrame(function() { toast.classList.add('toast-visible'); });

        var timer = setTimeout(function() { dismiss(toast); }, duration);

        toast.querySelector('.toast-close').addEventListener('click', function() {
            clearTimeout(timer);
            dismiss(toast);
        });
    }

    function dismiss(toast) {
        toast.classList.remove('toast-visible');
        toast.classList.add('toast-out');
        setTimeout(function() { if (toast.parentNode) toast.parentNode.removeChild(toast); }, 300);
    }

    var progressCounter = 0;

    // Persistent progress toast (no auto-dismiss)
    function showProgress(message, pct) {
        var id = 'toast-progress-' + (++progressCounter);
        var container = getContainer();
        var toast = document.createElement('div');
        toast.className = 'toast toast-info';
        toast.id = id;
        toast.innerHTML = '<div class="toast-progress-wrap">' +
            '<span class="toast-msg">' + esc(message) + '</span>' +
            '<div class="toast-progress-bar-wrap"><div class="toast-progress-bar" style="width:' + (pct || 0) + '%"></div></div>' +
            '</div>' +
            '<button class="toast-close" aria-label="Dismiss">&times;</button>';
        container.appendChild(toast);
        requestAnimationFrame(function() { toast.classList.add('toast-visible'); });
        toast.querySelector('.toast-close').addEventListener('click', function() {
            dismiss(toast);
        });
        return id;
    }

    function updateProgress(id, message, pct) {
        var toast = document.getElementById(id);
        if (!toast) return;
        var msg = toast.querySelector('.toast-msg');
        if (msg) msg.textContent = message;
        var bar = toast.querySelector('.toast-progress-bar');
        if (bar) bar.style.width = (pct || 0) + '%';
    }

    function dismissProgress(id) {
        var toast = document.getElementById(id);
        if (toast) dismiss(toast);
    }

    return {
        show: show,
        success: function(msg, dur) { show(msg, 'success', dur); },
        error: function(msg, dur) { show(msg, 'error', dur); },
        info: function(msg, dur) { show(msg, 'info', dur); },
        progress: showProgress,
        updateProgress: updateProgress,
        dismissProgress: dismissProgress
    };
})();

// ─── Modal Helper ───────────────────────────────────────────────────────────

var Modal = (function() {
    var currentOverlay = null;
    var previousFocus = null;
    var titleIdCounter = 0;

    function open(opts) {
        close(); // close any existing modal

        previousFocus = document.activeElement;

        var overlay = document.createElement('div');
        overlay.className = 'modal-overlay';
        overlay.id = 'modal-overlay';

        var modal = document.createElement('div');
        modal.className = 'modal' + (opts.className ? ' ' + opts.className : '');
        modal.setAttribute('role', 'dialog');
        modal.setAttribute('aria-modal', 'true');
        modal.setAttribute('tabindex', '-1');

        var closeBtn = document.createElement('button');
        closeBtn.className = 'modal-close';
        closeBtn.innerHTML = '&times;';
        closeBtn.setAttribute('aria-label', 'Close dialog');
        closeBtn.addEventListener('click', close);

        modal.appendChild(closeBtn);

        if (opts.title) {
            var titleId = 'modal-title-' + (++titleIdCounter);
            var h3 = document.createElement('h3');
            h3.id = titleId;
            h3.textContent = opts.title;
            modal.appendChild(h3);
            modal.setAttribute('aria-labelledby', titleId);
        }

        if (typeof opts.content === 'string') {
            var div = document.createElement('div');
            div.innerHTML = opts.content;
            modal.appendChild(div);
        } else if (opts.content) {
            modal.appendChild(opts.content);
        }

        overlay.appendChild(modal);
        overlay.addEventListener('click', function(e) {
            if (e.target === overlay) close();
        });

        document.body.appendChild(overlay);
        currentOverlay = overlay;

        // Focus the modal
        modal.focus();

        // Escape key + focus trap
        document.addEventListener('keydown', onKeyDown);

        return modal;
    }

    function close() {
        if (currentOverlay) {
            if (currentOverlay.parentNode) currentOverlay.parentNode.removeChild(currentOverlay);
            currentOverlay = null;
        }
        document.removeEventListener('keydown', onKeyDown);
        // Restore focus
        if (previousFocus && previousFocus.focus) {
            try { previousFocus.focus(); } catch(_) {}
            previousFocus = null;
        }
    }

    function onKeyDown(e) {
        if (e.key === 'Escape') { close(); return; }
        if (e.key === 'Tab' && currentOverlay) {
            trapFocus(e);
        }
    }

    function trapFocus(e) {
        var modal = currentOverlay ? currentOverlay.querySelector('.modal') : null;
        if (!modal) return;
        var focusable = modal.querySelectorAll(
            'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
        );
        if (focusable.length === 0) return;
        var first = focusable[0];
        var last = focusable[focusable.length - 1];
        if (e.shiftKey) {
            if (document.activeElement === first || document.activeElement === modal) {
                e.preventDefault();
                last.focus();
            }
        } else {
            if (document.activeElement === last) {
                e.preventDefault();
                first.focus();
            }
        }
    }

    function getModal() {
        return currentOverlay ? currentOverlay.querySelector('.modal') : null;
    }

    return { open: open, close: close, getModal: getModal };
})();

// ─── File Type Detection ────────────────────────────────────────────────────

var FileTypes = (function() {
    var textExts = ['.txt', '.md', '.json', '.js', '.css', '.html', '.py', '.go', '.sh',
        '.yml', '.yaml', '.xml', '.csv', '.log', '.ini', '.conf', '.toml', '.env',
        '.sql', '.rs', '.ts', '.tsx', '.jsx', '.c', '.cpp', '.h', '.java', '.rb',
        '.php', '.lua', '.makefile', '.dockerfile', '.gitignore', '.mod', '.sum',
        '.cfg', '.properties', '.bat', '.ps1', '.r', '.swift', '.kt', '.scala'];
    var imageExts = ['.png', '.jpg', '.jpeg', '.gif', '.svg', '.webp', '.bmp', '.ico'];
    var codeExts = ['.js', '.ts', '.tsx', '.jsx', '.py', '.go', '.rs', '.c', '.cpp', '.h',
        '.java', '.rb', '.php', '.lua', '.sh', '.bat', '.ps1', '.r', '.swift', '.kt',
        '.scala', '.sql', '.css', '.html', '.xml'];
    var archiveExts = ['.zip', '.tar', '.gz', '.bz2', '.xz', '.7z', '.rar', '.tar.gz', '.tgz'];
    var videoExts = ['.mp4', '.mkv', '.avi', '.mov', '.webm', '.flv', '.wmv'];
    var audioExts = ['.mp3', '.wav', '.ogg', '.flac', '.aac', '.m4a', '.wma'];
    var spreadsheetExts = ['.csv', '.xls', '.xlsx', '.ods'];

    function getExt(filename) {
        if (!filename) return '';
        var dot = filename.lastIndexOf('.');
        return dot !== -1 ? filename.substring(dot).toLowerCase() : '';
    }

    function detect(extOrFilename) {
        var ext = extOrFilename.indexOf('.') === 0 ? extOrFilename : getExt(extOrFilename);
        if (textExts.indexOf(ext) !== -1 || ext === '') return 'text';
        if (imageExts.indexOf(ext) !== -1) return 'image';
        if (ext === '.pdf') return 'pdf';
        return 'other';
    }

    function isText(filename) {
        var ext = getExt(filename);
        return textExts.indexOf(ext) !== -1 || ext === '';
    }

    function icon(name, isDir) {
        if (isDir) return '<span class="file-icon file-icon-folder">&#128193;</span>';

        var ext = getExt(name);

        if (codeExts.indexOf(ext) !== -1) {
            return '<span class="file-icon file-icon-code">&#128221;</span>';
        }
        if (imageExts.indexOf(ext) !== -1) {
            return '<span class="file-icon file-icon-image">&#127912;</span>';
        }
        if (ext === '.pdf' || ext === '.doc' || ext === '.docx' || ext === '.odt') {
            return '<span class="file-icon file-icon-doc">&#128196;</span>';
        }
        if (archiveExts.indexOf(ext) !== -1) {
            return '<span class="file-icon file-icon-archive">&#128230;</span>';
        }
        if (videoExts.indexOf(ext) !== -1) {
            return '<span class="file-icon file-icon-video">&#127916;</span>';
        }
        if (audioExts.indexOf(ext) !== -1) {
            return '<span class="file-icon file-icon-audio">&#127925;</span>';
        }
        if (spreadsheetExts.indexOf(ext) !== -1) {
            return '<span class="file-icon file-icon-spreadsheet">&#128202;</span>';
        }
        if (ext === '.md' || ext === '.txt' || ext === '.log') {
            return '<span class="file-icon file-icon-text">&#128196;</span>';
        }
        if (ext === '.yml' || ext === '.yaml' || ext === '.toml' || ext === '.ini' || ext === '.conf' || ext === '.env') {
            return '<span class="file-icon file-icon-config">&#9881;</span>';
        }

        return '<span class="file-icon file-icon-default">&#128196;</span>';
    }

    var galleryImageExts = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp', '.tiff', '.tif',
        '.heic', '.heif', '.avif',
        '.cr2', '.cr3', '.nef', '.arw', '.dng', '.orf', '.rw2', '.pef', '.srw', '.raf'];

    function isGalleryImage(filename) {
        var ext = getExt(filename);
        return galleryImageExts.indexOf(ext) !== -1;
    }

    return { detect: detect, isText: isText, icon: icon, getExt: getExt, isGalleryImage: isGalleryImage };
})();
