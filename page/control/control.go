package control

import (
	"context"
	"io"

	"github.com/goradd/base"
	"github.com/goradd/html5tag"
	"github.com/goradd/maps"
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
// Control methods. See the Control method implementation for a description of each method.

type ControlI interface {
	base.BaseI
	treeNoder
}

type attributeScriptEntry struct {
	id       string        // id of the object to execute the command on. This should be the id of the control, or a related html object.
	f        string        // the  function to call
	commands []interface{} // parameters to the function
}

// Control is the basis for UI controls and widgets in GoRADD.
// It corresponds to a standard html form object or tag.
// A Control can also associate javascript
// with itself to make sure the javascript is loaded on the page when the control is drawn, and can render
// javascript that will initialize a custom javascript widget.
//
// A Control can have child Controls. It
// can either allow the framework to automatically draw the child Controls as part of the inner-html of
// the Control, can use a template to draw the Child controls, or manually draw them. Controls form
// a hierarchical tree structure, with the Form control being the root of the tree.
//
// A Control is part of a system that will reflect the state of the control between the client and server.
// When a user updates a control in the browser and performs an action that requires a response from the
// server, the GoRADD javascript will gather all the changes in the form and send those to the server.
// The control can read those values and update its own internal state, so that from the perspective
// of the programmer referring to the control, the values in the Control are the same as what the user sees in a browser.
//
// This Control struct is a mixin that all controls should use. You would not normally create a Control directly,
// but rather create one of the "subclasses" of Control.
type Control struct {
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
func (c *Control) Init(tag string, self ControlI, form FormI, parent ControlI, id string) {
	c.Tag = tag
	c.Base.Init(self)
	c.treeNode.Init(self, form, parent, id)
	c.needsRefresh = true
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
