package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goradd/serve/config"
)

func clearGlobals() {
	config.ProxyPath = ""
}

func fnFound(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "Found")
}

func fnNotFound(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "Not Found")
}

func Test_PatternRegistrations(t *testing.T) {
	type testT struct {
		name      string
		proxyPath string
		foundPath string
		path      string
		code      int
		result    string
	}

	tests := []testT{
		{"not found", "/abc", "/test", "/test", 200, "Not Found"},
		{"redirect", "/abc", "/test/", "/abc/test", 301, ""}, // redirect
		{"level 0", "/abc", "/test/", "/abc/test/", 200, "Found"},
		{"level 1", "/abc", "/test/", "/abc/test/test3", 200, "Found"},
		{"level 2", "/abc", "/test/", "/abc/test/test3/test4", 200, "Found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.ProxyPath = tt.proxyPath
			PatternMuxer = http.NewServeMux()
			RegisterStaticHandler(tt.foundPath, http.HandlerFunc(fnFound))
			h := WithMuxer(PatternMuxer, http.HandlerFunc(fnNotFound))

			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)
			s := string(body)

			if tt.code != resp.StatusCode {
				t.Errorf("got %d, want %d", resp.StatusCode, tt.code)
			} else if tt.code == 200 {
				if tt.result != s {
					t.Errorf("got %s, want %s", s, tt.result)
				}
			}
		})
	}
}

func Test_AppRegistrations(t *testing.T) {
	type testT struct {
		name      string
		proxyPath string
		foundPath string
		path      string
		code      int
		result    string
	}

	tests := []testT{
		{"not found", "/abc", "/test", "/test", 200, "Not Found"},
		{"redirect", "/abc", "/test/", "/abc/test", 301, ""}, // redirect
		{"level 0", "/abc", "/test/", "/abc/test/", 200, "Found"},
		{"level 1", "/abc", "/test/", "/abc/test/test3", 200, "Found"},
		{"level 2", "/abc", "/test/", "/abc/test/test3/test4", 200, "Found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.ProxyPath = tt.proxyPath
			AppMuxer = http.NewServeMux()
			RegisterAppHandler(tt.foundPath, http.HandlerFunc(fnFound))
			h := WithMuxer(AppMuxer, http.HandlerFunc(fnNotFound))

			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)
			s := string(body)

			if tt.code != resp.StatusCode {
				t.Errorf("got %d, want %d", resp.StatusCode, tt.code)
			} else if tt.code == 200 {
				if tt.result != s {
					t.Errorf("got %s, want %s", s, tt.result)
				}
			}
		})
	}
}

/*
func Test_PathRegistrations(t *testing.Translate) {
	clearGlobals()
	fnFound := func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "Found")
	}
	fnFoundRoot := func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "Found Root")
	}
	fnNotFound := func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "Not Found")
	}

	type testT struct {
		path   string
		code   int
		result string
	}

	RegisterPrefixHandler("", http.HandlerFunc(fnFoundRoot))
	assert.Panics(t, func() {
		RegisterPrefixHandler("/", http.HandlerFunc(fnFoundRoot)) //
	},
		"Blank path and root path should be equal",
	)
	RegisterPrefixHandler("test", http.HandlerFunc(fnFound))

	mux2 := NewMux()
	mux2.Handle("/test3", http.HandlerFunc(fnFound))
	RegisterPrefixHandler("/test/test2/", mux2)
	mux := NewMux() // test using mux in the middle of registration process
	h := UsePatternMuxer(mux, http.HandlerFunc(fnNotFound))

	tests := []testT{
		{"/test4", 200, "Found Root"},
		{"/test/", 200, "Found"},
		{"/test/test2/", 404, "404 page not found\n"},
		{"/test/test2/test3", 200, "Found"},
		{"/test/test2/test4", 404, "404 page not found\n"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.Translate) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			resp := w.Result()
			body, _ := io.ReadAll(resp.Body)
			s := string(body)

			assert.Equal(t, tt.code, resp.StatusCode)
			assert.Equal(t, tt.result, s)
		})
	}

}

func drawTest(ctx context.Context, w io.Writer) (err error) {
	_, _ = io.WriteString(w, "test")
	return nil
}

func drawTestErr(ctx context.Context, w io.Writer) (err error) {
	_, _ = io.WriteString(w, "test")
	return fmt.Errorf("testErr")
}

func TestRegisterDrawFunc(t *testing.Translate) {
	fnNotFound := func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "Not Found")
	}

	m := NewMux()
	_ = UseAppMuxer(m, http.HandlerFunc(fnNotFound))

	RegisterDrawFunc("/drawTest.html", drawTest)
	RegisterDrawFunc("/drawTestErr", drawTestErr)

	req := httptest.NewRequest("GET", "/drawTest.html", nil)

	w := httptest.NewRecorder()
	AppMuxer.ServeHTTP(w, req)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	assert.EqualValues(t, "test", body)
	assert.Contains(t, w.Header().Get("Content-Type"), "html")

	req = httptest.NewRequest("GET", "/drawTestErr", nil)
	w = httptest.NewRecorder()
	assert.Panics(t, func() {
		AppMuxer.ServeHTTP(w, req)
	})

}
*/
