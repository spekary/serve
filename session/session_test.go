package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// runRequestTest will run a session test by first calling the setupHandler, and then calling the testHandler
// mimicking a process where a session variable is set in one request, and then retrieved in a later request
func runRequestTest(t *testing.T, setupHandler, testHandler http.Handler) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	setupHandler.ServeHTTP(rec, req)

	if status := rec.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v, body: %v",
			status, http.StatusOK, rec.Body)
	}

	// extract cookie
	cookie := rec.Header().Get("Set-Cookie")

	// now run it through the tester
	req = httptest.NewRequest("GET", "/", nil)
	rec = httptest.NewRecorder()
	req.Header.Set("Cookie", cookie)

	testHandler.ServeHTTP(rec, req)
}

const intKey = "test.intKey"
const boolKey = "test.boolKey"
const stringKey = "test.stringKey"
const floatKey = "test.floatKey"

func setRequestHandler(sessionManager ManagerI) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		setupTest(ctx)
	}

	return sessionManager.Use(http.HandlerFunc(fn))
}

func testRequestHandler(t *testing.T, sessionManager ManagerI) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		runTest(t, ctx)

	}
	return sessionManager.Use(http.HandlerFunc(fn))
}

func setupTest(ctx context.Context) {
	SetInt(ctx, intKey, 4) // testing replacing a value here
	SetInt(ctx, intKey, 5)
	SetBool(ctx, boolKey, true)
	SetString(ctx, stringKey, "Here")
	SetFloat64(ctx, floatKey, 7.6)
}

func runTest(t *testing.T, ctx context.Context) {
	i := GetInt(ctx, intKey)
	assert.Equal(t, 5, i)
	assert.True(t, Has(ctx, intKey))
	assert.False(t, Has(ctx, "randomval"))

	// test that getting the wrong kind of value produces no value
	s := GetString(ctx, intKey)
	assert.Equal(t, s, "")

	b := GetBool(ctx, boolKey)
	assert.True(t, b)

	f := GetFloat64(ctx, floatKey)
	assert.Equal(t, 7.6, f)
	// repeat
	f = GetFloat64(ctx, floatKey)
	assert.Equal(t, 7.6, f)

	f2 := GetFloat32(ctx, floatKey)
	assert.Equal(t, float32(0.0), f2)

	Clear(ctx)
	f = GetFloat64(ctx, floatKey)
	assert.Equal(t, 0.0, f)

}
