package page

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_controlCache_AllControls(t *testing.T) {
	form := newTestForm()
	c1 := newTestControl(form, "c1")
	c2 := newTestControl(form, "c2")
	_ = newTestControl(c1, "")
	_ = newTestControl(c2, "")
	_ = newTestControl(c2, "")

	var count int
	for c := range form.Page().AllControls() {
		_, ok := c.(ControlI)
		assert.True(t, ok)
		count++
	}
	assert.Equal(t, 6, count)

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
		t.Run(tt.name, func(t1 *testing.T) {
			var count2 int
			form.Page().AllControls()(func(i ControlI) bool {
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

func Test_controlCache_serialize(t *testing.T) {
	form := newTestForm()
	c1 := newTestControl(form, "c1")
	c2 := newTestControl(form, "c2")
	_ = newTestControl(c1, "")
	_ = newTestControl(c2, "")
	_ = newTestControl(c2, "")

	var buf bytes.Buffer
	e := gob.NewEncoder(&buf)
	form.Page().controlCache.serialize(e)

	cache := &controlCache{}
	d := gob.NewDecoder(&buf)
	cache.deserialize(d)
	assert.Equal(t, cache.counter, form.Page().controlCache.counter)

	c := cache.getControl("c2")
	assert.NotNil(t, c)
}

func Test_controlCache_setGeneratedIdPrefix(t *testing.T) {
	form := newTestForm()
	form.SetGeneratedIdPrefix("b")
	c1 := newTestControl(form, "")
	c2 := newTestControl(form, "")
	_ = newTestControl(c1, "")
	_ = newTestControl(c2, "")
	c5 := newTestControl(c2, "")

	assert.Equal(t, "b5", c5.ID())
}
