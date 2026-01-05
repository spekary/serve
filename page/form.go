package page

import (
	"context"
	"io"
	http2 "net/http"
)

type FormI interface {
	ID() string
	Init(ctx context.Context, f FormI, p *Page)
	SetupNewForm(ctx context.Context, p *Page)
	Run(ctx context.Context) error
	Exit(ctx context.Context, w http2.ResponseWriter)

	Unmarshalled()
	Cleanup()

	PageDrawingFunction() PageDrawFunc
	DrawHeaderTags(ctx context.Context, w io.Writer)
	RenderAjax(ctx context.Context, w io.Writer)
}
