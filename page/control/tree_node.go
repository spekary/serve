package control

import (
	"iter"

	"github.com/goradd/serve/log"
	"github.com/goradd/serve/page"
	"golang.org/x/exp/slices"
)

type treeNoder interface {
	ID() string
	Detach()
	SetParent(newParent treeNoder) bool
	Parent() ControlI
	Children() iter.Seq[ControlI]
	AllChildren() iter.Seq[ControlI]
	Form() FormI

	childIDs() []string
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
		id = t.form.generateId()
	}
	t.id = id
	form.cacheControl(self)
	t.SetParent(parent)
}

func (t *treeNode) ID() string {
	return t.id
}

// Form returns the enclosing form of the control.
func (t *treeNode) Form() FormI {
	return t.form
}

// Parent returns the parent control, or nil if this has no parent.
func (t *treeNode) Parent() ControlI {
	return t.form.GetControl(t.parentId)
}

// SetParent changes the parent of the node to newParent.
// If the node is already in the tree, will remove it from its sibling list.
// Returns true if data changed.
func (t *treeNode) SetParent(newParent treeNoder) bool {
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

// Detach removes the item from its parent.
// After this call, the item is still in the control cache, so it is still available for use, but you
// should remember its id so you can get it from the cache again in the future.
func (t *treeNode) Detach() {
	parentNode := t.parentNode()
	if parentNode == nil {
		if t.parentId != "" {
			log.Debug(nil, logModule, "Parent node is missing")
		}
		return
	}
	parentNode.removeChild(t)
	t.parentId = ""
}

func (t *treeNode) parentNode() treeNoder {
	if t.parentId == "" {
		return nil
	}
	return t.form.GetControl(t.parentId)
}

func (t *treeNode) childIDs() []string {
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

// Children returns an iterator that yields every immediate child control.
func (t *treeNode) Children() iter.Seq[ControlI] {
	return func(yield func(ControlI) bool) {
		for _, id := range t.childIds {
			child := t.form.GetControl(id)
			if child == nil {
				continue // Skip if child was removed from cache
			}
			if !yield(child) {
				return
			}
		}
	}
}

// AllChildren returns an iterator that recursively yields every child control and
// subchild of the control.
func (t *treeNode) AllChildren() iter.Seq[ControlI] {
	return func(yield func(ControlI) bool) {
		for child := range t.Children() {
			if !yield(child) {
				return
			}
			for child2 := range child.AllChildren() {
				if !yield(child2) {
					return
				}
			}
		}
	}
}

func (t *treeNode) serialize(e page.Encoder) {
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

func (t *treeNode) deserialize(d page.Decoder) {
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
