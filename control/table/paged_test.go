package table

import (
	"bytes"
	"context"
	"encoding/gob"
	"testing"

	control2 "github.com/goradd/serve/control"
	"github.com/goradd/serve/page"

	"github.com/goradd/html5tag"
	"github.com/stretchr/testify/assert"
)

type pagedTableTestForm struct {
	page.MockForm
}

func (f *pagedTableTestForm) Init(self page.FormI, id string) {
	f.MockForm.Init(self, id)
}

func (*pagedTableTestForm) RowAttributes(row int, data interface{}) html5tag.Attributes {
	return html5tag.NewAttributes().AddValues("a", "b")
}

func (*pagedTableTestForm) HeaderRowAttributes(row int) html5tag.Attributes {
	return html5tag.NewAttributes().AddValues("c", "d")
}

func (*pagedTableTestForm) FooterRowAttributes(row int) html5tag.Attributes {
	return html5tag.NewAttributes().AddValues("e", "f")
}

func (*pagedTableTestForm) BindData(ctx context.Context, s control2.DataManagerI) {

}

func TestPagedTable_Serialize(t *testing.T) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	f := new(pagedTableTestForm)
	f.Init(f, "MockFormId")

	f.AddControls(context.Background(),
		PagedTableCreator{
			ID:               "table",
			Caption:          "This is a table",
			HideIfEmpty:      true,
			HeaderRowCount:   2,
			FooterRowCount:   3,
			RowStylerID:      f.ID(),
			HeaderRowStyler:  f,
			FooterRowStyler:  f,
			DataProvider:     f,
			Sortable:         true,
			SortHistoryLimit: 3,
			OnCellClick:      nil,
			PageSize:         7,
			SaveState:        false, // must have a session to test
			Columns:          nil,   // testing columns here will cause circular import
		},
		control2.DataPagerCreator{
			ID:             "dp",
			PagedControlID: "table",
		},
	)

	c := GetPagedTable(f, "table")

	c.Serialize(enc)

	c2 := PagedTable{}
	dec := gob.NewDecoder(&buf)
	c2.Deserialize(dec)

	assert.Equal(t, "This is a table", c2.caption)
	assert.Equal(t, 3, c2.sortHistoryLimit)
}
