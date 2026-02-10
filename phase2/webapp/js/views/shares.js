function renderShares() {
    var app = document.getElementById('app');
    app.innerHTML =
        '<div class="toolbar">' +
            '<h2>Share Links</h2>' +
        '</div>' +
        '<p style="color:var(--text-muted);margin-bottom:1rem">' +
            'To create a share link, navigate to a file in the browser and click Share.' +
        '</p>' +
        '<a href="#browser" class="btn btn-outline">Go to Files</a>';
}
