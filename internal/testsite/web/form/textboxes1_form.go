package form

import (
	"context"

	"github.com/goradd/serve/page"
)

const Textboxes1Path = "/textboxes1"
const Textboxes1Id = "textboxes1"

type TextboxesForm1 struct {
	page.FormBase
}

func (f *TextboxesForm1) Init(self page.FormI, id string) {
	f.FormBase.Init(self, id)
}

func (f *TextboxesForm1) CreateControls(ctx context.Context) {
	f.createTemplateControls(ctx)
}

func init() {
	page.RegisterForm(Textboxes1Path, Textboxes1Id, func() page.FormI {
		f := new(TextboxesForm1)
		return f
	})
}
