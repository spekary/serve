package session

import (
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

func TestSetGet(t *testing.T) {
	// setup the ScsSession
	store := memstore.NewWithCleanupInterval(24 * time.Hour)
	s := scs.New()
	s.Store = store
	sm := NewScsManager(s)

	// run the session tests
	runRequestTest(t, setRequestHandler(sm), testRequestHandler(t, sm))
	runRequestTest(t, setupStackRequestHandler(sm), testStackRequestHandler(t, sm))
}
