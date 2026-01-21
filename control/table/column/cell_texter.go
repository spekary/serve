package column

import (
	"github.com/goradd/serve/control/table"
	"github.com/goradd/serve/page"
)

type CellTexter interface {
	table.CellTexter
}

func GetCellTexter(ctrl page.ControlI, id string) CellTexter {
	return ctrl.Form().GetControl(id).(CellTexter)
}
