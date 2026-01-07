package page

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testControl struct {
	ControlBase
}

type testForm struct {
	FormBase
}

func newTestForm() FormI {
	f := new(testForm)
	f.setPage(new(Page))
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
	c3 := newTestControl(form, c, "c3")

	assert.Equal(t, "c1", c.ID())
	assert.Equal(t, "c2", c2.ID(), "skipped already existing control id c1")
	assert.Len(t, form.childIDs(), 2)
	assert.Len(t, c.childIDs(), 1)
	assert.Len(t, c3.childIDs(), 0)

	assert.Panics(t, func() {
		newTestControl(form, form, "c3")
	}, "registering duplicate control")
}

func Test_treeNode_Detach(t *testing.T) {
	form := newTestForm()
	c1 := newTestControl(form, form, "c1")
	c2 := newTestControl(form, c1, "c2")
	c3 := newTestControl(form, c2, "c3")

	assert.Equal(t, "c1", c2.parentID())
	assert.Len(t, c1.childIDs(), 1)
	c2.Detach()
	assert.Equal(t, "", c2.parentID())
	assert.Equal(t, "c2", c3.parentID())
	assert.Len(t, c3.childIDs(), 0)
}

func Test_treeNode_SetParent(t *testing.T) {
	form := newTestForm()
	c1 := newTestControl(form, form, "c1")
	c2 := newTestControl(form, form, "c2")
	c3 := newTestControl(form, c2, "c3")

	ret := c3.SetParentControl(c2)
	assert.False(t, ret)

	ret = c3.SetParentControl(c1)
	assert.True(t, ret)

	assert.Equal(t, "c1", c3.parentID())
	assert.Len(t, c1.childIDs(), 1)
	assert.Len(t, c2.childIDs(), 0)

	// same as Detach()
	ret = c3.SetParentControl(nil)
	assert.True(t, ret)
	assert.Equal(t, "", c3.parentID())
}

func Test_treeNode_AllChildren(t *testing.T) {
	tests := []struct {
		name     string
		childIDs []string
		wantIDs  []string
	}{
		{
			"Multiple children",
			[]string{"c1", "c2", "c3"},
			[]string{"c1", "c2", "c3"},
		},
		{
			"No children",
			[]string{},
			[]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := newTestForm()
			for _, id := range tt.childIDs {
				newTestControl(form, form, id)
			}

			gotNodes := slices.Collect(form.ChildControls())

			// Extract IDs for easy comparison
			var gotIDs []string
			for _, node := range gotNodes {
				gotIDs = append(gotIDs, node.ID())
			}

			if !slices.Equal(gotIDs, tt.wantIDs) {
				t.Errorf("ChildControls() = %v, want %v", gotIDs, tt.wantIDs)
			}
		})
	}
}

type mockSerializer struct {
	failAt int
	calls  int
	err    error
}

func (m *mockSerializer) Encode(v any) error {
	m.calls++

	if m.failAt > 0 && m.calls == m.failAt {
		return m.err
	}
	return nil
}

func (m *mockSerializer) Decode(v any) error {
	m.calls++

	if m.failAt > 0 && m.calls == m.failAt {
		return m.err
	}
	return nil
}

func Test_treeNode_Serialize(t *testing.T) {
	// Just testing tree node serialization and deserialization
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	d := gob.NewDecoder(&b)

	tn := treeNode{
		id:       "a",
		parentId: "b",
		childIds: []string{"c", "d", "e"},
		form:     nil,
	}

	tn.serialize(e)

	tn2 := treeNode{}
	tn2.deserialize(d)
	assert.Equal(t, "a", tn2.id)
	assert.Equal(t, "b", tn2.parentId)
	assert.Equal(t, []string{"c", "d", "e"}, tn2.childIds)

	// test panic
	for i := 1; i <= 3; i++ {
		e2 := &mockSerializer{
			failAt: i,
			err:    fmt.Errorf("error"),
		}
		assert.Panics(t, func() {
			tn.serialize(e2)
		})
	}

	for i := 1; i <= 3; i++ {
		e2 := &mockSerializer{
			failAt: i,
			err:    fmt.Errorf("error"),
		}
		assert.Panics(t, func() {
			tn.deserialize(e2)
		})
	}

}

func Test_treeNode_Coverage(t *testing.T) {
	// Basic tests for code coverage

	form := newTestForm()
	c1 := newTestControl(form, form, "c1")
	assert.Equal(t, form, c1.Form())
	assert.Equal(t, form, c1.ParentControl())
	assert.True(t, form.HasChildControls())
	assert.False(t, c1.HasChildControls())
	assert.Equal(t, c1, form.FindChildControl("c1"))
	assert.Nil(t, c1.FindChildControl("c1"))
	c1.setForm(nil)
	assert.Nil(t, c1.Form())
}

func Test_treeNode_AllChildControls(t1 *testing.T) {
	form := newTestForm()
	c1 := newTestControl(form, form, "c1")
	c2 := newTestControl(form, form, "c2")
	_ = newTestControl(form, c1, "")
	_ = newTestControl(form, c2, "")
	c22 := newTestControl(form, c2, "")

	var count int
	for c := range form.AllChildControls() {
		_, ok := c.(ControlI)
		assert.True(t1, ok)

		count++
	}
	assert.Equal(t1, 5, count)

	count = 0
	for c := range c1.AllChildControls() {
		_, ok := c.(ControlI)
		assert.True(t1, ok)
		count++
	}
	assert.Equal(t1, 1, count)

	count = 0
	for _ = range c22.AllChildControls() {
		count++
	}
	assert.Equal(t1, 0, count)

	// test failures
	tests := []struct {
		name  string
		count int
	}{
		{"fail at 1", 1},
		{"fail at 2", 2},
		{"fail at 3", 3},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			var count2 int
			form.AllChildControls()(func(i ControlI) bool {
				count2++
				if count2 == tt.count {
					return false
				}
				return true
			})
			assert.Equal(t1, tt.count, count2)
		})
	}
}
