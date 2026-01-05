package page

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPage_MarshalBinary(t *testing.T) {
	testPage := new(Page)
	testPage.BodyAttributes = `class="test"`
	testPage.stateId = `abcdefg`
	testPage.idPrefix = `d`
	testPage.idCounter = 4
	testPage.title = "My Title"
	testPage.htmlHeaderTags = []string{`<meta bob="foo">`, `<meta name="mike">'`}

	b, err := testPage.MarshalBinary()
	assert.NoError(t, err)
	assert.NotNil(t, b)

	page2 := new(Page)
	err = page2.UnmarshalBinary(b)
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(testPage, page2))
}
