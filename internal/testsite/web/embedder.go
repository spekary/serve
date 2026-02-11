package web

// This file embeds the static files into the application as static files during
// development.
//
// For deployment, these files would be copied to the deployment directory, compressed
// and embedded from there.
//
// The root directory represents the root of the website, or the "/" directory.
// You can modify this by using the ProxyPath setting if your application is served
// from a logical subdirectory of a bigger website.
//
// Simply layout all of your static files here and they will be served as if they were part
// of a file system. If the user navigates to a file that does not exist in the file system here,
// the Goradd muxer will be invoked and the rest of your application will serve the resource requested.

import (
	"embed"
	"io/fs"

	"github.com/goradd/serve/http"
)

//go:embed root/*
var root embed.FS

// go:embed assets/*
var a embed.FS

func init() {
	sub, _ := fs.Sub(root, "root")
	serv := http.FileSystemServer{Fsys: sub, SendModTime: true}
	http.RegisterAppHandler("/", serv)
}
