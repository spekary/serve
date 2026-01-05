package control

import (
	"context"
	"io"
	"iter"
	"reflect"

	"github.com/goradd/base"
	"github.com/goradd/html5tag"
	"github.com/goradd/maps"
	"github.com/goradd/serve/log"
	"github.com/goradd/serve/page"
	"github.com/goradd/serve/page/action"
	"github.com/goradd/serve/page/event"
)

const logModule = "control"

const sessionControlStates string = "goradd.controlStates"
const sessionControlTypeState string = "goradd.controlType"

const RequiredErrorMessage string = "A value is required"

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
	Refresh(i ControlI, model interface{})
	// Update reads data from the control, and puts it into the model
	Update(i ControlI, model interface{})
	// Modifies returns true if the control has been changed such that it will modify its corresponding data
	Modifies(i ControlI, model interface{}) bool
}

// DataLoader is an optional interface that DataConnectors can use if they need to load data from the database
// to present a choice of items to the user to select from. The Load method will be called whenever the entire control
// gets redrawn.
type DataLoader interface {
	Load(ctx context.Context) []interface{}
}

// ControlI is the interface that all controls must support. The functions are implemented by the
// ControlBase methods. See the ControlBase method implementation for a description of each method.

type ControlI interface {
	base.BaseI
	treeNoder

	UpdateFormValues(request *page.RequestContext)
	IsDisabled() bool
	IsOnPage() bool
	Refresh()

	Validate(ctx context.Context) bool
	ValidationState() ValidationState
	ValidationType(*event.Event) event.ValidationType
	SetValidationType(typ event.ValidationType) ControlI
	ChildValidationChanged()
	ResetValidation()

	Serialize(e page.Encoder)
	Deserialize(d page.Decoder)

	// package private functions

	doAction(ctx context.Context)
	initBase(self ControlI)
	validationTypeValue() event.ValidationType
	blockParentValidationValue() bool
	validateSelfAndChildren(ctx context.Context) bool
	validateSelfAndSiblings(ctx context.Context) bool
	validateSiblingsAndChildren(ctx context.Context) bool
	resetValidationValues() (changed bool)
}

type attributeScriptEntry struct {
	id       string        // id of the object to execute the command on. This should be the id of the control, or a related html object.
	f        string        // the  function to call
	commands []interface{} // parameters to the function
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

	// Tag is the tag that will enclose the control, like "div" or "input"
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
	actionValue interface{}
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
	dataConnector DataConnector

	// watchedKeys are the notification keys that will cause the control to refresh.
	watchedKeys map[string]string

	// anything added here needs to be also added to the GOB encoder!
}

// Init initializes a control
func (c *ControlBase) Init(tag string, self ControlI, form FormI, parent ControlI, id string) {
	c.Tag = tag
	c.Base.Init(self)
	c.treeNode.Init(self, form, parent, id)
	c.needsRefresh = true
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

func (c *ControlBase) doAction(ctx context.Context) {
	var e *event.Event
	var ok bool
	var isPrivate bool

	request := page.GetRequest(ctx)

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
					log.Debug(ctx, logModule, "doAction - DoPrivateAction",
						"dest_id", dest.ID(),
						"action_id", p.ID,
						"action_type", reflect.TypeOf(p.Action).String(),
						"trigger_id", p.ControlId)
					dest.DoPrivateAction(ctx, p)
				} else {
					if log.HasLogger(log.FrameworkDebugLog) {
						log.FrameworkDebugf("doAction - DoAction, DestId: %s, ActionId: %d, DoAction: %s, TriggerId: %s",
							dest.ID(), p.ID, reflect.TypeOf(p.Action).String(), p.ControlId)
					}
					dest.DoAction(ctx, p)
				}
			}
		}
	} else {
		log.FrameworkDebug("doAction - failed validation: ", e.String())
	}
}

// Refresh will force the control to be completely redrawn on the next update.
func (c *ControlBase) Refresh() {
	c.needsRefresh = true
}

// IsDisabled returns true if the disabled attribute is true.
func (c *ControlBase) IsDisabled() bool {
	return c.attributes.IsDisabled()
}

// IsOnPage returns true if the control has been rendered on the page.
func (c *ControlBase) IsOnPage() bool {
	return c.isOnPage
}

// UpdateFormValues is called by the framework to cause the control to retrieve its values from the form.
// Control implementations should implement this function and then look in the context ctx
// to retrieve its values.
func (c *ControlBase) UpdateFormValues(request *page.RequestContext) {
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
			for target := c.Parent(); target != nil; target = target.Parent() {
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
					break
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
		var valid bool
		for _, t := range targets {
			valid = t.Validate(ctx) && valid
		}
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
	for sibling := range c.Parent().Children() {
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
	for child := range c.Children() {
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
	for sibling := range c.Parent().Children() {
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
		if c.Parent() != nil {
			c.Parent().ChildValidationChanged() // notify parent wrappers
		}
	}
}

func (c *ControlBase) ResetValidation() {
	var changed bool
	for ctrl := range c.SelfAndAllChildren() {
		changed = ctrl.resetValidationValues() || changed
	}
	if changed {
		if p := c.Parent(); p != nil {
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
// has changed. Parent controls can use this to change messages or attributes in response.
func (c *ControlBase) ChildValidationChanged() {
	if c.Parent() != nil {
		c.Parent().ChildValidationChanged()
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
		for child := range c.AllChildren() {
			if !yield(child) {
				return
			}
		}
	}
}

// Serialize encodes the control for the pagecache serializer.
// Control implementations should call this before their own serialization process.
func (c *ControlBase) Serialize(e page.Encoder) {
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
	if err := e.Encode(c.needsRefresh); err != nil {
		panic(err)
	}
	if err := e.Encode(c.dataConnector); err != nil {
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
func (c *ControlBase) Deserialize(d page.Decoder) {
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
	if err := d.Decode(&c.dataConnector); err != nil {
		panic(err)
	}
	if err := d.Decode(&c.watchedKeys); err != nil {
		panic(err)
	}
}

func init() {
	/*
		gob.Register(new(stateType))
		gob.Register(new(stateStoreType))
		gob.Register(new(maps.Map[string, any]))
		gob.Register(new(maps.Map[string, SavedState]))
		gob.Register(new(eventMap))
		gob.Register(new(map[event.EventID]*event.Event))*/
}
