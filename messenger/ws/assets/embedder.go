//go:build !release

package assets

// This file embeds the static files found here into the application during development.
//
// For deployment, these files should be copied to the deployment directory, compressed
// and embedded from there. See goradd-project/build and goradd-project/deploy.

import (
	"embed"
	"path"

	"github.com/goradd/serve/config"
	"github.com/goradd/serve/http"
)

//go:embed js
var a embed.FS

func init() {
	http.RegisterAssetDirectory(path.Join(config.AssetPath, "messenger"), a)
}
