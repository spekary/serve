package control

import (
	"context"

	"github.com/goradd/serve/page"
)

// FormI described the virtual functions that have default functionality which can
// be overridden.
type FormI interface {
	ControlI
	register
	SetupNewForm(ctx context.Context, p *page.Page)
	Preload(ctx context.Context)
	AddHeadTags()
	CreateControls(ctx context.Context)
	LoadControls(ctx context.Context)
}

// Form is a base form control on which to build individual form controls that control a particular
// web page.
type Form struct {
	Control
	registry
	page *page.Page
}

func (f *Form) Init(self FormI, id string) {
	if id == "" {
		panic("id is required")
	}
	f.Control.Init("form", self, self, nil, id)
}

// SetupNewForm is called by the framework's page router whenever a new URL is loaded.
// You should not need to call this function normally.
func (f *Form) SetupNewForm(ctx context.Context, p *page.Page) {
	f.page = p
	f.this().Preload(ctx)
	f.this().AddHeadTags()
	f.this().CreateControls(ctx)
	f.this().LoadControls(ctx)
}

func (f *Form) this() FormI {
	return f.Base.Self().(FormI)
}

func (f *Form) Restore() {}

// AddHeadTags is a lifecycle call that happens when a new form is created. This is where you should call
// AddHtmlHeaderTag or SetTitle on the page to set tags that appear in the <head> tag of the page.
// Head tags cannot be changed after the page is created.
func (f *Form) AddHeadTags() {

}

// Preload is a lifecycle function that gets called whenever a page is first loaded, either by a whole page load,
// or an ajax call.
// Its a good place to validate that the current user should have access to the information on the page.
// You should panic on any errors.
func (f *Form) Preload(ctx context.Context) {
}

// CreateControls is a lifecycle function that gets called whenever a page is created. It happens after the Run call.
// This is the place to add controls to the form
func (f *Form) CreateControls(ctx context.Context) {
}

// LoadControls is a lifecycle call that happens after a form is first created. It is the place to initialize the value
// of the controls in the form based on variables sent to the form or session variables.
func (f *Form) LoadControls(ctx context.Context) {
}
