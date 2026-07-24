package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	static := fstest.MapFS{
		"index.html":     {Data: []byte("<html><body>app</body></html>")},
		"assets/app.js":  {Data: []byte("console.log('app')")},
		"assets/app.css": {Data: []byte("body{}")},
		".gitkeep":       {Data: []byte("")},
	}
	return New(static)
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("status field = %q, want %q", body.Status, "ok")
	}
}

func TestHealthWrongMethodReturns405(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/health", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Error.Code != "method_not_allowed" {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, "method_not_allowed")
	}
}

func TestUnknownAPIPathReturnsJSONError(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, "not_found")
	}
	if body.Error.Message == "" {
		t.Fatal("error.message is empty")
	}
}

func TestServesStaticFiles(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	b, _ := io.ReadAll(rec.Body)
	if string(b) != "console.log('app')" {
		t.Fatalf("body = %q", string(b))
	}
}

func TestSPAFallbackServesIndexHTML(t *testing.T) {
	srv := newTestServer(t)
	for _, path := range []string{"/", "/assets-page/deep/link"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want %d", path, rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), "<body>app</body>") {
			t.Fatalf("%s: body does not contain index.html content: %q", path, rec.Body.String())
		}
	}
}

func TestMissingFrontendBuildServesHint(t *testing.T) {
	srv := New(fstest.MapFS{".gitkeep": {Data: []byte("")}})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "npm") {
		t.Fatalf("body should hint at building the frontend: %q", rec.Body.String())
	}
}
