// Package page is the user-interface layer of goradd, and implements state management and rendering
// of an html page, as well as the framework for rendering controls.
//
// To use the page package, you start by creating a form object, and then add controls to that form.
// You also should add a drawing template to define additional html for the form.
package page

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	http2 "net/http"

	"github.com/goradd/html5tag"
	"github.com/goradd/serve/i18n"
	"github.com/goradd/serve/log"
)

// PageRenderStatus keeps track of whether we are rendering the page or not
type PageRenderStatus int

// Future note. Below is for general information but should NOT be used to synchronize multiple drawing routines.
// An architecture using channels to synchronize page changes and drawing would be better.
// For now, except for testing, we should not get in a situation where multiple copies of a form
// are being used.
const (
	PageIsNotRendering PageRenderStatus = iota // FormBase has started rendering but has not finished
	PageIsRendering
)

// PageCacheVersion helps us keep track of when a change to the application changes the pagecache format. It is only needed
// when serializing the pagecache. Some page cache stores may be difficult to invalidate the whole thing, so this lets
// us invalidate old pagecaches individually. Feel free to bump this as needed, though you should use
// a number after UserPageCacheVersion so there is no conflict with the goradd default.
var PageCacheVersion int32 = 1

// ControlRegistrySalt is used to generate unique ids in the control registry. However, if the control registry
// detects a collision, you will need to change this value and restart your app. If you have a running
// page cache, you should change the PageCacheVersion above as well to invalidate it.
var ControlRegistrySalt = "234$sfbg"

// UserPageCacheVersion is a version number you can use as a starting point if you want to keep
// track of the page cache version yourself.
const UserPageCacheVersion = 10000

// PageDrawFunc is the type for the page drawing function.
// This is implemented by the page drawing template, but can be implemented in other ways.
type PageDrawFunc func(context.Context, *Page, io.Writer) error

// Drawer is the interface for items that draw into the draw buffer.
type Drawer interface {
	Draw(context.Context, io.Writer)
}

// A code we use during serialization to indicate that we just unserialized a control id
const controlCode = "**grc**"

// The Page object is the top level drawing object, and is essentially a wrapper for the form. The Page draws the
// html, head and body tags, and includes the one Form object on the page. The page also maintains a record of all
// the controls included on the form.
type Page struct {
	// BodyAttributes contains the attributes that will be output with the body tag. It should be set before the
	// form draws, like in the AddHeadTags function.
	BodyAttributes string

	stateId      string // Id in cache of the pagestate. Needs to be output by form.
	renderStatus PageRenderStatus
	idPrefix     string // For creating unique ids for the app

	form           FormI
	idCounter      int
	title          string // page title to draw in head tag
	htmlHeaderTags []html5tag.VoidTag
	responseError  int

	language int // Don't serialize this. This is a cached version of what the session holds.
}

// Init initializes the page. Should be called by a form just after creating Page.
func (p *Page) Init() {
}

func (p *Page) runPage(ctx context.Context, w http2.ResponseWriter) (err error) {
	defer func() {
		p.renderStatus = PageIsNotRendering
		log.Debug(ctx, "page", "Page stopped rendering")
	}()
	p.renderStatus = PageIsRendering
	log.Debug(ctx, "page", "Page started rendering", "page_state", p.stateId)

	request := GetRequest(ctx)

	// cache the language tags so we only need to look them up once for every call
	//p.language = i18n.SetDefaultLanguage(ctx, grCtx.Header.Get("accept-language"))

	if p.form == nil {
		// Create a new form and draw it after running through initial setup functions.
		path := request.URL.Path
		f := creationFunction(path)
		if f == nil {
			panic("form not found for path: " + path)
		}
		p.form = f(ctx, p)
		p.Draw(ctx, w)
	} else {
		p.form.Run(ctx)
		if request.RequestMode() == RequestModeAjax {
			p.DrawAjax(ctx, w)
			w.Header().Add("Content-Type", "application/json")
		} else if request.RequestMode() == RequestModeServer {
			p.Draw(ctx, w)
		}
	}

	p.Form().Exit(ctx, w)

	pageCache.Set(p.stateId, p)

	return
}

// Form returns the form for the page.
func (p *Page) Form() FormI {
	return p.form
}

// Draw draws the page.
func (p *Page) Draw(ctx context.Context, w io.Writer) {
	f := p.form.PageDrawingFunction()
	if err := f(ctx, p, w); err != nil {
		panic(err)
	}
}

// DrawHeaderTags draws all the inner html for the head tag
func (p *Page) DrawHeaderTags(ctx context.Context, w io.Writer) {
	if p.title != "" {
		if _, err := io.WriteString(w, "  <title>"); err != nil {
			panic(err)
		}
		if _, err := io.WriteString(w, p.title); err != nil {
			panic(err)
		}
		if _, err := io.WriteString(w, "  </title>\n"); err != nil {
			panic(err)
		}
	}

	// draw things like additional meta tags, etc
	if p.htmlHeaderTags != nil {
		for _, tag := range p.htmlHeaderTags {
			if _, err := io.WriteString(w, tag.Render()); err != nil {
				panic(err)
			}
		}
	}

	p.Form().DrawHeaderTags(ctx, w)
	return
}

// Title returns the content of the <title> tag that will be output in the head of the page.
func (p *Page) Title() string {
	return p.title
}

// SetTitle sets the content of the <title> tag.
func (p *Page) SetTitle(title string) {
	p.title = title
}

// StateID returns the page state id. This is output by the form so that we can recover the saved state of the page
// each time we call into the application.
func (p *Page) StateID() string {
	return p.stateId
}

// DrawAjax renders the page during an ajax call. Since the page itself is already rendered, it simply hands off this
// responsibility to the form.
func (p *Page) DrawAjax(ctx context.Context, w io.Writer) {
	p.form.RenderAjax(ctx, w)
	return
}

func (p *Page) MarshalJSON() (data []byte, err error) {
	return
}

func (p *Page) UnmarshalJSON(data []byte) (err error) {
	return
}

// MarshalBinary is called by the framework to serialize the page state.
func (p *Page) MarshalBinary() (data []byte, err error) {
	var buf bytes.Buffer
	e := gob.NewEncoder(&buf)
	if err = e.Encode(PageCacheVersion); err != nil {
		return
	}
	if err = e.Encode(p.stateId); err != nil {
		return
	}
	if err = e.Encode(p.idPrefix); err != nil {
		return
	}
	if err = e.Encode(p.title); err != nil {
		return
	}
	if err = e.Encode(p.htmlHeaderTags); err != nil {
		return
	}
	if err = e.Encode(p.BodyAttributes); err != nil {
		return
	}
	if err = e.Encode(p.form); err != nil {
		return
	}

	data = buf.Bytes()
	return
}

func (p *Page) UnmarshalBinary(data []byte) (err error) {
	b := bytes.NewBuffer(data)
	dec := gob.NewDecoder(b)

	var pageCacheVersion int32
	if err = dec.Decode(&pageCacheVersion); err != nil {
		panic(err)
	}
	if pageCacheVersion != PageCacheVersion {
		return fmt.Errorf("stale data in cache") // This is a soft error indicating that the system should create a new page state
	}

	if err = dec.Decode(&p.stateId); err != nil {
		panic(err)

	}
	if err = dec.Decode(&p.idPrefix); err != nil {
		panic(err)
	}
	if err = dec.Decode(&p.title); err != nil {
		panic(err)
	}
	if err = dec.Decode(&p.htmlHeaderTags); err != nil {
		panic(err)
	}
	if err = dec.Decode(&p.BodyAttributes); err != nil {
		panic(err)
	}
	if err = dec.Decode(&p.form); err != nil {
		panic(err)
	}

	return
}

// AddHtmlHeaderTag adds the given tag to the head section of the page.
func (p *Page) AddHtmlHeaderTag(t html5tag.VoidTag) {
	p.htmlHeaderTags = append(p.htmlHeaderTags, t)
}

func (p *Page) HasMetaTag(name string) bool {
	for _, t := range p.htmlHeaderTags {
		if t.Tag == "meta" &&
			t.Attr["name"] == name {
			return true
		}
	}
	return false
}

// PushRedraw will cause the form to refresh in between events. This will cause the client to pull
// the ajax response. Its possible that this will happen while drawing. We avoid the race condition
// by sending the message anyways, and allowing the client to send an event back to us, essentially
// using the javascript event mechanism to synchronize us. We might get an unnecessary redraw, but
// that is not a big deal.
/*
func (p *Page) PushRedraw() {
	channel := "form-" + p.stateId
	if ws.HasChannel(channel) { // If we call this while launching a page, the channel isn't created yet, but the page is going to be drawn, so its ok.
		ws.SendMessage(channel, map[string]interface{}{"grup": true})
	} else {
		log.FrameworkDebug("Pushing redraw with no channel.")
	}
}
*/

// LanguageCode returns the language code that will be put in the lang attribute of the html tag.
// It is taken from the i18n package.
func (p *Page) LanguageCode() string {
	return i18n.CanonicalValue(p.language)
}

// Cleanup is called by the page cache when the page is removed from memory.
func (p *Page) Cleanup() {
	p.Form().Cleanup()
}

// Restore is called immediately after the page has been deserialized, to fix up decoded controls.
func (p *Page) Restore() {
	p.Form().Restore()
}
