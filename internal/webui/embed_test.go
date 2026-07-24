package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// fakeAssets is a small in-memory build the tests drive HandlerFS against,
// so routing logic is verified without depending on a real `npm run build`
// (or the placeholder committed at internal/webui/dist/index.html).
func fakeAssets() fstest.MapFS {
	return fstest.MapFS{
		"index.html":     {Data: []byte("<html>spa shell</html>")},
		"assets/app.js":  {Data: []byte("console.log('app')")},
		"assets/app.css": {Data: []byte("body{color:red}")},
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandlerFS_ServesRealAssetWithContentType(t *testing.T) {
	t.Parallel()
	h := HandlerFS(fakeAssets())

	rec := get(t, h, "/assets/app.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/app.js = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "console.log('app')" {
		t.Errorf("GET /assets/app.js body = %q, want the real asset body", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" || ct == "text/html; charset=utf-8" {
		t.Errorf("GET /assets/app.js Content-Type = %q, want a javascript content type", ct)
	}
}

func TestHandlerFS_ServesIndexAtRoot(t *testing.T) {
	t.Parallel()
	h := HandlerFS(fakeAssets())

	rec := get(t, h, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "<html>spa shell</html>" {
		t.Errorf("GET / body = %q, want index.html's content", rec.Body.String())
	}
}

func TestHandlerFS_FallsBackToIndexForUnknownSPARoutes(t *testing.T) {
	t.Parallel()
	h := HandlerFS(fakeAssets())

	for _, path := range []string{"/agents", "/p/acme", "/p/acme/t/13"} {
		rec := get(t, h, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, rec.Code)
		}
		if rec.Body.String() != "<html>spa shell</html>" {
			t.Errorf("GET %s body = %q, want index.html's content (SPA fallback)", path, rec.Body.String())
		}
	}
}

func TestHandlerFS_UnknownPathNeverTouchesRealAssetsUnexpectedly(t *testing.T) {
	t.Parallel()
	h := HandlerFS(fakeAssets())

	// A path that happens to share a prefix with a real asset directory but
	// does not name a real file still falls back to index.html rather than
	// 404ing — the SPA owns unknown routes, not the file server.
	rec := get(t, h, "/assets/does-not-exist.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/does-not-exist.js = %d, want 200 (SPA fallback)", rec.Code)
	}
	if rec.Body.String() != "<html>spa shell</html>" {
		t.Errorf("GET /assets/does-not-exist.js body = %q, want index.html's content", rec.Body.String())
	}
}

func TestEmbeddedPlaceholderIndexResolves(t *testing.T) {
	t.Parallel()
	// This is the compile-time trap the package doc warns about: if
	// internal/webui/dist/ ever ended up empty, //go:embed would already
	// have failed the build before this test runs. Here we just prove the
	// production Handler() serves *something* at "/" using whatever is
	// actually embedded right now (the committed placeholder, absent a real
	// `npm run build` + copy step).
	rec := get(t, Handler(), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / via the embedded Handler = %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("GET / via the embedded Handler returned an empty body")
	}
}
