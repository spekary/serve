package page

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	http2 "github.com/goradd/serve/http"
	"github.com/goradd/serve/page/action"
	"github.com/goradd/serve/page/event"
	"github.com/goradd/serve/session"
	strings2 "github.com/goradd/strings"

	"github.com/goradd/serve/log"
)

type httpRequestContextKey struct{}

// RequestMode tracks what kind of request we are processing.
type RequestMode int

const (
	// RequestModeServer indicates we are calling back to a previously sent form using a standard form post
	RequestModeServer RequestMode = iota
	// RequestModeHttp indicates this is a first-time request for a page
	RequestModeHttp
	// RequestModeAjax indicates we are calling back in to a currently showing form using an ajax request
	RequestModeAjax
	// RequestModeCustomAjax indicates we are calling an entry point from ajax, but not through our javascript process.
	// This could be custom control attempting a direct ajax push of data.
	// For future expansion if needed
	//RequestModeCustomAjax
)

const (
	HtmlVarAction    = HiddenInputPrefix + "Action"
	HtmlVarPagestate = HiddenInputPrefix + "PageState"
	HtmlVarParams    = HiddenInputPrefix + "params"
)

// MultipartFormMax is the maximum size of a mult-part form that will be
// stored in memory in bytes. This data is stored in the "Value" part of the multi-part form data.
// Anything outside of this will be stored in temporary on-disk files. Note that the
// built-in http handler will reserve an additional 10MB for in-memory storage of non-content
// data as well.
//
// The default is to store all multi-part data in temporary files.
var MultipartFormMax int64 = 0

// String satisfies the Stringer interface and returns a description of the RequestMode
func (m RequestMode) String() string {
	switch m {
	case RequestModeServer:
		return "Server"
	case RequestModeHttp:
		return "Http"
	case RequestModeAjax:
		return "Do"
	}
	return "Unknown"
}

// RequestContext contains information taken from the http.Request object.
type RequestContext struct {
	// URL is the url being queried
	URL *url.URL
	// formVars is a private version of the form variables. Use the FormValue and FormValues functions to get to these values.
	formVars url.Values
	// postFormVars is similar to formVars, but just for post type variables.
	postFormVars url.Values
	// Host is the host value extracted from the request
	Host string
	// RemoteAddr is the ip address of the client
	RemoteAddr string
	// Referrer is the referring url, if there is one and it is included in the request. In other words, if a link was
	// clicked to get here, it would be the URL of the page that had the link
	Referrer string
	// Cookies are the cookies coming from the client, mapped by name
	Cookies map[string]*http.Cookie
	// Header is the http header coming from the client.
	Header http.Header
	// MultipartForm is the multipart form information if the request had a multi-part form in it.
	MutliPartForm *multipart.Form

	requestMode          RequestMode
	pageStateId          string
	ActionControlID      string        // If an action, the control sending the action
	EventID              event.EventID // The event to send to the control
	ActionValues         action.RawActionValues
	RefreshIds           []string
	hasTimezoneInfo      bool
	clientTimezoneOffset int
	clientTimezone       string

	// NoJavaScript indicates javascript is turned off by the browser
	NoJavaScript bool
}

// ParseRequest processes the request, extracting information from the request and placing it
// into the returned context.
// Per comments in the ResponseWriter, we need to read and process the entire request before attempting to write.
// Generally, this shouldn't be a problem since we are buffering output.
// By placing this information into a context, we avoid re-parsing and processing request data later.
func ParseRequest(r *http.Request) (ctx context.Context, err error) {
	err = parseForm(r)
	if err != nil {
		return
	}

	ctx = r.Context()
	requestData := &RequestContext{
		URL:           r.URL,
		formVars:      r.Form,
		postFormVars:  r.PostForm,
		Host:          r.Host,
		RemoteAddr:    r.RemoteAddr,
		Referrer:      r.Referer(),
		Header:        r.Header,
		MutliPartForm: r.MultipartForm,
	}
	requestData.Cookies = make(map[string]*http.Cookie)
	for _, c := range r.Cookies() {
		requestData.Cookies[c.Name] = c
	}

	err = fillAppData(ctx, requestData)
	if err != nil {
		return
	}

	ctx = context.WithValue(ctx, httpRequestContextKey{}, requestData)

	return
}

func parseForm(r *http.Request) (err error) {
	if contentType := r.Header.Get("content-type"); contentType != "" {
		if strings.Contains(contentType, "multipart") {
			err = r.ParseMultipartForm(MultipartFormMax)
			if err != nil {
				return fmt.Errorf("error parsing multipart form: %v, %w", r, err)
			}

			for key, group := range r.MultipartForm.File {
				for _, fh := range group {
					log.Debug(r.Context(), "page", "Multipart Form File", "key", key, "filename", fh.Filename)
				}
			}
		} else {
			err = r.ParseForm()
		}
	} else {
		err = r.ParseForm()
	}

	if err != nil {
		return fmt.Errorf("error parsing form: %v, %w", r, err)
	}
	return
}

func GetRequest(ctx context.Context) *RequestContext {
	return ctx.Value(httpRequestContextKey{}).(*RequestContext)
}

// FormValue returns the given form variable value, either from post or get variables.
// If the value does not exist, or has multiple values, returns false in ok.
// Use FormValues for multipart values.
func (ctx *RequestContext) FormValue(key string) (value string, ok bool) {
	if ctx.formVars == nil {
		return
	}
	var v []string
	if v, ok = ctx.formVars[key]; ok && len(v) == 1 {
		value = v[0]
	}
	return
}

// FormValues returns the corresponding form value as a string slice. Use this when you are expecting more than
// one value in the given form variable
func (ctx *RequestContext) FormValues(key string) (value []string, ok bool) {
	if ctx.formVars == nil {
		return
	}
	value, ok = ctx.formVars[key]
	return
}

// PostFormValue returns the given POST form variable value.
// If the value does not exist, or has multiple values, returns false in ok.
// Use PostFormValues for multiple values.
func (ctx *RequestContext) PostFormValue(key string) (value string, ok bool) {
	if ctx.postFormVars == nil {
		return
	}
	var v []string
	if v, ok = ctx.postFormVars[key]; ok && len(v) == 1 {
		value = v[0]
	}
	return
}

// PostFormValues returns the corresponding POST form value as a string slice.
// Use this when you are expecting more than
// one value in the given form variable.
func (ctx *RequestContext) PostFormValues(key string) (value []string, ok bool) {
	if ctx.postFormVars == nil {
		return
	}
	value, ok = ctx.postFormVars[key]
	return
}

// fillAppData fills the app structure with app specific information from the request
func fillAppData(ctx context.Context, r *RequestContext) error {
	var ok bool
	if r.pageStateId, ok = r.FormValue(HtmlVarPagestate); ok {
		return processPageState(ctx, r)
	} else if h := r.Header.Get("X-Requested-With"); strings.ToLower(h) == "xmlhttprequest" {
		// A custom ajax call
		//r.requestMode = RequestModeCustomAjax
	} else {
		processNewRequest(ctx, r)
	}
	return nil
}

func processPageState(ctx context.Context, r *RequestContext) error {
	var v string
	v, _ = r.FormValue(HtmlVarParams)
	if v == "" {
		// javascript is turned off
		// we are in a minimalist environment, where only buttons submit forms
		r.NoJavaScript = true
		r.requestMode = RequestModeServer
		aId, _ := r.FormValue(HtmlVarAction)
		parts := strings.Split(aId, "_")
		r.ActionControlID = parts[0]
		if len(parts) > 1 {
			r.ActionValues.Control = []byte(parts[1])
		}
		return nil
	}

	if h := r.Header.Get("X-Requested-With"); strings.ToLower(h) == "xmlhttprequest" {
		r.requestMode = RequestModeAjax
	} else {
		r.requestMode = RequestModeServer
	}
	return processPageParams(ctx, r, v)
}

func processTimezoneInfo(ctx context.Context,
	r *RequestContext,
	timezone string,
	timezoneOffset int) {

	if timezone != "" {
		if timezoneOffset > 24*60 || timezoneOffset < -24*60 {
			slog.Warn("TimezoneOffset is out of range", "TimezoneOffset", timezoneOffset)
			return
		}
		r.clientTimezoneOffset = timezoneOffset

		if !strings2.IsASCII(timezone) {
			slog.Warn("invalid timezone name", "timezone", timezone)
		}

		r.clientTimezone = timezone
		r.hasTimezoneInfo = true
	} else if session.Has(ctx, session.TimezoneOffset) {
		// recover previously set timezone from this session
		r.hasTimezoneInfo = true
		r.clientTimezoneOffset = session.GetInt(ctx, session.TimezoneOffset)
		r.clientTimezone = session.GetString(ctx, session.Timezone)
	}

}

func processPageParams(ctx context.Context, r *RequestContext, v string) error {
	var params struct {
		ControlID      string                 `json:"controlID"`
		EventID        int                    `json:"eventID"`
		Values         action.RawActionValues `json:"actionValues"`
		RefreshIDs     []string               `json:"refresh"`
		Timezone       string                 `json:"tz"`
		TimezoneOffset int                    `json:"tzo"`
	}

	dec := json.NewDecoder(strings.NewReader(v))

	dec.UseNumber()
	dec.DisallowUnknownFields()

	if err := dec.Decode(&params); err != nil {
		return fmt.Errorf("error decoding goradd posted values: %w", err)
	}
	// Treat all parameters as untrusted, and provide some minimal validation checking
	if !strings2.IsASCII(params.ControlID) {
		return fmt.Errorf("invalid control id")
	}
	r.ActionControlID = params.ControlID

	for _, r := range params.RefreshIDs {
		if !strings2.IsASCII(r) {
			return fmt.Errorf("invalid control id")
		}
	}
	r.RefreshIds = params.RefreshIDs

	if params.EventID != 0 {
		// event ids are mapped, so no danger of out of bounds errors
		r.EventID = event.EventID(params.EventID)
	}

	// Values are validated when read
	r.ActionValues = params.Values
	processTimezoneInfo(ctx, r, params.Timezone, params.TimezoneOffset)

	// Save in a session for recovery when we have a session but do not have client info
	session.SetInt(ctx, session.TimezoneOffset, params.TimezoneOffset)
	session.SetString(ctx, session.Timezone, params.Timezone)

	var ok bool
	if r.pageStateId, ok = r.FormValue(HtmlVarPagestate); !ok {
		return fmt.Errorf("no pagestate found in response")
	} else if !strings2.IsASCII(r.pageStateId) {
		return fmt.Errorf("invalid page state")
	}
	return nil
}

func processNewRequest(ctx context.Context, r *RequestContext) {
	// A new call to our web page
	r.requestMode = RequestModeHttp

	// Recover client timezone if it was saved earlier
	if session.Has(ctx, session.TimezoneOffset) {
		r.hasTimezoneInfo = true
		r.clientTimezoneOffset = session.GetInt(ctx, session.TimezoneOffset)
		r.clientTimezone = session.GetString(ctx, session.Timezone)
	}
}

// RequestMode returns the request mode of the current request.
func (ctx *RequestContext) RequestMode() RequestMode {
	return ctx.requestMode
}

// ClientTimezoneOffset returns the number of minutes offset from GMT for the client's timezone.
func (ctx *RequestContext) ClientTimezoneOffset() int {
	return ctx.clientTimezoneOffset
}

// ClientTimezone returns the name of the timezone of the client, if available.
func (ctx *RequestContext) ClientTimezone() string {
	return ctx.clientTimezone
}

// HasTimezoneInfo returns true if timezone info is valid.
func (ctx *RequestContext) HasTimezoneInfo() bool {
	return ctx.hasTimezoneInfo
}

// ConvertToBool is a helper function that can convert Put or Get values and other possible kinds of values into
// a bool value.
func ConvertToBool(v interface{}) bool {
	var val bool
	switch s := v.(type) {
	case string:
		sLower := strings.ToLower(s)
		if sLower == "true" || sLower == "on" || sLower == "1" {
			val = true
		} else if sLower == "false" || sLower == "off" || sLower == "" || sLower == "0" {
			val = false
		} else {
			panic(fmt.Errorf("unknown checkbox string value: %s", s))
		}
	case int:
		if s == 0 {
			val = false
		} else {
			val = true
		}
	case bool:
		val = s
	default:
		panic(fmt.Errorf("unknown checkbox value: %v", v))
	}

	return val
}

// OutputLen returns the number of bytes that have been written to the output.
func OutputLen(ctx context.Context) int {
	return http2.OutputLen(ctx)
}

func ResetOutputBuffer(ctx context.Context) []byte {
	return http2.ResetOutputBuffer(ctx)
}

// NewMockContext creates a context for testing.
func NewMockContext() (ctx context.Context) {
	sm := session.NewMock()
	ctx = sm.With(context.Background())

	r := httptest.NewRequestWithContext(ctx, "", "/", nil)
	ctx, _ = ParseRequest(r)
	return ctx
}
