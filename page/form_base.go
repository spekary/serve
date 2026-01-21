package page

import (
	"context"
	"encoding/gob"
	"errors"
	"io"
	http2 "net/http"
	"path"

	"github.com/goradd/html5tag"
	"github.com/goradd/maps"
	"github.com/goradd/serve/config"
	"github.com/goradd/serve/http"
	"github.com/goradd/serve/i18n"
	"github.com/goradd/serve/log"
	"github.com/goradd/serve/messenger"
	"golang.org/x/text/language"
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
	Page() *Page
	GetControl(id string) (c ControlI)
	SetGeneratedIdPrefix(prefix string)
	GenerateId() string

	SetupNewForm(ctx context.Context)
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
	LanguageTag() language.Tag

	AddStyleSheetFile(path string, attributes html5tag.Attributes)
	AddJavaScriptFile(path string, forceHeader bool, attributes html5tag.Attributes)
	AddFrameworkFiles()

	renderAjax(ctx context.Context, w io.Writer)
	addControl(c ControlI)
	setPage(p *Page)
}

type headerItem = maps.SliceMap[string, html5tag.Attributes]

// FormBase is a base form control on which to build individual form controls that control a particular
// web page.
type FormBase struct {
	ControlBase
	page *Page

	drawing             bool
	response            Response
	headerStyleSheets   headerItem
	importedStyleSheets headerItem // when refreshing, these get moved to the headerStyleSheets
	headerJavaScripts   headerItem
	bodyJavaScripts     headerItem
	importedJavaScripts headerItem // when refreshing, these get moved to the bodyJavaScripts
	csrf                string     // csrf attack check string
	languageIndex       int        // The index into the list of i18n languages supported by the app that the web page will be translated into.
}

func (f *FormBase) Init(self FormI, id string) {
	if id == "" {
		panic("id is required")
	}
	f.ControlBase.Init(self, nil, id)
	f.Tag = "form"
}

// SetupNewForm is called by the framework's page router whenever a new URL is loaded.
// You should not need to call this function normally.
func (f *FormBase) SetupNewForm(ctx context.Context) {
	f.this().Preload(ctx)
	f.this().AddHeadTags()
	f.this().CreateControls(ctx)
	f.this().LoadControls(ctx)

	// Cache and save the language tag for situations
	// where we do not have the context
	_, f.languageIndex, _ = i18n.LanguageFromContext(ctx)
}

func (f *FormBase) LanguageTag() language.Tag {
	l, _ := i18n.LanguageFromIndex(f.languageIndex)
	return l
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

// AddRelatedFiles is called by the framework when drawing the form to add JavaScript, style sheets, and other related files
// to the form.
//
// In your override, you would typically call [FormBase.AddJavaScriptFile] and [FormBase.AddStyleSheetFile] to add the files
// to the form, and then call this parent version of the function to get the default functionality.
func (f *FormBase) AddRelatedFiles() {
	f.this().AddFrameworkFiles()
	if messenger.Messenger != nil {
		files := messenger.Messenger.JavascriptFiles()
		for file, attr := range files {
			f.AddJavaScriptFile(file, false, attr)
		}
	}
}

// addGoraddFiles is called by the framework to add the various goradd files to the form.
func (f *FormBase) AddFrameworkFiles() {
	f.AddJavaScriptFile(path.Join(config.AssetPath, "goradd", "js", "goradd.js"), false, nil)
	if !config.Release {
		f.AddJavaScriptFile(path.Join(config.AssetPath, "goradd", "test", "js", "goradd-test.js"), false, nil)
	}
	if config.Debug {
		f.AddJavaScriptFile(path.Join(config.AssetPath, "goradd", "js", "goradd-debug.js"), false, nil)
	}

	f.AddStyleSheetFile(path.Join(config.AssetPath, "goradd", "css", "goradd.css"), nil)
}

// AddJavaScriptFile registers a JavaScript file such that it will get loaded on the page.
//
// The path is either a url, or an internal path to the location of the file
// in the development environment.
//
// If forceHeader is true, the file will be listed in the header, which you should only do if the file has some
// preliminary javascript that needs to be executed before the dom loads.
// You can specify forceHeader and a "defer" attribute to get the effect of loading the javascript in the background.
// With forceHeader false, the file will be loaded after
// the dom is loaded, allowing the browser to show the page and then load the javascript in the background, giving the
// appearance of a more responsive website. If you add the file during an ajax operation, the file will be loaded
// dynamically by the goradd javascript. Controls generally should call this during the initial creation of the control if the control
// requires additional javascript to function.
//
// attributes are the attributes that will be included with the script tag, which is useful for things like
// crossorigin and integrity attributes.
func (f *FormBase) AddJavaScriptFile(path string, forceHeader bool, attributes html5tag.Attributes) {
	if forceHeader && f.isOnPage {
		panic("You cannot force a JavaScript file to be in the header if you insert it after the page is drawn.")
	}

	if path[:4] != "http" {
		url := http.GetAssetUrl(path)

		if url == "" {
			panic(path + " is not in a registered asset directory")
		}
		path = url
	}

	if f.isOnPage {
		if f.headerJavaScripts.Has(path) ||
			f.bodyJavaScripts.Has(path) {
			return // file is already on the page
		}
		f.importedJavaScripts.Set(path, attributes)
	} else if forceHeader {
		f.headerJavaScripts.Set(path, attributes)
	} else {
		f.bodyJavaScripts.Set(path, attributes)
	}
}

// AddMasterJavaScriptFile adds a javascript file that is a concatenation of other javascript files the system uses.
// This allows you to concatenate and minimize all the javascript files you are using without worrying about
// libraries and controls that are adding the individual files through the AddJavaScriptFile function
func (f *FormBase) AddMasterJavaScriptFile(url string, attributes []string, files []string) {
	// TODO
}

// AddStyleSheetFile registers a StyleSheet file such that it will get loaded on the page.
// The file will be loaded on the page at initial draw in the header, or will be inserted into the file if the page
// is already drawn. The path is either a url to an external resource, or a local directory to a resource on disk.
// Paths must be registered with RegisterAssetDirectory, and will be served from their local location in a development environment,
// but from the corresponding registered path when deployed.
//
// attributes are the attributes that will be included with the link tag, which is useful for things like
// crossorigin and integrity attributes.
//
// To control the cache-control settings on the file, you should call SetCacheControl.
func (f *FormBase) AddStyleSheetFile(path string, attributes html5tag.Attributes) {
	if path[:4] != "http" {
		url := http.GetAssetUrl(path)

		if url == "" {
			panic(path + " is not in a registered asset directory")
		}
		path = url
	}

	if f.isOnPage {
		if f.headerStyleSheets.Has(path) {
			return // the style sheet was already included when the form was loaded the first time
		}
		f.importedStyleSheets.Set(path, attributes)
	} else {
		f.headerStyleSheets.Set(path, attributes)
	}
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
	f.headerStyleSheets.Copy(&f.importedStyleSheets)
	f.importedStyleSheets.Clear()

	f.headerJavaScripts.Copy(&f.importedJavaScripts)
	f.importedJavaScripts.Clear()
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
	for k, v := range f.importedStyleSheets.All() {
		f.response.addStyleSheet(k, v)
	}

	for k, v := range f.importedJavaScripts.All() {
		f.response.addJavaScriptFile(k, v)
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

func (f *FormBase) setPage(p *Page) {
	f.page = p
}

func (f *FormBase) Page() (p *Page) {
	return f.page
}

func (f *FormBase) Serialize(e Encoder) {
	f.ControlBase.Serialize(e)
	if !config.Release {
		// The response is currently only changed between posts by the testing framework
		// If we ever need to change forms using some kind of push mechanism, we will need to serialize
		// the response.
		f.response.Serialize(e)
	}

	if err := e.Encode(&f.headerStyleSheets); err != nil {
		panic(err)
	}
	if err := e.Encode(&f.importedStyleSheets); err != nil {
		panic(err)
	}
	if err := e.Encode(&f.headerJavaScripts); err != nil {
		panic(err)
	}
	if err := e.Encode(&f.bodyJavaScripts); err != nil {
		panic(err)
	}
	if err := e.Encode(&f.importedJavaScripts); err != nil {
		panic(err)
	}
	if err := e.Encode(f.csrf); err != nil {
		panic(err)
	}
	if err := e.Encode(f.languageIndex); err != nil {
		panic(err)
	}
}

func (f *FormBase) Deserialize(d Decoder) {
	f.ControlBase.Deserialize(d)

	if !config.Release {
		// The response is currently only changed between posts by the testing framework
		// If we ever need to change forms using some kind of push mechanism, we will need to serialize
		// the response.
		f.response.Deserialize(d)
	}

	if err := d.Decode(&f.headerStyleSheets); err != nil {
		panic(err)
	}
	if err := d.Decode(&f.importedStyleSheets); err != nil {
		panic(err)
	}
	if err := d.Decode(&f.headerJavaScripts); err != nil {
		panic(err)
	}
	if err := d.Decode(&f.bodyJavaScripts); err != nil {
		panic(err)
	}
	if err := d.Decode(&f.importedJavaScripts); err != nil {
		panic(err)
	}
	if err := d.Decode(&f.csrf); err != nil {
		panic(err)
	}
	if err := d.Decode(&f.languageIndex); err != nil {
		panic(err)
	}

	return
}

func init() {
	gob.Register(&FormBase{})
	gob.Register(new(headerItem))
}

type MockForm struct {
	FormBase
}

func NewMockForm() *MockForm {
	f := new(MockForm)
	f.Init(f, "MockFormID")
	return f
}

func (f *MockForm) Init(self FormI, id string) {
	f.setPage(new(Page))
	f.FormBase.Init(self, id)
}

func (f *MockForm) AddRelatedFiles() {
}
