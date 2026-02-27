package web

import "embed"

// Dist embeds the compiled frontend assets from the web/dist directory.
// Build the frontend first: cd web && npm run build
//
//go:embed all:dist
var Dist embed.FS
