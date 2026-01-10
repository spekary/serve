package page

import (
	"context"
	"encoding/gob"
	"fmt"
	"html"
	"io"
	"iter"
	"reflect"

	"github.com/goradd/base"
	"github.com/goradd/goradd/pkg/orm/query"
	"github.com/goradd/goradd/pkg/stringmap"
	"github.com/goradd/html5tag"
	"github.com/goradd/maps"
	"github.com/goradd/serve/config"
	"github.com/goradd/serve/i18n"
	"github.com/goradd/serve/log"
	"github.com/goradd/serve/page/action"
	"github.com/goradd/serve/page/event"
	"github.com/goradd/serve/page/javascript"
	"github.com/goradd/serve/pool"
	"github.com/goradd/serve/session"
)

const sessionControlStates string = "goradd.controlStates"
const sessionControlTypeState string = "goradd.controlType"

const RequiredErrorMessage string = "A value is required"

const ControlTypeDataAttribute = "grctl"

// ValidationState is used internally by the framework to determine how the control's wrapper handles drawing validation error
// messages. Different wrappers use it to set classes or attributes of the error message or the overall control.
type ValidationState int

type eventMap map[event.EventID]*event.Event

const (
	// ValidationWaiting is the default for controls that accept validation. It means that the control expects to be validated,
	// but has not yet been validated. Wrappers should save a spot for the error message of this control so that if
	// an error appears, it will not change the layout of the form.
	ValidationWaiting ValidationState = iota
	// ValidationNever indicates that the control will never fail validation. Essentially it indicates that the wrapper does not
	// need to save a spot for an error message for this control.
	ValidationNever
	// ValidationValid indicates the control has been validated. This state gets entered if some control on the form has failed validation, but
	// this control passed validation. You can choose to display a special message, or a special color, etc., to
	// indicate to the user that this is not the source of the validation problem, or do nothing.
	ValidationValid
	// ValidationInvalid indicates the control has failed validation, and the wrapper should somehow call that out to the user. The error message
	// should be displayed at a minimum, but likely other things should happen as well, like a special color, and
	// aria attributes should be set.
	ValidationInvalid
)

// ControlTemplateFunc is the type of function control templates should create
type ControlTemplateFunc func(ctx context.Context, control ControlI, w io.Writer) error

// ControlWrapperFunc is a template function that specifies how wrappers will draw
type ControlWrapperFunc func(ctx context.Context, control ControlI, ctrl string, w io.Writer) error

// SavedState is the container for saving control states so that the control state can be restored when
// the user comes back to the page.
//
// This is particularly useful for controls that affect the appearance of
// the page. An example would be a textbox or dropdown that filters a list of items.
type SavedState = maps.MapI[string, any]

type stateType = maps.Map[string, any]
type stateStoreType = maps.Map[string, SavedState]

// DefaultCheckboxLabelDrawingMode is a setting used by checkboxes and radio buttons to default how they draw labels.
// Some CSS frameworks are very picky about whether checkbox labels wrap the control, or sit next to the control,
// and whether the label is before or after the control
var DefaultCheckboxLabelDrawingMode = html5tag.LabelAfter

// The DataConnector moves data between the control and the database model. It is a thin view-model controller
// that can be customized on a per-control basis.
type DataConnector interface {
	// Refresh reads from the model, and puts it into the control
	Refresh(i ControlI, model any)
	// Update reads data from the control, and puts it into the model
	Update(i ControlI, model any)
	// Modifies returns true if the control has been changed such that it will modify its corresponding data
	Modifies(i ControlI, model any) bool
}

// DataLoader is an optional interface that DataConnectors can use if they need to load data from the database
// to present a choice of items to the user to select from. The Load method will be called whenever the entire control
// gets redrawn.
type DataLoader interface {
	Load(ctx context.Context) []any
}

// ControlI is the interface that all controls must support. The functions are implemented by the
// ControlBase methods. See the ControlBase method implementation for a description of each method.

type ControlI interface {
	base.BaseI
	treeNoder

	// Drawing support

	Draw(ctx context.Context, w io.Writer)
	DrawTag(context.Context, io.Writer)
	DrawInnerHtml(context.Context, io.Writer)
	DrawTemplate(context.Context, io.Writer) error
	DrawPreRender(context.Context, io.Writer)
	DrawPostRender(context.Context, io.Writer)
	DrawAjax(ctx context.Context, response *Response)
	DrawChildControls(ctx context.Context, w io.Writer)
	DrawText(ctx context.Context, w io.Writer)
	PutCustomScript(ctx context.Context, response *Response)
	NeedsRefresh() bool
	WasRendered() bool
	IsOnPage() bool
	Refresh()
	IsRendering() bool
	IsVisible() bool
	SetVisible(bool)

	// html and css

	SetTag(tag string) ControlI
	SetAttribute(name string, val interface{}) ControlI
	Attribute(string) string
	HasAttribute(string) bool
	ProcessAttributeString(s string) ControlI
	DrawingAttributes(context.Context) html5tag.Attributes
	AddClass(class string) ControlI
	RemoveClass(class string) ControlI
	HasClass(class string) bool
	SetStyles(html5tag.Style)
	SetStyle(name string, value string) ControlI
	SetWidthStyle(w interface{}) ControlI
	SetHeightStyle(w interface{}) ControlI
	Attributes() html5tag.Attributes
	MergeAttributes(a html5tag.Attributes) ControlI
	SetDisplay(d string) ControlI

	UpdateFormValues(request *RequestContext)
	SetDisabled(d bool)
	IsDisabled() bool

	SetTextIsHtml(bool) ControlI
	TextIsLabel() bool
	Text() string
	SetText(t string) ControlI

	SetActionValue(v any) ControlI
	ActionValue() any
	DoAction(ctx context.Context, a action.Params)
	DoPrivateAction(ctx context.Context, a action.Params)
	AddRenderScript(f string, params ...any)
	AddRelatedRenderScript(id string, f string, params ...any)
	WrapEvent(eventName string, selector string, eventJs string, options map[string]interface{}) string

	Validate(ctx context.Context) bool
	ValidationState() ValidationState
	ValidationType(*event.Event) event.ValidationType
	SetValidationType(typ event.ValidationType) ControlI
	ChildValidationChanged()
	ResetValidation()
	ValidationMessage() string
	SetValidationError(e string)

	Serialize(e Encoder)
	Deserialize(d Decoder)
	Deserialized()

	// SaveState tells the control whether to save the basic state of the control, so that when the form is reentered, the
	// data in the control will remain the same. This is particularly useful if the control is used as a filter for the
	// contents of another control.
	SaveState(context.Context, bool)
	MarshalState(m SavedState)
	UnmarshalState(m SavedState)

	// Creator

	ApplyOptions(ctx context.Context, o ControlOptions)
	AddControls(ctx context.Context, creators ...Creator)

	// package private functions

	doAction(ctx context.Context)
	initBase(self ControlI)
	validationTypeValue() event.ValidationType
	blockParentValidationValue() bool
	validateSelfAndChildren(ctx context.Context) bool
	validateSelfAndSiblings(ctx context.Context) bool
	validateSiblingsAndChildren(ctx context.Context) bool
	resetValidationValues() (changed bool)
	shouldAutoRenderValue() bool
	markOnPage(v bool)
	resetDrawingFlags()
}

type attributeScriptEntry struct {
	id       string // id of the object to execute the command on. This should be the id of the control, or a related html object.
	f        string // the  function to call
	commands []any  // parameters to the function
}

// ControlBase is the basis for UI controls and widgets in GoRADD.
// It corresponds to a standard html form object or tag.
// A ControlBase can also associate javascript
// with itself to make sure the javascript is loaded on the page when the control is drawn, and can render
// javascript that will initialize a custom javascript widget.
//
// A ControlBase can have child Controls. It
// can either allow the framework to automatically draw the child Controls as part of the inner-html of
// the ControlBase, can use a template to draw the Child controls, or manually draw them. Controls form
// a hierarchical tree structure, with the FormBase control being the root of the tree.
//
// A ControlBase is part of a system that will reflect the state of the control between the client and server.
// When a user updates a control in the browser and performs an action that requires a response from the
// server, the GoRADD javascript will gather all the changes in the form and send those to the server.
// The control can read those values and update its own internal state, so that from the perspective
// of the programmer referring to the control, the values in the ControlBase are the same as what the user sees in a browser.
//
// This ControlBase struct is a mixin that all controls should use. You would not normally create a ControlBase directly,
// but rather create one of the "subclasses" of ControlBase.
type ControlBase struct {
	base.Base
	treeNode

	// Tag is the tag that will enclose the control, like "div" or "input".
	// It should only be set during the initialization phase of a control, preferably
	// immediately after calling Init.
	// Once the control is drawn, it should not be changed.
	Tag string
	// IsVoidTag should be true if the tag should not have a closing tag, like "img"
	IsVoidTag bool
	// hasNoSpace is for special situations where we want no space between this and the next tag.
	// Spans in particular may need this.
	hasNoSpace bool
	// attributes are the collection of custom attributes to apply to the control. This does not include all the
	// attributes that will be drawn, as some are added temporarily just before drawing by GetDrawingAttributes()
	attributes html5tag.Attributes
	// text is a multipurpose string that can be button text, inner text inside of tags, etc. depending on the control.
	text string
	// textLabelMode describes how to draw the internal label
	textLabelMode html5tag.LabelDrawingMode
	// textIsHtml will prevent the text output from being escaped
	textIsHtml bool

	// attributeScripts are commands to send to our javascript to redraw portions of the control via ajax.
	attributeScripts []attributeScriptEntry

	// isRequired indicates that we will require a value during validation
	isRequired bool
	// isHidden indicates that we will not draw the control, but rather an invisible placeholder for the control.
	isHidden bool
	// isOnPage indicates we have drawn the control at some point in the past
	isOnPage bool
	// shouldAutoRender indicates that we will eventually draw the control even if it is not drawn directly.
	shouldAutoRender bool

	// internal status functions. Do not serialize.

	// needsRefresh will cause the control to redraw as part of the response.
	needsRefresh bool
	// isRendering is true when we are in the middle of rendering the control.
	isRendering bool
	// wasRendered indicates that the page was drawn during the current response.
	wasRendered bool

	// isBlock is true to use a div for the wrapper, false for a span
	isBlock bool

	// ErrorForRequired is the error that will display if a control value is required but not set.
	ErrorForRequired string

	// ValidMessage is the message to display if the control has successfully been validated.
	// Leave blank if you don't want a message to show when valid.
	// Can be useful to contrast between invalid and valid controls in a busy form.
	ValidMessage string
	// validationMessage is the current validation message that will display when drawing the control
	// This gets copied from ValidMessage at drawing time if the control is in an invalid state
	validationMessage string
	// validationState is the current validation state of the control, and will affect how the control is drawn.
	validationState ValidationState
	// validationType indicates how the control will validate itself. See ValidationType for a description.
	validationType event.ValidationType
	// validationTargets is the list of control IDs to target validation
	validationTargets []string
	// This blocks a parent from validating this control. Useful for dialogs, and other situations where sub-controls should control their own space.
	blockParentValidation bool

	// actionValue is the value that will be provided as the ControlValue for any actions that are triggered by this control.
	actionValue any
	// events are all the events added by the control user that the control might trigger
	events eventMap
	// eventCounter is used to generate a unique id for an event to help us route the event through the system.
	eventCounter event.EventID
	// shouldSaveState indicates that we should save parts of our state into a session variable so that if
	// the client should come back to the form, we will attempt to restore the state of the control. The state
	// in this situation would be the user's input, so text in a Textbox, or the selection from a list.
	shouldSaveState bool
	// encoded is used during the serialization process to prevent encoding a control multiple times.
	encoded bool

	// dataConnector automates the transfer of data between a data store and the control.
	//dataConnector DataConnector
	// if we implement this, break it into a mixin

	// watchedKeys are the notification keys that will cause the control to refresh.
	watchedKeys map[string]string

	// anything added here needs to be also added to the GOB encoder!
}

// Init initializes a control.
func (c *ControlBase) Init(self ControlI, parent ControlI, id string) {
	c.Base.Init(self)
	var form FormI
	if parent != nil {
		form = parent.Form()
	}
	c.treeNode.Init(self, form, parent, id)
	c.needsRefresh = true
	// parent.Refresh() should we do this automatically, or allow user to control this?
	c.attributes = html5tag.NewAttributes() // Almost always will have attributes, so create the memory up front
}

// this supports object-oriented features by giving easy access to the virtual function interface.
// Subclasses should provide a duplicate. Calls that implement chaining should return the result of this function.
func (c *ControlBase) this() ControlI {
	return c.Self().(ControlI)
}

// initBase is called by the framework during the deserialization process
func (c *ControlBase) initBase(self ControlI) {
	c.Base.Init(self)
}

func (c *ControlBase) SetStyles(style html5tag.Style) {
	c.attributes.SetStyles(style)
	c.Refresh() // TODO: Do this with javascript
}

func (c *ControlBase) SetStyle(name string, value string) ControlI {
	if changed, _ := c.attributes.SetStyleChanged(name, value); changed {
		c.Refresh() // TODO: Do this with javascript
	}
	return c.this()
}

// RemoveClassesWithPrefix will remove the classes on a control that start with the given string.
// Some CSS frameworks use prefixes to as a kind of namespace for their class tags, and this can
// make it easier to remove a group of classes with this kind of prefix.
func (c *ControlBase) RemoveClassesWithPrefix(prefix string) {
	if c.attributes.RemoveClassesWithPrefix(prefix) {
		c.Refresh() // TODO: Do this with javascript
	}
}

func (c *ControlBase) SetWidthStyle(w interface{}) ControlI {
	v := html5tag.StyleString(w)
	c.attributes.SetStyle("width", v)
	c.AddRenderScript("css", "width", v) // use javascript to set this value
	return c.this()
}

func (c *ControlBase) SetHeightStyle(h interface{}) ControlI {
	v := html5tag.StyleString(h)
	c.attributes.SetStyle("height", v)
	c.AddRenderScript("css", "height", v) // use javascript to set this value
	return c.this()
}

// SetTextIsHtml to true to turn off html escaping of the text output.
func (c *ControlBase) SetTextIsHtml(h bool) ControlI {
	c.textIsHtml = h
	return c.this()
}

// TextIsLabel is used by the drawing routines to determine if the control's text should be wrapped with a label tag.
// This is normally used by checkboxes and radio buttons that use the label tag in a special way.
func (c *ControlBase) TextIsLabel() bool {
	return false
}

// Text returns the text of the control.
func (c *ControlBase) Text() string {
	return c.text
}

// SetText sets the text of the control. Not all controls use this value.
func (c *ControlBase) SetText(t string) ControlI {
	if t != c.text {
		c.text = t
		c.Refresh()
	}
	return c.this()
}

func (c *ControlBase) Attributes() html5tag.Attributes {
	//TODO implement me
	panic("implement me")
}

// DrawPreRender prepares the control for drawing.
// If you override it, be sure to also call this function as well.
func (c *ControlBase) DrawPreRender(ctx context.Context, w io.Writer) {
	if c.wasRendered || c.isRendering {
		panic(fmt.Sprintf("Control %s has already been drawn.", c.ID()))
	}

	// Because we may be rerendering a parent control, we need to make sure all "child" controls are marked as NOT being on the form
	// before rendering it again.
	for child := range c.ChildControls() {
		child.markOnPage(false)
	}

	// Finally, let's specify that we have begun rendering this control
	c.isRendering = true
}

func (c *ControlBase) Draw(ctx context.Context, w io.Writer) {
	c.this().DrawPreRender(ctx, w)

	if !config.Minify && GetRequest(ctx).RequestMode() != RequestModeAjax {
		if _, err := fmt.Fprintf(w, "<!-- ControlBase Type:%s, Id:%s -->\n", c.TypeOf(), c.ID()); err != nil {
			panic(err)
		}
	}

	if c.isHidden {
		// We are invisible, but not using a wrapper. This creates a problem, in that when we go visible, we do not know what to replace
		// To fix this, we create an empty, invisible control in the place where we would normally draw
		if _, err := fmt.Fprint(w, `<span id="`, c.this().ID(), `" style="display:none;" data-grctl></span>`, "\n"); err != nil {
			panic(err)
		}
	} else {
		c.this().DrawTag(ctx, w)
	}

	response := c.form.Response()
	c.this().PutCustomScript(ctx, response)
	c.GetActionScripts(response)
	c.this().DrawPostRender(ctx, w)
	return
}

// DrawAjax is responsible for rendering the control during an ajax call.
//
// Some objects automatically render their child objects, and some don't,
// so we detect whether the parent is being rendered, and assume the parent is taking care of rendering for
// us if so.
//
// Override if you want more control over ajax drawing, like if you detect parts of your control that have changed
// and then want to draw only those parts. This will get called on every control on every ajax draw request.
// It is up to you to test the blnRendered flag of the control to know whether the control was already rendered
// by a parent control before drawing here.
func (c *ControlBase) DrawAjax(ctx context.Context, response *Response) {

	if c.this().NeedsRefresh() {
		// simply re-render the control and assume rendering will handle rendering its children

		func() {
			// wrap in a function to get deferred PutBuffer to execute immediately after drawing
			buf := pool.GetBuffer()
			c.this().Draw(ctx, buf)
			response.SetControlHtml(c.ID(), buf.String())
			pool.PutBuffer(buf) // If this doesn't happen because of a panic, that is fine
		}()
	} else {
		// add attribute changes
		if c.attributeScripts != nil {
			for _, entry := range c.attributeScripts {
				response.ExecuteControlCommand(entry.id, entry.f, entry.commands...)
			}
			c.attributeScripts = nil
		}

		// ask the child controls to potentially render, since this control doesn't need to
		for child := range c.ChildControls() {
			if child.IsOnPage() || child.shouldAutoRenderValue() {
				child.DrawAjax(ctx, response)
			}
		}
	}
	return
}

// DrawPostRender is called by the framework at the end of drawing, and is the place where controls
// do any post-drawing cleanup needed.
func (c *ControlBase) DrawPostRender(ctx context.Context, w io.Writer) {
	// Update watcher
	//if ($This->objWatcher) {
	//$This->objWatcher->makeCurrent();
	//}

	c.isRendering = false
	c.wasRendered = true
	c.isOnPage = true
	c.needsRefresh = false
	c.attributeScripts = nil // Entire control was redrawn, so don't need these
	return
}

// DrawTag is responsible for drawing the ControlBase's tag itself.
// ControlBase implementations can override this to draw the tag in a different way, or draw more than one tag if
// drawing a compound control.
func (c *ControlBase) DrawTag(ctx context.Context, w io.Writer) {
	var ctrl string

	log.Debug(ctx, logModule, "Drawing tag: "+c.ID())

	attributes := c.this().DrawingAttributes(ctx)

	if c.IsVoidTag {
		ctrl = html5tag.RenderVoidTag(c.Tag, attributes)
	} else {
		buf := pool.GetBuffer()
		c.this().DrawInnerHtml(ctx, buf)
		c.RenderAutoControls(ctx, buf)
		if c.Tag == "" {
			ctrl = buf.String() // a wrapper with no tag. Just inserts functionality and draws its children.
		} else if c.hasNoSpace {
			ctrl = html5tag.RenderTagNoSpace(c.Tag, attributes, buf.String())

		} else {
			ctrl = html5tag.RenderTag(c.Tag, attributes, buf.String())
		}
		defer pool.PutBuffer(buf)
	}
	if _, err := io.WriteString(w, ctrl); err != nil {
		panic(err)
	}
}

// RenderAutoControls is an internal function to draw controls marked to autoRender. These are generally used for hidden controls
// that can be shown without impacting layout, or that are scripts only. ControlBase implementations that need to
// put these controls in particular locations on the form can override this.
func (c *ControlBase) RenderAutoControls(ctx context.Context, w io.Writer) {
	// Figuring out where to draw these controls can be difficult.

	for ctrl := range c.ChildControls() {
		if ctrl.shouldAutoRenderValue() &&
			!ctrl.WasRendered() {

			ctrl.Draw(ctx, w)
		}
	}
	return
}

// DrawTemplate is used by the framework to draw the Control using a template.
// Controls that use templates should use this function signature for the template. That will override this one, and
// we will then detect that the template was drawn. Otherwise, we detect that no template was defined, and it will move
// on to drawing the controls without a template, or just the text if text is defined.
func (c *ControlBase) DrawTemplate(ctx context.Context, w io.Writer) (err error) {
	// Don't change this to use some kind of function injection, as such things are not serializable
	return NoTemplateError()
}

// DrawInnerHtml is used by the framework to draw just the inner html of the control, if the control is not a self
// terminating (void) control. Sub-controls can override this.
func (c *ControlBase) DrawInnerHtml(ctx context.Context, w io.Writer) {
	if err := c.this().DrawTemplate(ctx, w); err == nil {
		return
	} else if IsNoTemplateError(err) {
		// There is no template, so do the default of drawing child controls and then
		// text.
		if c.HasChildControls() {
			c.this().DrawChildControls(ctx, w)
			return
		}

		c.this().DrawText(ctx, w)
	} else {
		panic(err)
	}

	return
}

// DrawChildControls renders the child controls that have not yet been drawn into the buffer.
func (c *ControlBase) DrawChildControls(ctx context.Context, w io.Writer) {
	for child := range c.ChildControls() {
		if !child.WasRendered() {
			child.Draw(ctx, w)
		}
	}
	return
}

// DrawText renders the text of the control, escaping if needed.
func (c *ControlBase) DrawText(ctx context.Context, w io.Writer) {
	if c.text != "" {
		text := c.text

		if !c.textIsHtml {
			text = html.EscapeString(text)
		}
		if _, err := io.WriteString(w, text); err != nil {
			panic(err)
		}
	}
	return
}

func (c *ControlBase) shouldAutoRenderValue() bool { return c.shouldAutoRender }

func (c *ControlBase) resetDrawingFlags() {
	c.wasRendered = false
	c.needsRefresh = false
}

// SetTag sets the tag name displayed in the html object.
func (c *ControlBase) SetTag(tag string) ControlI {
	c.Tag = tag
	return c
}

// SetAttribute sets an HTML attribute of the control. You can manually set almost any attribute, but be careful
// not to set the id attribute, or any attribute that is managed by the control itself. If you are setting
// a data-* attribute, use [SetDataAttribute] instead. If you are adding a class to the control, use [AddClass].
func (c *ControlBase) SetAttribute(name string, val any) ControlI {
	if name == "id" {
		panic("You can only set the 'id' attribute of a control when it is created")
	}

	changed, err := c.attributes.SetChanged(name, html5tag.ValueString(val))
	if err != nil {
		panic(err)
	}

	if changed {
		// The val passed in might be a calculation, so we need to get the ultimate new value
		v2 := c.attributes.Get(name)
		// We are recording here that the attribute intends to change. If we are responding to an ajax
		// request, we will send back a command to only change the attribute on the control if the
		// control does not get completely redrawn. If the control is completely redrawn, the new
		// attribute will automatically be drawn, so there would be no need to also send an attribute change command.
		c.AddRenderScript("attr", name, v2)
	}
	return c.this()
}

// Attribute returns the value of a custom attribute. Note that this will not return values that are set only during
// drawing and that are managed by the ControlBase implementation.
func (c *ControlBase) Attribute(name string) string {
	return c.attributes.Get(name)
}

// HasAttribute returns true if the control has the indicated custom attribute defined.
func (c *ControlBase) HasAttribute(name string) bool {
	return c.attributes.Has(name)
}

// DrawingAttributes is called by the framework just before drawing a control, and should
// return a set of attributes that should override those set by the user. This allows controls to set attributes
// that should take precedence over other attributes, and that are critical to drawing the
// tag of the control. This function is designed to only be called by ControlBase implementations.
func (c *ControlBase) DrawingAttributes(ctx context.Context) html5tag.Attributes {
	a := c.attributes.Copy()
	a.SetID(c.id)          // make sure the control id is set at a minimum
	a.SetData("grctl", "") // make sure control is registered. Overriding controls can put a control name here.

	if c.isRequired {
		a.Set("aria-required", "true")
	}

	channels := stringmap.JoinStrings(c.watchedKeys, "=", ";")

	if channels != "" {
		a.SetData("grWatch", channels)
	}

	return a
}

// SetDataAttribute will set a data-* attribute. The name should be camelCase, without "data" in the name.
// For example:
//
//	SetDataAttribute("myVal", 5)
//
// will result in the "data-my-val=5" attribute appearing in the HTML, which will be accessible from
// javascript using .data("myVal").
func (c *ControlBase) SetDataAttribute(name string, val any) {
	var v string
	var ok bool

	if v, ok = val.(string); !ok {
		v = fmt.Sprint(v)
	}

	changed, err := c.attributes.SetDataChanged(name, v)
	if err != nil {
		panic(err)
	}

	if changed {
		c.AddRenderScript("data", name, v) // Use the data method to set the data during ajax requests
	}
}

// MergeAttributes will merge the given attributes into the control's attributes.
func (c *ControlBase) MergeAttributes(a html5tag.Attributes) ControlI {
	c.attributes.Merge(a)
	return c.this()
}

// ProcessAttributeString is used by the drawing template to let you set attributes in the draw tag.
// Attributes are of the form `name="value"`.
func (c *ControlBase) ProcessAttributeString(s string) ControlI {
	if s != "" {
		c.attributes.MergeString(s)
	}
	return c.this()
}

// AddClass will add a class or classes to the control. If adding multiple classes at once, separate them with
// a space.
func (c *ControlBase) AddClass(class string) ControlI {
	if changed := c.attributes.AddClassChanged(class); changed {
		// Note here. We cannot just draw the class, because DrawingAttributes might return
		// a class, and DrawingAttributes requires a context. So we coordinate with goradd.js
		// to be able to add and remove a class.
		c.AddRenderScript("class", "+"+class)
	}
	return c.this()
}

// RemoveClass will remove the named class from the control.
func (c *ControlBase) RemoveClass(class string) ControlI {
	if changed := c.attributes.RemoveClass(class); changed {
		c.AddRenderScript("class", "-"+class)
	}
	return c.this()
}

// HasClass returns true if the class has been assigned to the control from the GO side. We do not currently detect
// class changes done in javascript.
func (c *ControlBase) HasClass(class string) bool {
	return c.attributes.HasClass(class)
}

// NeedsRefresh returns true if the control needs to be completely redrawn. Generally you control
// this by calling Refresh(), but subclasses can implement other ways of detecting this.
func (c *ControlBase) NeedsRefresh() bool {
	return c.needsRefresh
}

// PutCustomScript is called by the framework to ask the control to inject any javascript it needs into the form.
// In particular, this is the place where Controls add javascript that transforms the html into a custom javascript control.
// A ControlBase implementation does this by calling functions on the response object.
func (c *ControlBase) PutCustomScript(ctx context.Context, response *Response) {
}

// GetActionScripts is an internal function called during drawing to gather all the event related
// scripts attached to the control and send them to the response.
func (c *ControlBase) GetActionScripts(r *Response) {
	// Render actions
	if c.events != nil {
		for id, e := range c.events {
			s := event.RenderActions(e, c.this(), id)
			r.ExecuteJavaScript(s, PriorityStandard)
		}
	}
}

// WrapEvent is an internal function to allow the control to customize its treatment of event processing.
func (c *ControlBase) WrapEvent(eventName string, selector string, eventJs string, options map[string]interface{}) string {
	if selector != "" {
		return fmt.Sprintf("g$('%s').on('%s', '%s', function(event, eventData){%s}, %s);", c.ID(), eventName, selector, eventJs, javascript.ToJavaScript(options))
	} else {
		return fmt.Sprintf("g$('%s').on('%s', function(event, eventData){%s}, %s);", c.ID(), eventName, eventJs, javascript.ToJavaScript(options))
	}
}

func (c *ControlBase) doAction(ctx context.Context) {
	var e *event.Event
	var ok bool
	var isPrivate bool

	request := GetRequest(ctx)

	if e, ok = c.events[request.EventID]; ok {
		isPrivate = event.IsPrivate(e)
	}

	if !ok {
		// This is the situation where we are submitting a form using a button in a browser
		// where javascript has been turned off. We assume we only have a click event on the button
		// and so just grab it.
		var id event.EventID
		for id, e = range c.events {
			break
		}
		if id == 0 {
			return
		}
	}

	if (event.GetValidationOverride(e) != event.ValidateNone && event.GetValidationOverride(e) != event.ValidateDefault) ||
		(event.GetValidationOverride(e) == event.ValidateDefault && c.this().ValidationType(e) != event.ValidateNone) {
		c.form.ResetValidation()
	}

	if c.passesValidation(ctx, e) {
		log.Debug(ctx, logModule, "doAction - triggered event", "event", e.String())
		if callbackAction := event.GetCallbackAction(e); callbackAction != nil {
			cba := callbackAction.(action.CallbackActionAccessor)
			p := action.NewActionParams(
				event.Name(e),
				cba.GetActionID(),
				callbackAction,
				c.ID(),
				request.ActionValues,
			)

			controlId := cba.GetDestinationControlID()
			if controlId == action.DefaultControlId {
				controlId = c.ID()
			}
			if dest := c.form.GetControl(controlId); dest != nil {
				if isPrivate {
					if log.IsDebugging() {
						log.Debug(ctx, logModule, "doAction - DoPrivateAction",
							"dest_id", dest.ID(),
							"action_id", p.ID,
							"action_type", reflect.TypeOf(p.Action).String(),
							"trigger_id", p.ControlId)
					}
					dest.DoPrivateAction(ctx, p)
				} else {
					if log.IsDebugging() {
						log.Debug(ctx, logModule, "doAction - DoAction",
							"dest_id", dest.ID(),
							"action_id", p.ID,
							"action_type", reflect.TypeOf(p.Action).String(),
							"trigger_id", p.ControlId)
					}
					dest.DoAction(ctx, p)
				}
			}
		}
	} else {
		log.Debug(ctx, logModule, "doAction - failed validation",
			"error", e.String())
	}
}

// SetActionValue sets a value that is provided to actions when they are triggered. The value can be a static value
// or one of the javascript.* objects that can dynamically generate values. The value is then sent back to the Action
// function after the action is triggered as the ControlActionValue in the action.Params struct.
func (c *ControlBase) SetActionValue(v any) ControlI {
	c.actionValue = v
	return c.this()
}

// ActionValue returns the control's action value that is sent to the Action function in the ControlActionValue of the
// action.Params struct.
func (c *ControlBase) ActionValue() any {
	return c.actionValue
}

// DoAction handles actions that are triggered by events on controls.
//
// Forms and controls should implement this method to handle an action.
// Typically, the DoAction function will first look at the ID, or Event to know how to handle it.
//
// When a control does not handle an action, it should pass it to its "superclass" object. If none
// of the superclasses of the object handle it, the action will be passed to its parent control in the form
// object hierarchy.
func (c *ControlBase) DoAction(ctx context.Context, a action.Params) {
	if a.ID == action.RefreshActionId {
		c.Refresh()
		return
	}
	if p := c.ParentControl(); p != nil { // a form does not have a parent
		p.DoAction(ctx, a)
	}
}

// DoPrivateAction processes actions that a control sets up for itself, and that it does not want to give the opportunity
// for users of the control to manipulate or remove those actions. Generally, private actions should call their embedded
// DoPrivateAction method too.
func (c *ControlBase) DoPrivateAction(ctx context.Context, a action.Params) {
}

// Specifying an action is deprecated. Instead, call Action on the event.
func (c *ControlBase) On(e *event.Event, a ...action.ActionI) ControlI {
	c.Refresh() // completely redraw the control. The act of redrawing will turn off old scripts.
	// TODO: Adding scripts should instead just redraw the associated script block. We will need to
	// implement a script block with every control connected by id
	c.eventCounter++

	// Get a new event id
	for {
		if _, ok := c.events[c.eventCounter]; ok {
			c.eventCounter++
		} else {
			break
		}
	}

	if c.events == nil {
		c.events = map[event.EventID]*event.Event{}
	}
	c.events[c.eventCounter] = e

	event.SetEventID(e, c.eventCounter)

	if len(a) > 1 {
		e.Action(action.Group(a...))
	} else if len(a) == 1 {
		e.Action(a[0])
	}

	return c.this()
}

// Off removes all event handlers from the control, or the events with the specified ids.
func (c *ControlBase) Off(ids ...event.EventID) {
	if ids == nil {
		for id, e := range c.events {
			if !event.IsPrivate(e) {
				delete(c.events, id)
			}
		}
	} else {
		for _, id := range ids {
			if !event.IsPrivate(c.events[id]) {
				delete(c.events, id)
			}
		}
	}
}

// PrivateOff removes all private event handlers from the control, or the events with the specified id.
// This is intended to only be used by control implementations. Do not
// call this normally.
func (c *ControlBase) PrivateOff(ids ...event.EventID) {
	if ids == nil {
		for id, e := range c.events {
			if event.IsPrivate(e) {
				delete(c.events, id)
			}
		}
	} else {
		for _, id := range ids {
			if event.IsPrivate(c.events[id]) {
				delete(c.events, id)
			}
		}
	}
}

// Event returns the event associated with the eventName, which corresponds to the javascript
// trigger name. If there are multiple events with the same event name, the first event will
// be returned. (Multiple events with the same name could be listed if some events have a delay.)
func (c *ControlBase) Event(eventName string) *event.Event {
	for _, e := range c.events {
		if event.Name(e) == eventName {
			return e
		}
	}
	return nil
}

// Refresh will force the control to be completely redrawn on the next update.
func (c *ControlBase) Refresh() {
	c.needsRefresh = true
}

// IsVisible returns whether the control will be drawn.
func (c *ControlBase) IsVisible() bool {
	return !c.isHidden
}

// IsDisplayed returns true if the control will be displayed.
func (c *ControlBase) IsDisplayed() bool {
	return c.attributes.IsDisplayed()
}

// SetVisible controls whether the ControlBase will be drawn. Controls that are not visible are not rendered in
// html, but rather a hidden stub is rendered as a placeholder in case the control is made visible again.
func (c *ControlBase) SetVisible(v bool) {
	if c.isHidden == v { // these are opposite in meaning
		c.isHidden = !v
		c.Refresh()
	}
}

// SetDisplay sets the "display" property of the style attribute of the html control to the given value.
// Also consider using SetVisible. If you use SetDisplay to hide a control, the control will still be
// rendered in html, but the browser will not show it.
func (c *ControlBase) SetDisplay(d string) ControlI {
	c.attributes.SetDisplay(d)
	c.Refresh()
	return c.this()
}

// SetDisabled will set the "disabled" attribute of the control.
func (c *ControlBase) SetDisabled(d bool) {
	c.attributes.SetDisabled(d)
	c.Refresh()
}

// IsDisabled returns true if the disabled attribute is true.
func (c *ControlBase) IsDisabled() bool {
	return c.attributes.IsDisabled()
}

func (c *ControlBase) markOnPage(v bool) {
	c.isOnPage = v
}

// IsOnPage returns true if the control has been rendered on the page.
func (c *ControlBase) IsOnPage() bool {
	return c.isOnPage
}

// IsRendering returns true if we are in the process of rendering the control.
func (c *ControlBase) IsRendering() bool {
	return c.isRendering
}

// SetHasNoSpace tells the control to draw its inner html with no space around it.
// This should generally only be called by control implementations. If this is not set, spaces
// might be added to make the HTML more readable, which can affect some html control types.
func (c *ControlBase) SetHasNoSpace(v bool) ControlI {
	c.hasNoSpace = v
	return c
}

// by the form automatically, after all other controls are drawn, if the control was not drawn in
// some other way. An example of an auto-rendered control would be a dialog box that starts out hidden,
// but then is shown by some user response. Such controls are normally shown by javascript, and are
// absolutely positioned so that they do not affect the layout of the rest of the form.
func (c *ControlBase) SetShouldAutoRender(r bool) {
	c.shouldAutoRender = r
}

// UpdateFormValues is called by the framework to cause the control to retrieve its values from the form.
// Control implementations should implement this function and then look in the context ctx
// to retrieve its values.
func (c *ControlBase) UpdateFormValues(request *RequestContext) {
}

// SetValidationType specifies how this control validates other controls. Typically, its either ValidateNone or ValidateForm.
//
//   - ValidateForm will validate all the controls on the form.
//   - ValidateSiblingsAndChildren will validate the immediate siblings of the target controls and their children
//   - ValidateSiblingsOnly will validate only the siblings of the target controls
//   - ValidateTargetsOnly will validate only the specified target controls
func (c *ControlBase) SetValidationType(typ event.ValidationType) ControlI {
	c.validationType = typ
	return c.this()
}

// ValidationType is called by the framework to return the validation type.
//
// Control implementations can override this function.
func (c *ControlBase) ValidationType(e *event.Event) event.ValidationType {
	if c.validationType == event.ValidateNone || c.validationType == event.ValidateDefault {
		return event.ValidateNone
	} else {
		return c.validationType
	}
}

func (c *ControlBase) validationTypeValue() event.ValidationType {
	return c.validationType
}

func (c *ControlBase) blockParentValidationValue() bool {
	return c.blockParentValidation
}

// SetValidationTargets specifies which controls to validate, in conjunction with the ValidationType setting,
// giving you very fine-grained control over validation. The default
// is to use just this control as the target.
func (c *ControlBase) SetValidationTargets(controlIDs ...string) {
	c.validationTargets = controlIDs
}

// passesValidation checks to see if the event requires validation, and if so, if it passes the required validation
func (c *ControlBase) passesValidation(ctx context.Context, e *event.Event) (valid bool) {
	validation := c.this().ValidationType(e)

	if v := event.GetValidationOverride(e); v != event.ValidateDefault {
		validation = v
	}

	if validation == event.ValidateDefault || validation == event.ValidateNone {
		return true
	}

	var targets []ControlI

	if c.validationTargets == nil {
		if c.validationType == event.ValidateForm {
			targets = []ControlI{c.form}
		} else if c.validationType == event.ValidateContainer {
			for target := c.ParentControl(); target != nil; target = target.ParentControl() {
				switch target.validationTypeValue() {
				case event.ValidateChildrenOnly:
					fallthrough
				case event.ValidateSiblingsAndChildren:
					fallthrough
				case event.ValidateSiblingsOnly:
					fallthrough
				case event.ValidateTargetsOnly:
					validation = target.validationTypeValue()
					targets = []ControlI{target}
				default:
					// do nothing
				}
			}
			if targets == nil {
				// Target is the form
				targets = []ControlI{c.form}
				validation = event.ValidateForm
			}
		} else {
			targets = []ControlI{c}
		}
	} else {
		if c.validationType == event.ValidateForm ||
			c.validationType == event.ValidateContainer {
			panic("Unsupported validation type and target combo.")
		}
		for _, id := range c.validationTargets {
			if ctrl := c.form.GetControl(id); ctrl != nil {
				targets = append(targets, ctrl)
			}
		}
	}

	valid = true

	switch validation {
	case event.ValidateForm:
		valid = c.form.validateSelfAndChildren(ctx)
	case event.ValidateSiblingsAndChildren:
		for _, t := range targets {
			valid = t.validateSiblingsAndChildren(ctx) && valid
		}
	case event.ValidateSiblingsOnly:
		for _, t := range targets {
			valid = t.validateSelfAndSiblings(ctx) && valid
		}
	case event.ValidateChildrenOnly:
		for _, t := range targets {
			valid = t.validateSelfAndChildren(ctx) && valid
		}

	case event.ValidateTargetsOnly:
		var valid bool = true
		for _, t := range targets {
			valid = t.Validate(ctx) && valid
		}
	default:
		// do nothing
	}
	return valid
}

// Validate is called by the framework to validate a control, but not the control's children.
// It is designed to be overridden by ControlBase implementations.
// Overriding controls should call the parent version before doing their own validation.
func (c *ControlBase) Validate(ctx context.Context) bool {
	if c.validationState != ValidationNever {

		if c.validationMessage != c.ValidMessage {
			c.validationMessage = c.ValidMessage
		}
		if c.validationState != ValidationValid {
			c.validationState = ValidationValid
		}
	}
	return true
}

// validateSelfAndSiblings will validate self and siblings, but no children
func (c *ControlBase) validateSelfAndSiblings(ctx context.Context) bool {

	if c.parentId == "" {
		// the one and only form
		return true
	}

	var valid = true
	for sibling := range c.ParentControl().ChildControls() {
		if sibling.IsOnPage() {
			valid = sibling.Validate(ctx) && valid
		}
	}
	return valid
}

func (c *ControlBase) validateSelfAndChildren(ctx context.Context) bool {
	if !c.IsOnPage() {
		return true
	}

	var isValid = true
	for child := range c.ChildControls() {
		if !child.blockParentValidationValue() && child.IsOnPage() {
			isValid = child.validateSelfAndChildren(ctx) && isValid
		}
	}
	// validate self after validating all children, because self might want to invalidate child items
	// also make sure we validate the parent even if the children are invalid in case the parent is looking at the validation state of the children
	isValid = c.this().Validate(ctx) && isValid

	return isValid
}

// validateSiblingsAndChildren validates self, siblings of self, and all children of those.
func (c *ControlBase) validateSiblingsAndChildren(ctx context.Context) bool {
	if c.parentId == "" {
		return true
	}

	var isValid = true
	for sibling := range c.ParentControl().ChildControls() {
		if !sibling.IsOnPage() {
			continue
		}
		isValid = sibling.validateSelfAndChildren(ctx) && isValid
	}

	return isValid
}

// ValidationMessage is the currently set validation message that will print with the control. Normally this only
// gets set when a validation error occurs.
func (c *ControlBase) ValidationMessage() string {
	return c.validationMessage
}

// SetValidationError sets the validation error to the given string. It will also handle setting the wrapper class
// to indicate an error. Override if you have a different way of handling errors.
func (c *ControlBase) SetValidationError(e string) {
	if c.validationMessage != e {
		c.validationMessage = e

		if e == "" {
			c.validationState = ValidationWaiting
			c.AddRenderScript("removeAttr", "aria-invalid")
		} else {
			c.validationState = ValidationInvalid
			c.AddRenderScript("attr", "aria-invalid", "true")
		}
		if c.ParentControl() != nil {
			c.ParentControl().ChildValidationChanged() // notify parent wrappers
		}
	}
}

func (c *ControlBase) ResetValidation() {
	var changed bool
	for ctrl := range c.SelfAndAllChildren() {
		changed = ctrl.resetValidationValues() || changed
	}
	if changed {
		if p := c.ParentControl(); p != nil {
			p.ChildValidationChanged()
		}
	}
}

func (c *ControlBase) resetValidationValues() (changed bool) {
	if c.validationMessage != "" {
		c.validationMessage = ""
		changed = true
	}
	if c.validationState != ValidationWaiting {
		c.validationState = ValidationWaiting
		changed = true
	}
	return
}

// ChildValidationChanged is sent by the framework when a child control's validation message
// has changed. ParentControl controls can use this to change messages or attributes in response.
func (c *ControlBase) ChildValidationChanged() {
	if c.ParentControl() != nil {
		c.ParentControl().ChildValidationChanged()
	}
}

// ValidationState returns the current ValidationState value.
func (c *ControlBase) ValidationState() ValidationState {
	return c.validationState
}

// SelfAndAllChildren returns an iterator that yields the current control first,
// then all of its child Controls, and recursively all of those child controls for
// each control that has children.
func (c *ControlBase) SelfAndAllChildren() iter.Seq[ControlI] {
	return func(yield func(ControlI) bool) {
		// Yield self first
		if !yield(c) {
			return
		}
		for child := range c.AllChildControls() {
			if !yield(child) {
				return
			}
		}
	}
}

// AddRenderScript adds a javascript command to be executed on the next ajax draw.
// These commands allow javascript to change an aspect of the control without
// having to redraw the entire control. This should be used by ControlBase implementations only.
func (c *ControlBase) AddRenderScript(f string, params ...any) {
	c.attributeScripts = append(c.attributeScripts, attributeScriptEntry{id: c.ID(), f: f, commands: params})
}

// AddRelatedRenderScript adds a render script for a related html object. This is primarily used by control implementations.
func (c *ControlBase) AddRelatedRenderScript(id string, f string, params ...any) {
	c.attributeScripts = append(c.attributeScripts, attributeScriptEntry{id: id, f: f, commands: params})
}

// WasRendered returns true if the control has been rendered.
func (c *ControlBase) WasRendered() bool {
	return c.wasRendered
}

// T sends strings to the translator for translation, and returns the translated string.
// The language is taken from the session. See the i18n package for more info on that mechanism.
// Additionally, you can add an i18n.ID() call to add an id to the translation to disambiguate it from similar strings, and
// you can add an i18n.Comment() call to add an extracted comment for the translators.
// The message string should be a literal string and not a variable,
// so that an extractor can extract it from your source to put it into a translation file.
//
// Examples:
//
//	textbox.T("I have %d things", count, i18n.Comment("This will need multiple translations based on the count value"));
//	textbox.SetText(textbox.T("S", i18n.ID("SouthInitialism")));
func (c *ControlBase) T(message string, params ...interface{}) string {
	p := append([]interface{}{i18n.Language(c.form.LanguageTag())}, params...)
	return i18n.Translate(message, p...)
}

// SaveState sets whether the control should save its value and other state information so that if the form is redrawn,
// the value can be restored.
//
// Call this during control initialization to cause the control to remember what it
// is set to, so that if the user returns to the page, it will keep its value.
// This function is also responsible for restoring the previously saved state of the control,
// so call this only after you have set the default state of a control during creation or initialization.
func (c *ControlBase) SaveState(ctx context.Context, saveIt bool) {
	c.shouldSaveState = saveIt
	c.readState(ctx)
}

// This state is used by controls to restore the visual state of the control if the page is returned to. This is helpful
// in situations where a control is used to filter what is shown on the page, you zoom into an item, and then return to
// the parent control. In this situation, you want to see things in the same state they were in, and not have to set up
// the filter all over again.
func (c *ControlBase) writeState(ctx context.Context) {
	var stateStore *stateStoreType
	var state *stateType
	var ok bool

	if c.shouldSaveState {
		state = new(stateType)
		c.this().MarshalState(state)
		stateKey := c.Form().ID() + ":" + c.ID()
		if state.Len() > 0 {
			state.Set(sessionControlTypeState, c.TypeOf().String()) // so we can make sure the type is the same when we read, in situations where control Ids are dynamic
			i := session.Get(ctx, sessionControlStates)
			if i == nil {
				stateStore = new(stateStoreType)
				session.Set(ctx, sessionControlStates, stateStore)
			} else if _, ok = i.(*stateStoreType); !ok {
				stateStore = new(stateStoreType)
				session.Set(ctx, sessionControlStates, stateStore)
			} else {
				stateStore = i.(*stateStoreType)
			}
			stateStore.Set(stateKey, state)
		}
	}
}

// readState is an internal function that will read the state of itself
func (c *ControlBase) readState(ctx context.Context) {
	var stateStore *stateStoreType
	var state *stateType
	var ok bool

	if c.shouldSaveState {
		if i := session.Get(ctx, sessionControlStates); i != nil {
			if stateStore, ok = i.(*stateStoreType); !ok {
				return
				// Indicates the entire control state store changed types, so completely ignore it
			}

			key := c.Form().ID() + ":" + c.ID()
			i2 := stateStore.Get(key)
			if state, ok = i2.(*stateType); !ok {
				return
				// Indicates This particular item was not stored correctly
			}

			if typ := state.Get(sessionControlTypeState).(string); typ != c.TypeOf().String() {
				return // types are not equal, ids must have changed
			}

			c.this().UnmarshalState(state)
		}
	}
}

// MarshalState is a helper function for control implementations to save their state,
// so that if the form is reloaded, the value that the user entered will not be lost.
// Implementing controls should add items to the given map.
// Note that the control id is used as a key for the state,
// so that if you are dynamically adding controls, you should make sure you give a specific, non-changing control id
// to the control, or the state may be lost.
func (c *ControlBase) MarshalState(m SavedState) {
}

// UnmarshalState is a helper function for control implementations to retrieve their state from state storage.
// To implement it, a control should read the data out of the given map.
// If needed, implement your own version checking scheme.
// The given map will be guaranteed to have been written out by the same kind of control as the one reading it.
// Control implementations should be sure to call the superclass version too.
func (c *ControlBase) UnmarshalState(m SavedState) {
}

// Serialize encodes the control for the pagecache serializer.
// Control implementations should call this before their own serialization process.
func (c *ControlBase) Serialize(e Encoder) {
	c.treeNode.serialize(e)
	if err := e.Encode(c.Tag); err != nil {
		panic(err)
	}
	if err := e.Encode(c.IsVoidTag); err != nil {
		panic(err)
	}
	if err := e.Encode(c.hasNoSpace); err != nil {
		panic(err)
	}
	if err := e.Encode(c.attributes); err != nil {
		panic(err)
	}
	if err := e.Encode(c.text); err != nil {
		panic(err)
	}
	if err := e.Encode(c.textLabelMode); err != nil {
		panic(err)
	}
	if err := e.Encode(c.textIsHtml); err != nil {
		panic(err)
	}
	if err := e.Encode(c.isRequired); err != nil {
		panic(err)
	}
	if err := e.Encode(c.isHidden); err != nil {
		panic(err)
	}
	if err := e.Encode(c.isOnPage); err != nil {
		panic(err)
	}
	if err := e.Encode(c.shouldAutoRender); err != nil {
		panic(err)
	}
	if err := e.Encode(c.isBlock); err != nil {
		panic(err)
	}
	if err := e.Encode(c.ErrorForRequired); err != nil {
		panic(err)
	}
	if err := e.Encode(c.ValidMessage); err != nil {
		panic(err)
	}
	if err := e.Encode(c.validationMessage); err != nil {
		panic(err)
	}
	if err := e.Encode(c.validationState); err != nil {
		panic(err)
	}
	if err := e.Encode(c.validationType); err != nil {
		panic(err)
	}
	if err := e.Encode(c.validationTargets); err != nil {
		panic(err)
	}
	if err := e.Encode(c.blockParentValidation); err != nil {
		panic(err)
	}
	if err := e.Encode(c.events); err != nil {
		panic(err)
	}
	if err := e.Encode(c.eventCounter); err != nil {
		panic(err)
	}
	if err := e.Encode(c.shouldSaveState); err != nil {
		panic(err)
	}
	if err := e.Encode(c.needsRefresh); err != nil { // not sure. Might always be false.
		panic(err)
	}
	if err := e.Encode(c.watchedKeys); err != nil {
		panic(err)
	}
}

// Deserialize is called by GobDecode to deserialize the control.  It is overridable, and control implementations
// should call this first before calling their own version. However, after deserialization, the control will
// not be ready for use, since its parent, form or child controls still need to be deserialized.
// The Decoded function should be called to fix up the necessary internal pointers.
func (c *ControlBase) Deserialize(d Decoder) {
	c.treeNode.deserialize(d)
	if err := d.Decode(&c.Tag); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.IsVoidTag); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.hasNoSpace); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.attributes); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.text); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.textLabelMode); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.textIsHtml); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.isRequired); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.isHidden); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.isOnPage); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.shouldAutoRender); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.isBlock); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.ErrorForRequired); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.ValidMessage); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.validationMessage); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.validationState); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.validationType); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.validationTargets); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.blockParentValidation); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.events); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.eventCounter); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.shouldSaveState); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.needsRefresh); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.watchedKeys); err != nil {
		panic(err)
	}
}

// Deserialized is called after a control has been restored from serialization and
// the internal form pointer has been restored. Use this as an opportunity to recreate
// any internal pointers a control needs to do its work.
func (c *ControlBase) Deserialized() {}

// EventList is used by Creators to declare a list of events.
type EventList []*event.Event

// DataAttributeMap is used by Creators to declare a map of data attributes.
type DataAttributeMap map[string]interface{}

// Nodes is used by Creators to specify a list of nodes.
func Nodes(n ...query.NodeI) []query.NodeI {
	return n
}

// ControlOptions are options common to all controls
type ControlOptions struct {
	// Attributes will set the attributes of the control. Use DataAttributes to set data attributes, Styles to set styles, and Class to set the class
	Attributes html5tag.Attributes
	// DataAttributes will set the data-* attributes of the control.
	DataAttributes DataAttributeMap
	// Styles sets the styles of the control's tag
	Styles html5tag.Style
	// Class sets the class of the control's tag. Prefix a class with "+" to add a class, or "-" to remove a class.
	Class string
	// IsDisabled initializes the control in the disabled state, with a "disabled" attribute
	IsDisabled bool
	// IsRequired is used by the validator. If a value is required, and the control is empty, it will not pass validation.
	IsRequired bool
	// IsHidden initializes this control as hidden. A placeholder will be sent in the html so that when the control is shown through ajax, we will know where to put it.
	IsHidden bool
	// On adds events with actions to the control
	On EventList
	// DataConnector is the ViewModel layer that moves data between the control and an attached model.
	//DataConnector DataConnector
	// WatchedDbTables lets you specify database nodes to watch for changes. When a record in the table is altered, added or deleted,
	// this control will automatically redraw. To watch a specific record, call WatchDbRecord when you load the control's data.
	//WatchedDbTables []query.NodeI
}

// ApplyOptions is called by Creators to apply the default control options.
func (c *ControlBase) ApplyOptions(ctx context.Context, o ControlOptions) {
	if o.Attributes != nil {
		c.MergeAttributes(o.Attributes)
	}
	for k, v := range o.DataAttributes {
		c.SetDataAttribute(k, v)
	}
	for k, v := range o.Styles {
		c.SetStyle(k, v)
	}
	for _, a := range o.On {
		c.On(a)
	}
	if o.Class != "" {
		c.attributes.AddClass(o.Class) // Responds to add and remove class commands
	}
	if o.IsDisabled {
		c.attributes.SetDisabled(o.IsDisabled)
	}
	if o.IsRequired {
		c.isRequired = true
	}
	if o.IsHidden {
		c.isHidden = true
	}
	//c.dataConnector = o.DataConnector
	/*
		if o.WatchedDbTables != nil {
			c.WatchDbTables(ctx, o.WatchedDbTables...)
		}*/
}

// Creator is the interface all declarative helpers need to implement.
// It is used to add multiple controls with various settings from a single Go struct.
type Creator interface {
	Create(ctx context.Context, parent ControlI) ControlI
}

// AddControls adds sub-controls to a control using Creator objects
func (c *ControlBase) AddControls(ctx context.Context, creators ...Creator) {
	for _, creator := range creators {
		creator.Create(ctx, c)
	}
}

func init() {
	gob.Register(new(ControlBase))
	gob.Register(new(stateType))
	gob.Register(new(stateStoreType))
	gob.Register(new(maps.Map[string, any]))
	gob.Register(new(maps.Map[string, SavedState]))
	gob.Register(new(eventMap))
	gob.Register(new(map[event.EventID]*event.Event))
}
