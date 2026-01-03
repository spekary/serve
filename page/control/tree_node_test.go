package control

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testControl struct {
	Control
}

type testForm struct {
	Form
}

func newTestForm() FormI {
	f := new(testForm)
	f.Init(f, "testForm")
	return f
}
func newTestControl(form FormI, parent ControlI, id string) ControlI {
	c := new(testControl)
	c.Init("div", c, form, parent, id)
	return c
}

func Test_treeNode_Init(t *testing.T) {
	form := newTestForm()
	c := newTestControl(form, form, "c1")
	c2 := newTestControl(form, form, "")

	assert.Equal(t, "c1", c.ID())
	assert.Equal(t, "c2", c2.ID())
}
