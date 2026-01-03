package page

import (
	"context"
	"io"
	http2 "net/http"
)

type FormI interface {
	ID() string
	Init(ctx context.Context, f FormI, p *Page)
	Run(ctx context.Context)
	Exit(ctx context.Context, w http2.ResponseWriter)

	// functions after be restored from the cache
	Restore()
	Cleanup()

	PageDrawingFunction() PageDrawFunc
	DrawHeaderTags(ctx context.Context, w io.Writer)
	RenderAjax(ctx context.Context, w io.Writer)
}
