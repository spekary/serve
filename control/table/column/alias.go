package column

import (
	"context"

	"github.com/goradd/goradd/pkg/orm/query"
	table2 "github.com/goradd/serve/control/table"
	"github.com/goradd/serve/page"
)

type AliasGetter interface {
	GetAlias(key string) query.AliasValue
}

// AliasColumn is a column that uses the AliasGetter interface to get the alias text out of a database object.
// The data therefore should be a slice of objects that implement the AliasGetter interface. All ORM objects
// are AliasGetters (or should be). Call NewAliasColumn to create the column.
type AliasColumn struct {
	table2.ColumnBase
	alias string
}

// NewAliasColumn creates a new table column that gets its text from an alias attached to an ORM object.
// If the alias has a Date type, you MUST call SetTimeFormat to set the format of the printed string.
func NewAliasColumn(alias string) *AliasColumn {
	i := AliasColumn{}
	i.Init(alias)
	return &i
}

func (c *AliasColumn) Init(alias string) {
	c.ColumnBase.Init(c)
	c.SetTitle(alias)
	c.alias = alias
}

func GetNode(c *AliasColumn) query.NodeI {
	return query.Alias(c.alias)
}

// CellData is called by the framework to get the data to display in a cell.
func (c *AliasColumn) CellData(ctx context.Context, row int, col int, data interface{}) interface{} {
	if v, ok := data.(AliasGetter); !ok {
		return ""
	} else {
		a := v.GetAlias(c.alias)
		if a.IsNil() {
			return ""
		}
		return a
	}
}

func (c *AliasColumn) Serialize(e page.Encoder) {
	c.ColumnBase.Serialize(e)
	if err := e.Encode(c.alias); err != nil {
		panic(err)
	}
}

func (c *AliasColumn) Deserialize(dec page.Decoder) {
	c.ColumnBase.Deserialize(dec)
	if err := dec.Decode(&c.alias); err != nil {
		panic(err)
	}
}

// AliasColumnCreator creates a column that displays the content of a database alias. Each row must be
// an AliasGetter, which by default all the output from database queries provide that.
type AliasColumnCreator struct {
	// ID will assign the given id to the column. If you do not specify it, an id will be given it by the framework.
	ID string
	// Alias is the name of the alias to use when getting data out of the provided database row
	Alias string
	// Title is the static title string to use in the header row
	Title string
	// Sortable makes the column display sort arrows in the header
	// Deprecated: Use SortDirection instead
	Sortable bool
	// SortDirection sets the initial sorting direction of the column, and will make the column sortable
	// By default, the column is not sortable.
	SortDirection table2.SortDirection
	// IsHtml indicates that the texter is producing HTML rather than text that should be escaped.
	table2.ColumnOptions
}

func (c AliasColumnCreator) Create(ctx context.Context, parent table2.TableI) table2.ColumnI {
	col := NewAliasColumn(c.Alias)
	if c.ID != "" {
		col.SetID(c.ID)
	}
	col.SetTitle(c.Title)
	if c.Sortable {
		col.SetSortable()
	}
	if c.SortDirection != table2.NotSortable {
		col.SetSortDirection(c.SortDirection)
	}
	col.ApplyOptions(ctx, parent, c.ColumnOptions)
	return col
}

func init() {
	table2.RegisterColumn(AliasColumn{})
}
