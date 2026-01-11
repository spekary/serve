package dialog

import (
	"context"

	"github.com/goradd/serve/control"
	"github.com/goradd/serve/i18n"
	"github.com/goradd/serve/page"
	"github.com/goradd/serve/page/action"
)

const (
	editDlgSaveAction = iota + 10100
	editDlgDeleteAction
)

const (
	SaveButtonID    = "saveBtn"
	CancelButtonnID = "cancelBtn"
	DeleteButtonID  = "deleteBtn"
)

type EditablePanel interface {
	control.PanelI
	Load(ctx context.Context, pk string) error
	Save(ctx context.Context)
	Delete(ctx context.Context)
	DataI() interface{}
}

// EditPanel is a dialog panel that pre-loads Save, Cancel and Delete buttons, and treats its one
// child control as an EditablePanel.
type EditPanel struct {
	DialogPanel
	ObjectName string
}

// GetEditPanel creates a panel that is designed to hold an EditablePanel. It itself will be wrapped with
// the application's default dialog style, and it will automatically get Save, Cancel and Delete buttons.
func GetEditPanel(parent page.ControlI, id string, objectName string) (*EditPanel, bool) {
	if c := parent.Form().GetControl(id); c != nil { // dialog has already been created, but is hidden
		return c.(*EditPanel), false
	}

	dlg := NewDialogI(parent.Form(), id+"-dlg")
	dp := new(EditPanel)
	dp.Init(dp, dlg, id, objectName)
	return dp, true
}

func (p *EditPanel) Init(self page.ControlI, dlg page.ControlI, id string, objectName string) {
	p.DialogPanel.Init(self, dlg, id)
	p.AddButton(p.T("Delete", i18n.Domain(i18n.FrameworkDomain)),
		DeleteButtonID,
		&ButtonOptions{
			PushLeft: true,
			ConfirmationMessage: p.T("Are you sure you want to delete this %s?", objectName,
				i18n.Domain(i18n.FrameworkDomain)),
			OnClick: action.Do().ID(editDlgDeleteAction).ControlID(p.ID()),
		})

	p.AddCloseButton(p.T("Cancel", i18n.Domain(i18n.FrameworkDomain)), CancelButtonnID)
	p.AddButton(p.T("Save", i18n.Domain(i18n.FrameworkDomain)), SaveButtonID, &ButtonOptions{
		Validates: true,
		IsPrimary: true,
		OnClick:   action.Do().ID(editDlgSaveAction).ControlID(p.ID()),
	})

	p.ObjectName = objectName
}

func (p *EditPanel) Load(ctx context.Context, pk string) (data interface{}, err error) {
	ep := p.EditPanel()
	if ep == nil {
		panic("the child of an EditDialog must be an EditablePanel")
	}

	err = ep.Load(ctx, pk)
	if err != nil {
		return
	}

	if pk == "" {
		// Editing a new item
		p.SetButtonVisible(DeleteButtonID, false)
		p.SetTitle(p.T("New %s", p.ObjectName, i18n.Domain(i18n.FrameworkDomain)))
	} else {
		p.SetButtonVisible(DeleteButtonID, true)
		p.SetTitle(p.T("Edit %s", p.ObjectName, i18n.Domain(i18n.FrameworkDomain)))
	}
	data = ep.DataI()
	return
}

func (p *EditPanel) DoAction(ctx context.Context, a action.Params) {
	switch a.ID {
	case editDlgSaveAction:
		p.EditPanel().Save(ctx)
		p.Hide()
	case editDlgDeleteAction:
		p.EditPanel().Delete(ctx)
		p.Hide()
	default:
		p.DialogPanel.DoAction(ctx, a)
	}
}

// EditPanel returns the panel that has the edit controls
func (p *EditPanel) EditPanel() EditablePanel {
	for c := range p.ChildControls() {
		if ep, ok := c.(EditablePanel); ok {
			return ep
		}

	}
	return nil
}

func init() {
	page.RegisterControl(func() page.ControlI { return new(EditPanel) })
}
