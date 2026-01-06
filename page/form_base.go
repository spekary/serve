package page

import (
	"context"
	"errors"
	"io"
	http2 "net/http"

	"github.com/goradd/goradd/pkg/http"
	"github.com/goradd/html5tag"
	"github.com/goradd/maps"
	"github.com/goradd/serve/config"
	"github.com/goradd/serve/log"
)

const (
	HtmlCsrfToken = "Goradd__Csrf"
)

// CsrfError sentinel
var CsrfError = errors.New("csrf error")

// FormI described the virtual functions that have default functionality which can
// be overridden.
type FormI interface {
	ControlI
	GetControl(id string) (c ControlI)
	SetGeneratedIdPrefix(prefix string)
	GenerateId() string

	SetupNewForm(ctx context.Context, p *Page)
	Run(ctx context.Context) error
	Preload(ctx context.Context)
	AddHeadTags()
	CreateControls(ctx context.Context)
	LoadControls(ctx context.Context)
	Deserialized()
	Exit(ctx context.Context, w http2.ResponseWriter)
	Response() *Response
	PageDrawingFunction() PageDrawFunc
	DrawHeaderTags(ctx context.Context, w io.Writer)

	renderAjax(ctx context.Context, w io.Writer)
	addControl(c ControlI)
}

type headerItem = maps.SliceMap[string, html5tag.Attributes]

// FormBase is a base form control on which to build individual form controls that control a particular
// web page.
type FormBase struct {
	ControlBase
	page *Page

	drawing             bool
	response            Response
	headerStyleSheets   *headerItem
	importedStyleSheets *headerItem // when refreshing, these get moved to the headerStyleSheets
	headerJavaScripts   *headerItem
	bodyJavaScripts     *headerItem
	importedJavaScripts *headerItem // when refreshing, these get moved to the bodyJavaScripts
	csrf                string      // csrf attack check string
}

func (f *FormBase) Init(self FormI, id string) {
	if id == "" {
		panic("id is required")
	}
	f.ControlBase.Init("form", self, self, nil, id)
}

// SetupNewForm is called by the framework's page router whenever a new URL is loaded.
// You should not need to call this function normally.
func (f *FormBase) SetupNewForm(ctx context.Context, p *Page) {
	f.page = p
	f.this().Preload(ctx)
	f.this().AddHeadTags()
	f.this().CreateControls(ctx)
	f.this().LoadControls(ctx)
}
func (f *FormBase) GetControl(id string) ControlI {
	return f.page.getControl(id)
}

func (f *FormBase) SetGeneratedIdPrefix(prefix string) {
	f.page.setGeneratedIdPrefix(prefix)
}

func (f *FormBase) GenerateId() string {
	return f.page.generateId()
}

func (f *FormBase) addControl(c ControlI) {
	f.page.addControl(c)
}

// Run is called by the framework on an already existing form to process an action
// or similar kind of request, typically from an AJAX call.
func (f *FormBase) Run(ctx context.Context) (err error) {
	f.this().Preload(ctx)

	request := GetRequest(ctx)

	err = f.testCSRF(ctx, request)
	if err != nil {
		return
	}

	// update posted and get form values from the context for all controls
	for c := range f.AllChildControls() {
		if !c.IsDisabled() {
			c.UpdateFormValues(request)
		}
	}

	// if this is an event response, do the actions associated with the event
	if c := f.GetControl(request.ActionControlID); c != nil {
		c.doAction(ctx)
	}

	// Redraw controls that requested a redraw, probably through the watcher mechanism
	for _, id := range request.RefreshIds {
		if c := f.GetControl(id); c != nil {
			c.Refresh()
		}
	}
	return
}

func (f *FormBase) testCSRF(ctx context.Context, request *RequestContext) error {
	// Test for a CSRF attack
	csrf := f.csrf
	csrf2, found := request.FormValue(HtmlCsrfToken)
	if !found || csrf == "" || csrf != csrf2 {
		log.Warn(ctx, logModule, "Cross-site request forgery attack detected",
			"page_state", f.page.StateID(),
			"found", found,
			"form_csrf", csrf,
			"request_csrf", csrf2)

		return CsrfError
	}
	return nil
}

func (f *FormBase) this() FormI {
	return f.Base.Self().(FormI)
}

// AddHeadTags is a lifecycle call that happens when a new form is created. This is where you should call
// AddHtmlHeaderTag or SetTitle on the page to set tags that appear in the <head> tag of the page.
// Head tags cannot be changed after the page is created.
func (f *FormBase) AddHeadTags() {

}

// Preload is a lifecycle function that gets called whenever a page is first loaded, either by a whole page load,
// or an ajax call.
// Its a good place to validate that the current user should have access to the information on the page.
// You should panic on any errors.
func (f *FormBase) Preload(ctx context.Context) {
}

// CreateControls is a lifecycle function that gets called whenever a page is created. It happens after the Run call.
// This is the place to add controls to the form
func (f *FormBase) CreateControls(ctx context.Context) {
}

// LoadControls is a lifecycle call that happens after a form is first created. It is the place to initialize the value
// of the controls in the form based on variables sent to the form or session variables.
func (f *FormBase) LoadControls(ctx context.Context) {
}

// Exit is a lifecycle function that gets called after the form is processed, just before control is returned to the client.
//
// w is the cached buffered output and can be used to customize the output header.
func (f *FormBase) Exit(ctx context.Context, w http2.ResponseWriter) {
	return
}

// DrawPreRender performs setup operations just before drawing.
func (f *FormBase) DrawPreRender(ctx context.Context, w io.Writer) {
	f.ControlBase.DrawPreRender(ctx, w)

	f.SetAttribute("method", "post")
	// Setting the "action" attribute prevents iFrame clickjacking.
	// This only works because we never ajax draw the whole form, only server render
	request := GetRequest(ctx)
	f.SetAttribute("action", http.MakeLocalPath(request.URL.RequestURI()))

	return
}

// PageDrawingFunction returns the function used to draw the page object.
// If you want a custom drawing function for your page, implement this function in your form override.
func (f *FormBase) PageDrawingFunction() PageDrawFunc {
	return PageTmpl // Returns the default
}

// DrawHeaderTags draws additional header tags for the form.
// The FormBase version will draw the style sheets and javascript file header tags.
// If you override this, be sure to call this version too.
func (f *FormBase) DrawHeaderTags(ctx context.Context, w io.Writer) {
	f.mergeInjectedFiles()
	for path, attr := range f.headerStyleSheets.All() {
		var attributes = attr
		if attributes == nil {
			attributes = html5tag.NewAttributes()
		}
		attributes.Set("rel", "stylesheet")
		attributes.Set("href", path)
		_, _ = io.WriteString(w, html5tag.RenderVoidTag("link", attributes))
	}
	for path, attr := range f.headerJavaScripts.All() {
		var attributes = attr
		if attributes == nil {
			attributes = html5tag.NewAttributes()
		}
		attributes.Set("src", path)
		attributes.Set("type", "text/javascript")
		_, _ = io.WriteString(w, html5tag.RenderTag("script", attributes, ""))
	}

	return
}

func (f *FormBase) mergeInjectedFiles() {
	if f.importedStyleSheets != nil {
		if f.headerStyleSheets == nil {
			f.headerStyleSheets = new(headerItem)
		}
		f.headerStyleSheets.Merge(f.importedStyleSheets)
		f.importedStyleSheets = nil
	}

	if f.importedJavaScripts != nil {
		if f.headerJavaScripts == nil {
			f.headerJavaScripts = new(headerItem)
		}
		f.headerJavaScripts.Merge(f.importedJavaScripts)
		f.importedJavaScripts = nil
	}
}

func (f *FormBase) drawBodyScriptFiles(ctx context.Context, w io.Writer) (err error) {
	f.bodyJavaScripts.Range(func(path string, attr html5tag.Attributes) bool {
		var attributes = attr
		if attributes == nil {
			attributes = html5tag.NewAttributes()
		}
		attributes.Set("src", path)
		attributes.Set("type", "text/javascript")
		if _, err = io.WriteString(w, html5tag.RenderTag("script", attributes, "")+"\n"); err != nil {
			return false
		}
		return true
	})
	return
}

// Response returns the form's response object that you can use to queue up javascript commands to the browser to be
// sent on the next ajax or server request
func (f *FormBase) Response() *Response {
	return &f.response
}

func (f *FormBase) renderAjax(ctx context.Context, w io.Writer) {
	var buf2 []byte
	if f.drawing && !config.Release {
		panic("draw collision")
	}
	f.drawing = true
	defer func() { f.drawing = false }()

	if !f.response.hasExclusiveCommand() { // skip drawing if we are in a high priority situation
		// gather modified controls
		f.DrawAjax(ctx, &f.response)
	}

	// Inject any added style sheets and script files
	if f.importedStyleSheets != nil {
		f.importedStyleSheets.Range(func(k string, v html5tag.Attributes) bool {
			f.response.addStyleSheet(k, v)
			return true
		})
	}

	if f.importedJavaScripts != nil {
		f.importedJavaScripts.Range(func(k string, v html5tag.Attributes) bool {
			f.response.addJavaScriptFile(k, v)
			return true
		})
	}

	f.mergeInjectedFiles()

	f.resetAllDrawingFlags()
	var err error
	buf2, err = f.response.GetAjaxResponse()
	if err != nil {
		panic(err)
	}
	//f.response = NewResponse() Do NOT do this here! It messes with testing framework and multi-processing of ajax responses
	_, err = w.Write(buf2)
	if err != nil {
		panic(err)
	}
	log.Debug(ctx, logModule, "renderAjax", "buffer", string(buf2[:100]))
}

func (f *FormBase) resetAllDrawingFlags() {
	for c := range f.SelfAndAllChildren() {
		c.resetDrawingFlags()
	}
}
