package control

import (
	"github.com/goradd/serve/log"
	"golang.org/x/exp/slices"
)

type treeNoder interface {
	ID() string
	childIDs() []string
}

// treeNode is a mixin for the control for separating out responsibilities
// and making testing and serialization
type treeNode struct {
	// id is the id passed to the control when it is created, or assigned automatically if empty.
	id string

	// parent is the immediate parent control of this control. Only the form object will not have a parent.
	parentId string
	// children are the child controls that belong to this control. They are cached for speed, and to allow
	// children of controls to be accessed even when the control is not part of the form.
	childIds  []string // Child controls
	registrar register
}

func (t *treeNode) Init(self ControlI, registrar register, parent treeNoder, id string) {
	t.registrar = registrar
	if id == "" {
		id = t.registrar.generateId()
	}
	t.id = id
	registrar.registerControl(self)
	t.SetParent(parent)
}

func (t *treeNode) ID() string {
	return t.id
}

// SetParent sets the parent of the node to newParent.
// If the node is already in the tree, will remove it from its sibling list.
// Returns true if data changed.
func (t *treeNode) SetParent(newParent treeNoder) bool {
	if newParent == nil {
		if t.parentId == "" {
			// no change to parent
			return false
		}
		t.Detach()
		t.parentId = ""
		return true
	}
	if newParent.ID() == t.parentId {
		return false
	}
	t.Detach()
	t.parentId = newParent.ID()
	return true
}

// Detach removes the item from its parent.
// After this call, the item is still in the control registry, so it is still available for use, but you
// should remember its id so you can get it from the registry again in the future.
func (t *treeNode) Detach() {
	parentNode := t.parentNode()
	if parentNode == nil {
		if t.parentId != "" {
			log.Debug(nil, logModule, "Parent node is missing")
		}
		return
	}
	slices.DeleteFunc(parentNode.childIDs(), func(s string) bool {
		return s == t.id
	})
}

func (t *treeNode) parentNode() treeNoder {
	if t.parentId == "" {
		return nil
	}
	return t.registrar.RegisteredControl(t.parentId)
}

func (t *treeNode) childIDs() []string {
	return t.childIds
}
