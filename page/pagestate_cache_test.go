package page

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicPageCache(t *testing.T) {
	controlCache := NewFastPageCache(10, 60*60*24)

	p1 := &Page{stateId: "1"}
	p2 := &Page{stateId: "2"}

	controlCache.Set("1", p1)
	controlCache.Set("2", p2)

	p3 := controlCache.Get("1")
	assert.Equal(t, p1, p3)
}
