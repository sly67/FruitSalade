// Package webapp provides embedded static files for the file browser web app.
package webapp

import "embed"

//go:embed index.html css js manifest.json icons service-worker.js
var Assets embed.FS
