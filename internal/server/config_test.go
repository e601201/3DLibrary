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

// writeFakeBlender は実在する Blender 実行ファイルの代わりを作る。
// blenderPath は保存時に実在チェックを通るため、テストでも実体が要る。
func writeFakeBlender(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "blender")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPutConfigSavesAndReturnsConfig(t *testing.T) {
	srv, store := newConfigTestServer(t)
	blender := writeFakeBlender(t, t.TempDir())
	body, _ := json.Marshal(config.Config{
		BlenderPath: blender, ThumbnailSize: 1024, Theme: "dark",
	})
	rec := doRequest(t, srv, http.MethodPut, "/api/config", string(body))

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
	if saved.Theme != "dark" || saved.BlenderPath != blender {
		t.Fatalf("saved = %+v", saved)
	}
}

func TestPutConfigResolvesMacAppBundle(t *testing.T) {
	// macOS で /Applications/Blender.app を指定するのは自然な操作。
	// そのまま保存すると生成時に permission denied になるので、
	// 保存の時点で中の実行ファイルへ読み替える
	srv, store := newConfigTestServer(t)
	bundle := filepath.Join(t.TempDir(), "Blender.app")
	binary := filepath.Join(bundle, "Contents", "MacOS", "Blender")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(config.Config{
		BlenderPath: bundle, ThumbnailSize: 512, Theme: "system",
	})
	rec := doRequest(t, srv, http.MethodPut, "/api/config", string(body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	// レスポンスにも保存内容にも、読み替え後のパスが入ること
	// (設定画面の入力欄が実際に使われるパスを表示できるように)
	if got := decodeConfig(t, rec); got.BlenderPath != binary {
		t.Errorf("response blenderPath = %q, want %q", got.BlenderPath, binary)
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.BlenderPath != binary {
		t.Errorf("saved blenderPath = %q, want %q", saved.BlenderPath, binary)
	}
}

func TestPutConfigRejectsUnusableBlenderPath(t *testing.T) {
	// 誤ったパスは「生成に失敗しました: permission denied」ではなく
	// 保存時にはっきり弾く
	srv, _ := newConfigTestServer(t)
	for name, path := range map[string]string{
		"missing":   filepath.Join(t.TempDir(), "no-such-blender"),
		"directory": t.TempDir(),
	} {
		body, _ := json.Marshal(config.Config{
			BlenderPath: path, ThumbnailSize: 512, Theme: "system",
		})
		rec := doRequest(t, srv, http.MethodPut, "/api/config", string(body))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
			continue
		}
		if code := errorCode(t, rec); code != "blender_path_invalid" {
			t.Errorf("%s: error.code = %q, want blender_path_invalid", name, code)
		}
	}
}

func TestPutConfigInitializesLibraryDirWithOSMetadata(t *testing.T) {
	// Finder で作って中を確認しただけの空フォルダには .DS_Store が入る。
	// これで初期化がスキップされると、以降スキャンもテンプレート一覧も
	// 500 になり、保存し直しても復旧しない
	srv, _ := newConfigTestServer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	// 骨格ができていれば、以降のスキャンとテンプレート一覧も通る
	if rec := doRequest(t, srv, http.MethodPost, "/api/scan", ""); rec.Code != http.StatusOK {
		t.Errorf("scan status = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doRequest(t, srv, http.MethodGet, "/api/templates", ""); rec.Code != http.StatusOK {
		t.Errorf("templates status = %d: %s", rec.Code, rec.Body.String())
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
