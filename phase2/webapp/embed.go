// Package webapp provides embedded static files for the file browser web app.
package webapp

import "embed"

//go:embed index.html css js
var Assets embed.FS
