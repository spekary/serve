package page

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPage_Serialize(t *testing.T) {
	testPage := new(Page)
	testPage.BodyAttributes = `class="test"`
	testPage.stateId = `abcdefg`
	testPage.idPrefix = `d`
	testPage.idCounter = 4
	testPage.title = "My Title"
	testPage.htmlHeaderTags = []string{`<meta bob="foo">`, `<meta name="mike">'`}

	var b bytes.Buffer
	assert.NotPanics(t, func() { testPage.Serialize(&b) })

	page2 := new(Page)
	assert.NotPanics(t, func() { page2.Deserialize(&b) })

	assert.True(t, reflect.DeepEqual(testPage, page2))
}
