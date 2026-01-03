package control

import (
	"context"
	"encoding/gob"
	"fmt"
	http2 "net/http"
	"reflect"

	reflect2 "github.com/goradd/goradd/pkg/any"
	"github.com/goradd/serve/page"
)

// FormI described the virtual functions that have default functionality which can
// be overridden.
type FormI interface {
	Preload(ctx context.Context)
	AddHeadTags()
	CreateControls(ctx context.Context)
	LoadControls(ctx context.Context)
}

// Form is a base form control on which to build individual form controls that control a particular
// web page.
type Form struct {
	Control
	page *page.Page
}

func (f *Form) Init(ctx context.Context, self FormI, p *page.Page) {
	f.Control.Init(self)
	f.page = p
	self.Preload(ctx)
	self.AddHeadTags()
	self.CreateControls(ctx)
	self.LoadControls(ctx)
}

func (f *Form) Run(ctx context.Context) {
	csrf := f.CsrfString()
	csrf2, found := request.FormValue(htmlCsrfToken)
	if !found || csrf == "" || csrf != csrf2 {
		return fmt.Errorf("CSRF error. PageState: %s, Found: %v, Csrf1: %v, Csrf2: %s", p.stateId, found, csrf, csrf2)
	}

	f.this().UpdateValues(ctx) // Tell all the controls to update their values.
	// if this is an event response, do the actions associated with the event
	f.this().DoAction(request.actionControlID)
	/*
		if f.HasControl(request.actionControlID) {
			f.GetControl(request.actionControlID).control().doAction(ctx)
		}*/
	f.this().RefreshContrls(ctx) // find the controls in the request that need refreshing and refresh them
	// Redraw controls that requested a redraw, probably through the watcher mechanism
	/*
		for _, id := range grCtx.refreshIDs {
			if p.HasControl(id) {
				p.GetControl(id).Refresh()
			}
		}*/

}

func (f *Form) Exit(ctx context.Context, w http2.ResponseWriter) {
	f.this().WriteAllStates()
}

func (f *Form) Restore() {}

func (p *Page) encodeControlRegistry(e *gob.Encoder) (err error) {

	// We encode the control registry bottom up so that there is a high likelihood that child controls
	// will be available to parent controls when parent controls get deserialized. This make it possible
	// for us to deserialize forms and custom controls that save a pointer to a control, as long as
	// that pointer is exported.

	// make a copy of the ids
	ids := make(map[string]ControlI)
	for k, v := range p.controlRegistry {
		ids[k] = v
	}

	var l int = len(p.controlRegistry)

	if err = e.Encode(l); err != nil {
		return
	}
	p.form.RangeSelfAndAllChildren(func(ctrl ControlI) {
		p.encodeControl(ctrl, e)
		delete(ids, ctrl.ID())
	})

	// encode controls not attached to the form, like dialogs
	for len(ids) != 0 {
		// process one item out of map at a time
		// we need to do it this way because these unattached items might have children, and we must
		// ensure that all children get serialized first
		for _, c := range ids {
			c.RangeSelfAndAllChildren(
				func(ctrl ControlI) {
					if _, ok := ids[ctrl.ID()]; ok { // we didn't yet process it
						p.encodeControl(ctrl, e)
						delete(ids, ctrl.ID())
					}
				})
			break
		}
	}
	return
}

func (p *Page) encodeControl(ctrl ControlI, e *gob.Encoder) {
	if err := e.Encode(ctrl.ID()); err != nil {
		panic(err)
	}
	if err := e.Encode(controlRegistryID(ctrl)); err != nil {
		panic(err)
	}

	p.serializeControl(ctrl, e)
}

// Users can create exported items on their objects and they will be serialized and restored automatically
// Alternatively they can implement their own Serialize method.
func (p *Page) serializeControl(c ControlI, e Encoder) {
	v := reflect.Indirect(reflect.ValueOf(c))
	fieldCount := v.NumField()
	_ = fieldCount
	exportedFields := reflect2.FieldValues(c)

	// convert all embedded controls to the id of the control
	for name, val := range exportedFields {
		if ctrl, ok := val.(ControlI); ok {
			exportedFields[name] = controlCode + ctrl.ID()
		}
	}
	c.Serialize(e)
	if err := e.Encode(exportedFields); err != nil {
		panic("Error serializing exported fields of " + c.ID() + ": " + err.Error())
	}
}

func (p *Page) decodeControlRegistry(d *gob.Decoder) (err error) {
	p.controlRegistry = make(map[string]ControlI)
	var l int
	if err = d.Decode(&l); err != nil {
		panic(err)
	}

	for i := 0; i < l; i++ {
		if err = p.decodeControl(d); err != nil {
			return
		}
	}
	return
}

func (p *Page) decodeControl(d *gob.Decoder) (err error) {
	var id string
	var registryID uint64
	if err = d.Decode(&id); err != nil {
		panic(err)
	}
	if err = d.Decode(&registryID); err != nil {
		return
	}

	c := createRegisteredControl(registryID, p)
	p.controlRegistry[id] = c
	p.deserializeControl(c, d)
	return
}

func (p *Page) deserializeControl(c ControlI, d Decoder) {
	c.Deserialize(d)
	var exportedFields map[string]interface{}
	if err := d.Decode(&exportedFields); err != nil {
		panic(err)
	}
	// Substitute embedded control ids for the actual control
	for name, val := range exportedFields {
		if s, ok := val.(string); ok && strings2.StartsWith(s, controlCode) {
			id := s[len(controlCode):]
			if ctrl, ok2 := p.controlRegistry[id]; ok2 {
				exportedFields[name] = ctrl
			}
		}
	}

	if err := reflect2.SetFieldValues(c, exportedFields); err != nil {
		panic(err)
	}
}
