package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/e601201/3DLibrary/internal/config"
)

func newConfigTestServer(t *testing.T) (http.Handler, *config.Store) {
	t.Helper()
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	srv := New(fstest.MapFS{"index.html": {Data: []byte("app")}}, store)
	return srv, store
}

func doRequest(t *testing.T, srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func decodeConfig(t *testing.T, rec *httptest.ResponseRecorder) config.Config {
	t.Helper()
	var c config.Config
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("invalid JSON: %v (%s)", err, rec.Body.String())
	}
	return c
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid error JSON: %v (%s)", err, rec.Body.String())
	}
	return body.Error.Code
}

func TestGetConfigReturnsDefaults(t *testing.T) {
	srv, _ := newConfigTestServer(t)
	rec := doRequest(t, srv, http.MethodGet, "/api/config", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := decodeConfig(t, rec); got != config.Default() {
		t.Fatalf("config = %+v, want defaults", got)
	}
}

func TestPutConfigSavesAndReturnsConfig(t *testing.T) {
	srv, store := newConfigTestServer(t)
	rec := doRequest(t, srv, http.MethodPut, "/api/config",
		`{"blenderPath":"/usr/bin/blender","libraryDir":"","thumbnailSize":1024,"theme":"dark"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got := decodeConfig(t, rec); got.Theme != "dark" || got.ThumbnailSize != 1024 {
		t.Fatalf("response = %+v", got)
	}
	// 保存されていること(再起動相当: Store から読み直す)
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Theme != "dark" || saved.BlenderPath != "/usr/bin/blender" {
		t.Fatalf("saved = %+v", saved)
	}
}

func TestPutConfigRejectsInvalidValues(t *testing.T) {
	srv, _ := newConfigTestServer(t)
	for name, body := range map[string]string{
		"bad thumbnailSize": `{"blenderPath":"","libraryDir":"","thumbnailSize":300,"theme":"dark"}`,
		"bad theme":         `{"blenderPath":"","libraryDir":"","thumbnailSize":512,"theme":"sepia"}`,
		"broken JSON":       `{`,
	} {
		rec := doRequest(t, srv, http.MethodPut, "/api/config", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
		if code := errorCode(t, rec); code != "validation_failed" {
			t.Errorf("%s: error.code = %q, want validation_failed", name, code)
		}
	}
}

func TestPutConfigInitializesEmptyLibraryDir(t *testing.T) {
	srv, _ := newConfigTestServer(t)
	dir := t.TempDir()
	body, _ := json.Marshal(config.Config{LibraryDir: dir, ThumbnailSize: 512, Theme: "system"})
	rec := doRequest(t, srv, http.MethodPut, "/api/config", string(body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	for _, d := range []string{"source", "cache", "templates"} {
		if info, err := os.Stat(filepath.Join(dir, d)); err != nil || !info.IsDir() {
			t.Errorf("skeleton %s: %v", d, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "templates", "empty.blend")); err != nil {
		t.Errorf("empty.blend: %v", err)
	}
}

func TestPutConfigDoesNotTouchExistingLibrary(t *testing.T) {
	srv, _ := newConfigTestServer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(config.Config{LibraryDir: dir, ThumbnailSize: 512, Theme: "system"})
	rec := doRequest(t, srv, http.MethodPut, "/api/config", string(body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "source")); !os.IsNotExist(err) {
		t.Error("existing dir must not be modified")
	}
}

func TestPutConfigLibraryDirIsFileReturns400(t *testing.T) {
	srv, _ := newConfigTestServer(t)
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(config.Config{LibraryDir: file, ThumbnailSize: 512, Theme: "system"})
	rec := doRequest(t, srv, http.MethodPut, "/api/config", string(body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "library_init_failed" {
		t.Fatalf("error.code = %q, want library_init_failed", code)
	}
}

func TestConfigWrongMethodReturns405(t *testing.T) {
	srv, _ := newConfigTestServer(t)
	rec := doRequest(t, srv, http.MethodDelete, "/api/config", "")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if code := errorCode(t, rec); code != "method_not_allowed" {
		t.Fatalf("error.code = %q", code)
	}
}
