// Package ui provides embedded static files for the admin dashboard.
package ui

import "embed"

//go:embed index.html css js
var Assets embed.FS
