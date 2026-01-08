package page

import (
	"context"
	"io"
	"net/http"

	"github.com/goradd/serve/log"
)

type FormCreateFunc func() FormI

var routes = make(map[string]FormCreateFunc) // maps paths to form info

func RegisterForm(path string, createFunc FormCreateFunc) {
	if path == "" {
		panic(`you cannot register the empty path. If you want a default, register just a slash. ("/")`)
	}
	if _, ok := routes[path]; ok {
		panic("a form for this path is already registered: " + path)
	}
	form := createFunc()

	if !controlIsRegistered(form) {
		RegisterControl(func() ControlI { return createFunc() }) // a form is a control, and needs to be registered for the serializer
	}

	routes[path] = createFunc
}

func HasRoute(path string) bool {
	if path == "" {
		path = "/"
	}

	_, ok := routes[path]
	return ok
}

func creationFunction(path string) FormCreateFunc {
	if path == "" {
		path = "/"
	}

	f, _ := routes[path]
	return f
}

func ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx, err := parseRequest(req)
	if err != nil {
		panic(err)
	}
	page := getPage(ctx)

	if page == nil {
		// An ajax call, but we could not deserialize the old page. Refresh the entire page to get server access.
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"loc":"reload"}`) // the refresh will be handled in javascript
		return
	}

	err = page.runPage(ctx, w)
	if err != nil {
		// TODO: remove this. All errors should panic in place so we can know where the problem is
		panic(err)
	}
}

// getPage returns a page from the controlCache, whether it is previously allocated, or
// is a new page. If there is an error creating a new page, it should panic. If this is
// an ajax call and the previous page could not be found in the controlCache, then return nil in page.
func getPage(ctx context.Context) (page *Page) {
	var pageStateId string

	request := GetRequest(ctx)
	pageStateId = request.pageStateId
	if pageStateId != "" {
		page = pageCache.Get(pageStateId)
	}
	if page != nil {
		return // found a page in the pagestate controlCache
	}

	if request.requestMode == RequestModeAjax {
		// TODO: If this happens, we need to reload the whole page, because we lost the pagestate completely
		// generally this should only happen if the page state drops out of the controlCache,
		// which might happen if a page sits open for a long time, and then the user initiates an ajax action
		log.Debug(ctx, "page", "Ajax lost the page state")
		return
	}

	// This is a new request with no previous page history
	page = new(Page)
	pageStateId = pageCache.NewPageID()
	page.stateId = pageStateId
	return
}
