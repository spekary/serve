package session

import (
	"context"
	"testing"
)

func TestMockSetGet(t *testing.T) {
	// set up the mock session
	s := NewMock()
	ctx := s.With(context.Background())

	// run the session tests
	setupTest(ctx)
	runTest(t, ctx)
}
