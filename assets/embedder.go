//go:build !release

// Package assets contains the css and javascript required to run a goradd server.
package assets

// This file embeds the static files found here into the application during development.
//
// For deployment, these files should be copied to the deployment directory, compressed
// and embedded from there. See goradd-project/build and goradd-project/deploy.

import (
	"embed"
	"github.com/goradd/serve/config"
	"github.com/goradd/serve/http"
	"path"
)

//go:embed css js
var a embed.FS

func init() {
	// The path below is the same path the assets should be copied to for deployment.
	http.RegisterAssetDirectory(path.Join(config.AssetPrefix, "goradd"), a)
}
