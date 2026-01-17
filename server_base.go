package serve

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/goradd/serve/config"
	http2 "github.com/goradd/serve/http"
	"github.com/goradd/serve/i18n"
	"github.com/goradd/serve/messenger"
	"github.com/goradd/serve/messenger/ws"
	"github.com/goradd/serve/page"
	"github.com/goradd/serve/session"
)

// ServerBaseI defines the virtual functions that are callable on the Server.
type ServerBaseI interface {
	Init()

	/*
		ServeHTTP(w http.ResponseWriter, r *http.Request)
		PutContext(*http.Request) *http.Request
		SetupErrorHandling()
		SetupPagestateCaching()
		SetupSessionManager()
		SetupMessenger()
		SetupDatabaseWatcher()
		SetupPaths()
		SessionHandler(next http.Handler) http.Handler
		AccessLogHandler(next http.Handler) http.Handler
		PutDbContextHandler(next http.Handler) http.Handler
		ServeRequestHandler() http.Handler

	*/
}

type ServerBase struct {
	// HstsMaxAge sets the HSTS timeout length in seconds.
	// Set this to -1 to turn off HSTS, or 0 to reset it.
	HstsMaxAge            int64
	HstsIncludeSubdomains bool
	HstsPreload           bool

	SessionHandler session.ManagerI
}

func (a *ServerBase) Init() {
	a.HstsMaxAge = 86400 // one day
	a.HstsIncludeSubdomains = true
	a.HstsPreload = false
}

func (a *ServerBase) MakeHandler() http.Handler {
	// the handler chain gets built in the reverse order of getting called

	// These handlers are called in reverse order
	h := http.NotFoundHandler() // Should go at the end of the chain to catch whatever is missed
	h = http2.WithAppMuxer(h)   // Serves other dynamic files, and possibly the api
	h = a.WithPageHandler(h)    // Serves the Goradd dynamic pages
	h = a.WithSession(h)
	h = http2.WithBufferedOutput(h) // Must be in front of the session handler
	//	h = a.StatsHandler(h)
	h = http2.WithPatternMuxer(h) // Serves most static files and websocket requests.
	// Must be after the error handler so panics are intercepted by the error reporter
	// and must be in front of the buffered output handler because of the websocket server
	h = i18n.LanguageHandler(h) // observe the accept-language header value
	h = http2.WithHeaderValidator(h)
	h = http2.WithErrorHandler(h) // Default http error handler to intercept panics.
	h = a.WithHsts(h)
	// h = a.this().PressureRejector(h)	// Responds with 503 if memory is low
	//	h = a.this().AccessLogHandler(h)

	return h
}

// WithHsts adds an HSTS middleware to the handler stack.
//
// HSTS will force a browser to accept only
// HTTPS connections for everything coming from your domain, if the initial page was served over HTTPS. Many browsers
// already do this. What this additionally does is prevent the user from overriding this. Also, if your
// certificate is bad or expired, it will NOT allow the user the option of using your website anyway.
// This should be safe to send in development mode if your local server is not using HTTPS, since the header
// is ignored if a page is served over HTTP.
//
// Once the HSTS policy has been sent to the browser, it will remember it for the amount of time
// specified, even if the header is not sent again. However, you can override it by sending another header, and
// clear it by setting the timeout to 0. Set the timeout to -1 to turn it off.
func (a *ServerBase) WithHsts(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if a.HstsMaxAge >= 0 {
			http2.WriteHstsHeader(w, a.HstsMaxAge, a.HstsIncludeSubdomains, a.HstsPreload)
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

// WithSession puts the session handling middleware into the handler stack
func (a *ServerBase) WithSession(next http.Handler) http.Handler {
	return a.SessionHandler.Use(next)
}

// SetupSessionManager sets up the global session manager. The session can be used to save data that is specific to a user
// and specific to the user's time on a browser. Sessions are often used to save login credentials so that you know
// the current user is logged in.
//
// The default uses a 3rd party session manager, stores the session in memory, and tracks sessions using cookies.
// This setup is useful for development, testing, debugging, and for moderately used websites.
// However, this default does not scale, so if you are launching multiple copies of the app in production,
// you should override this with a scalable storage mechanism.
func (a *ServerBase) SetupSessionManager() {
	s := scs.New()
	store := memstore.NewWithCleanupInterval(24 * time.Hour) // replace this with a different store if desired
	s.Store = store
	if config.ProxyPath != "" {
		s.Cookie.Path = config.ProxyPath
	}
	s.IdleTimeout = 6 * time.Hour
	sm := session.NewScsManager(s)
	a.SessionHandler = sm
}

// SetupMessenger injects the global messenger that permits pub/sub communication between the server and client.
func (a *ServerBase) SetupMessenger() {
	// The default sets up a websocket based messenger appropriate for development and single-server applications
	if config.WebsocketMessengerPath != "" {
		m := new(ws.WsMessenger)
		messenger.Messenger = m
		m.Start()

		http2.RegisterStaticHandler(config.WebsocketMessengerPath, messenger.Messenger.(*ws.WsMessenger).WebSocketHandler())
	}

}

// WithPageHandler processes requests for data driven pages.
func (a *ServerBase) WithPageHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if page.HasRoute(r.URL.Path) {
			page.ServeHTTP(w, r)
		} else {
			next.ServeHTTP(w, r)
		}
	}
	return http.HandlerFunc(fn)
}
