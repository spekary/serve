package page

import (
	"iter"

	"github.com/goradd/serve/log"
	"golang.org/x/exp/slices"
)

type treeNoder interface {
	ID() string
	Detach()
	SetParentControl(newParent treeNoder) bool
	ParentControl() ControlI
	ChildControls() iter.Seq[ControlI]
	AllChildControls() iter.Seq[ControlI]
	Form() FormI
	HasChildControls() bool
	FindChildControl(id string) ControlI

	ChildControlIDs() []string
	parentID() string
	addChild(treeNoder)
	removeChild(treeNoder)
	setForm(form FormI)
}

// treeNode is a mixin for the ControlBase.
type treeNode struct {
	// id is the id passed to the control when it is created, or assigned automatically if empty.
	id string

	// parent is the immediate parent control of this control. Only the form object will not have a parent.
	parentId string
	// children are the child controls that belong to this control. They are cached for speed, and to allow
	// children of controls to be accessed even when the control is not part of the form.
	childIds []string // Child controls
	form     FormI
}

func (t *treeNode) Init(self ControlI, form FormI, parent treeNoder, id string) {
	t.form = form
	if id == "" {
		id = form.GenerateId()
	}
	t.id = id
	form.addControl(self)
	t.SetParentControl(parent)
}

func (t *treeNode) ID() string {
	return t.id
}

// Form returns the enclosing form of the control.
func (t *treeNode) Form() FormI {
	return t.form
}

// ParentControl returns the parent control, or nil if this has no parent.
func (t *treeNode) ParentControl() ControlI {
	return t.form.GetControl(t.parentId)
}

// SetParentControl changes the parent control of the control node to newParent.
// If the control is already in the tree, will remove it from its sibling list.
// Returns true if data changed.
// Altered controls are refreshed.
func (t *treeNode) SetParentControl(newParent treeNoder) bool {
	if newParent == nil {
		if t.parentId == "" {
			// no change to parent, already has no parent
			return false
		}
		t.Detach()
		t.parentId = ""
		return true
	}
	if newParent.ID() == t.parentId {
		return false // already the parent
	}
	t.Detach()
	t.parentId = newParent.ID()
	newParent.addChild(t)
	return true
}

// Detach removes the item from its parent control.
// After this call, the item is still in the control cache, so it is still available for use, but you
// should remember its id so you can get it from the cache again in the future.
func (t *treeNode) Detach() {
	parentNode := t.parentNode()
	if parentNode == nil {
		if t.parentId != "" {
			log.Error(nil, logModule, "Parent control node is missing",
				"parent_id", t.parentId)
		}
		return
	}
	parentNode.removeChild(t)
	t.parentId = ""
}

func (t *treeNode) HasChildControls() bool {
	return len(t.childIds) > 0
}

func (t *treeNode) FindChildControl(id string) ControlI {
	for _, cid := range t.childIds {
		if cid == id {
			return t.form.GetControl(id)
		}
	}
	return nil
}

func (t *treeNode) parentNode() treeNoder {
	if t.parentId == "" {
		return nil
	}
	return t.form.GetControl(t.parentId)
}

// ChildControlIDs returns the ids of all the child controls in this control
func (t *treeNode) ChildControlIDs() []string {
	return t.childIds
}

func (t *treeNode) parentID() string {
	return t.parentId
}

func (t *treeNode) addChild(child treeNoder) {
	t.childIds = append(t.childIds, child.ID())
}

func (t *treeNode) removeChild(child treeNoder) {
	t.childIds = slices.DeleteFunc(t.childIds, func(s string) bool {
		return s == child.ID()
	})
}

// ChildControls returns an iterator that yields every immediate child control.
func (t *treeNode) ChildControls() iter.Seq[ControlI] {
	return func(yield func(ControlI) bool) {
		for _, id := range t.childIds {
			child := t.form.GetControl(id)
			if child == nil {
				log.Error(nil, logModule, "Child control node is missing")
				continue // Skip if child was removed from control cache. This is an error.
			}
			if !yield(child) {
				return
			}
		}
	}
}

// AllChildControls returns an iterator that recursively yields every child control and
// subchild of the control.
func (t *treeNode) AllChildControls() iter.Seq[ControlI] {
	return func(yield func(ControlI) bool) {
		for child := range t.ChildControls() {
			if !yield(child) {
				return
			}
			for child2 := range child.AllChildControls() {
				if !yield(child2) {
					return
				}
			}
		}
	}
}

func (t *treeNode) serialize(e Encoder) {
	if err := e.Encode(t.id); err != nil {
		panic(err)
	}
	if err := e.Encode(t.parentId); err != nil {
		panic(err)
	}
	if err := e.Encode(t.childIds); err != nil {
		panic(err)
	}
}

func (t *treeNode) deserialize(d Decoder) {
	if err := d.Decode(&t.id); err != nil {
		panic(err)
	}
	if err := d.Decode(&t.parentId); err != nil {
		panic(err)
	}
	if err := d.Decode(&t.childIds); err != nil {
		panic(err)
	}
}

// setForm is called by the framework to set the parent form of the control after
// deserialization.
// You do not normally need to call this function.
func (t *treeNode) setForm(form FormI) {
	t.form = form
}
