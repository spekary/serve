package server

import (
	"net/http"

	"github.com/goradd/serve"
	"github.com/goradd/serve/page"
)

type Application struct {
	serve.ServerBase

	// Your own vars, methods and overrides
}

// NewApplication creates the application object and related objects
// You can potentially read command line params and make other versions of the app for testing purposes.
func NewApplication() *Application {
	a := new(Application)
	a.Init()
	return a
}

// Init initializes the application object.
func (a *Application) Init() {
	a.ServerBase.Init()

	// The initialization steps below setup various services used by the framework.
	// Customize as needed and as your app expands.
	// By default, it is set up to handle everything on a single server in memory.
	a.SetupSessionManager()
	a.SetupMessenger()
	page.SetPagestateCache(page.NewSerializedPageCache(100, 60*60*24))
}

// RunServer launches the main server.
func (a *Application) RunServer() (err error) {
	handler := a.MakeHandler()

	err = serve.ListenAndServeWithTimeouts("", handler)
	return
}

func (a *Application) MakeHandler() http.Handler {
	return a.ServerBase.MakeHandler()
}
