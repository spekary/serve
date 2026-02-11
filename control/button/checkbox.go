package button

import (
	"context"

	"github.com/goradd/serve/page/action"
	"github.com/goradd/serve/page/event"

	"github.com/goradd/html5tag"
	"github.com/goradd/serve/page"
)

// Checkbox is a basic html checkbox input form control.
type Checkbox struct {
	CheckboxBase
}

// NewCheckbox creates a new checkbox control.
func NewCheckbox(parent page.ControlI, id string) *Checkbox {
	c := &Checkbox{}
	c.CheckboxBase.Init(c, parent, id)
	return c
}

// DrawingAttributes is called by the framework to set the temporary attributes that the control
// needs.
func (c *Checkbox) DrawingAttributes(ctx context.Context) html5tag.Attributes {
	a := c.CheckboxBase.DrawingAttributes(ctx)
	a.SetData(page.ControlTypeDataAttribute, "checkbox")
	a.Set("name", c.ID()) // needed for posts
	a.Set("type", "checkbox")
	a.Set("value", "1") // required for html validity
	return a
}

// UpdateFormValues is an internal call that lets us reflect the value of the checkbox on the form.
// You do not normally need to call this function.
func (c *Checkbox) UpdateFormValues(request *page.RequestContext) {
	c.UpdateCheckboxFormValues(request)
}

type CheckboxCreator struct {
	// ID is the id of the control
	ID string
	// Text is the text of the label displayed right next to the checkbox.
	Text string
	// Checked will initialize the checkbox in its checked state.
	Checked bool
	// LabelMode specifies how the label is drawn with the checkbox.
	LabelMode html5tag.LabelDrawingMode
	// LabelAttributes are additional attributes placed on the label tag.
	LabelAttributes html5tag.Attributes
	// SaveState will save the value of the checkbox and restore it when the page is reentered.
	SaveState bool
	// OnChange is an action to take when the user checks or unchecks the control.
	OnChange action.ActionI

	page.ControlOptions
}

// Create is called by the framework to create a new control from the Creator. You
// do not normally need to call this.
func (c CheckboxCreator) Create(ctx context.Context, parent page.ControlI) page.ControlI {
	ctrl := NewCheckbox(parent, c.ID)
	if c.Text != "" {
		ctrl.SetText(c.Text)
	}
	if c.LabelMode != html5tag.LabelDefault {
		ctrl.LabelMode = c.LabelMode
	}
	if c.LabelAttributes != nil {
		ctrl.LabelAttributes().Merge(c.LabelAttributes)
	}

	ctrl.ApplyOptions(ctx, c.ControlOptions)
	if c.SaveState {
		ctrl.SaveState(ctx, c.SaveState)
	}

	if c.OnChange != nil {
		ctrl.On(event.Change().Action(c.OnChange))
	}
	return ctrl
}

// GetCheckbox is a convenience method to return the checkbox with the given id from the page.
func GetCheckbox(c page.ControlI, id string) *Checkbox {
	cb, _ := c.Form().GetControl(id).(*Checkbox)
	return cb
}

func init() {
	page.RegisterControl(func() page.ControlI { return new(Checkbox) })
}
