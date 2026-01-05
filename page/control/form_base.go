package control

import (
	"context"
	"errors"

	"github.com/goradd/html5tag"
	"github.com/goradd/maps"
	"github.com/goradd/serve/log"
	"github.com/goradd/serve/page"
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
	cacher
	SetupNewForm(ctx context.Context, p *page.Page)
	Run(ctx context.Context) error
	Preload(ctx context.Context)
	AddHeadTags()
	CreateControls(ctx context.Context)
	LoadControls(ctx context.Context)
	Unmarshalled()
	Cleanup()
}

type headerItem = maps.SliceMap[string, html5tag.Attributes]

// FormBase is a base form control on which to build individual form controls that control a particular
// web page.
type FormBase struct {
	ControlBase
	cache
	page *page.Page

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
func (f *FormBase) SetupNewForm(ctx context.Context, p *page.Page) {
	f.page = p
	f.this().Preload(ctx)
	f.this().AddHeadTags()
	f.this().CreateControls(ctx)
	f.this().LoadControls(ctx)
}

// Run is called by the framework on an already existing form to process an action
// or similar kind of request, typically from an AJAX call.
func (f *FormBase) Run(ctx context.Context) (err error) {
	f.this().Preload(ctx)

	request := page.GetRequest(ctx)

	err = f.testCSRF(ctx, request)
	if err != nil {
		return
	}

	// update posted and get form values from the context for all controls
	for c := range f.AllChildren() {
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

func (f *FormBase) testCSRF(ctx context.Context, request *page.RequestContext) error {
	// Test for a CSRF attack
	csrf := f.csrf
	csrf2, found := request.FormValue(HtmlCsrfToken)
	if !found || csrf == "" || csrf != csrf2 {
		log.Warn(ctx, "control", "Cross-site request forgery attack detected",
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

func (f *FormBase) Unmarshalled() {}
func (f *FormBase) Cleanup()      {}

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
