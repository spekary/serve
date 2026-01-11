package dialog

import (
	"context"

	"github.com/goradd/serve/control"
	"github.com/goradd/serve/i18n"
	"github.com/goradd/serve/page"
	"github.com/goradd/serve/page/action"
)

type SaveablePanel interface {
	control.PanelI
	Load(ctx context.Context, pk string) error
	Save(ctx context.Context)
	Data() interface{}
}

// SavePanel is a dialog panel that pre-loads Save, Cancel and Delete buttons, and treats its one
// child control as an EditablePanel.
type SavePanel struct {
	DialogPanel
}

// GetSavePanel creates a panel that is designed to hold an SaveablePanel. It itself will be wrapped with
// the application's default dialog style, and it will automatically get Save, Cancel and Delete buttons.
func GetSavePanel(parent page.ControlI, id string) (*SavePanel, bool) {
	if c := parent.Form().GetControl(id); c != nil { // dialog has already been created, but is hidden
		return c.(*SavePanel), false
	}

	dlg := NewDialogI(parent.Form(), id+"-dlg")
	dp := new(SavePanel)
	dp.Init(dp, dlg, id)
	return dp, true
}

func (p *SavePanel) Init(self page.ControlI, dlg page.ControlI, id string) {
	p.DialogPanel.Init(self, dlg, id)
	p.AddCloseButton(p.T("Cancel", i18n.Domain(i18n.FrameworkDomain)), CancelButtonnID)
	p.AddButton(p.T("Save", i18n.Domain(i18n.FrameworkDomain)), SaveButtonID, &ButtonOptions{
		Validates: true,
		IsPrimary: true,
		OnClick:   action.Do(),
	})
}

func (p *SavePanel) Load(ctx context.Context, pk string) (data interface{}, err error) {
	ep := p.SavePanel()
	if ep == nil {
		panic("the child of a SaveDialog must be an SaveablePanel")
	}

	err = ep.Load(ctx, pk)
	if err != nil {
		return
	}

	data = ep.Data()
	return
}

func (p *SavePanel) DoAction(ctx context.Context, a action.Params) {
	switch a.ControlId {
	case SaveButtonID:
		p.SavePanel().Save(ctx)
		p.Hide()
	default:
		p.DialogPanel.DoAction(ctx, a)
	}
}

// SavePanel returns the panel that has the controls
func (p *SavePanel) SavePanel() SaveablePanel {
	for c := range p.AllChildControls() {
		if c, ok := c.(SaveablePanel); ok {
			return c
		}
	}
	return nil
}

func init() {
	page.RegisterControl(func() page.ControlI { return new(SavePanel) })
}
