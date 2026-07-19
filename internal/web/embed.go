// Package web embeds the built single-page app served by the daemon's HTTP plane.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist is the build-output-only SPA rooted at the dist directory. The tracked
// .gitkeep lets the embed compile before the web toolchain has produced a build.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
