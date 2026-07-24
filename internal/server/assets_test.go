package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/e601201/3DLibrary/internal/config"
	"github.com/e601201/3DLibrary/internal/index"
)

// newLibraryServer は一時ライブラリ(source あり)を設定済みのサーバーを返す。
func newLibraryServer(t *testing.T) (*Server, string) {
	t.Helper()
	libDir := t.TempDir()
	for _, d := range []string{"source", "cache", "templates"} {
		if err := os.MkdirAll(filepath.Join(libDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	cfg := config.Default()
	cfg.LibraryDir = libDir
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	srv := New(fstest.MapFS{"index.html": {Data: []byte("app")}}, store)
	t.Cleanup(func() { srv.CloseLibrary() })
	return srv, libDir
}

func addAsset(t *testing.T, libDir, category, title string, withBlend bool) {
	t.Helper()
	dir := filepath.Join(libDir, "source", category, title)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if withBlend {
		if err := os.WriteFile(filepath.Join(dir, "model.blend"), []byte("blend"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func listAssets(t *testing.T, srv http.Handler) []index.Asset {
	t.Helper()
	rec := doRequest(t, srv, http.MethodGet, "/api/assets", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/assets = %d: %s", rec.Code, rec.Body.String())
	}
	var assets []index.Asset
	if err := json.Unmarshal(rec.Body.Bytes(), &assets); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return assets
}

func rescan(t *testing.T, srv http.Handler) int {
	t.Helper()
	rec := doRequest(t, srv, http.MethodPost, "/api/scan", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/scan = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		AssetCount int `json:"assetCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return body.AssetCount
}

func TestAssetsNotConfiguredReturns409(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	srv := New(fstest.MapFS{"index.html": {Data: []byte("app")}}, store)

	for _, req := range []struct{ method, path string }{
		{http.MethodGet, "/api/assets"},
		{http.MethodPost, "/api/scan"},
	} {
		rec := doRequest(t, srv, req.method, req.path, "")
		if rec.Code != http.StatusConflict {
			t.Errorf("%s %s = %d, want 409", req.method, req.path, rec.Code)
		}
		if code := errorCode(t, rec); code != "library_not_configured" {
			t.Errorf("%s %s error.code = %q", req.method, req.path, code)
		}
	}
}

func TestScanThenListAssets(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	addAsset(t, libDir, "Props", "NoBlend", false)

	if n := rescan(t, srv); n != 2 {
		t.Fatalf("assetCount = %d, want 2", n)
	}
	assets := listAssets(t, srv)
	if len(assets) != 2 {
		t.Fatalf("len = %d, want 2", len(assets))
	}
	if assets[0].Title != "Chair" || assets[0].IsIncomplete {
		t.Errorf("assets[0] = %+v", assets[0])
	}
	if assets[1].Title != "NoBlend" || !assets[1].IsIncomplete {
		t.Errorf("assets[1] = %+v, want incomplete NoBlend", assets[1])
	}
}

func TestAssetsJSONUsesCamelCase(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	rec := doRequest(t, srv, http.MethodGet, "/api/assets", "")
	var raw []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"isIncomplete", "thumbnailPath", "polygonCount", "updatedAt"} {
		if _, ok := raw[0][key]; !ok {
			t.Errorf("JSON key %q missing: %v", key, raw[0])
		}
	}
}

func TestRescanReflectsFilesystemChanges(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	// Finder で追加 → 再スキャンで現れる
	addAsset(t, libDir, "Characters", "Hero", true)
	rescan(t, srv)
	if len(listAssets(t, srv)) != 2 {
		t.Fatal("added asset should appear after rescan")
	}

	// Finder で削除 → 再スキャンで消える(行も削除)
	if err := os.RemoveAll(filepath.Join(libDir, "source", "Props", "Chair")); err != nil {
		t.Fatal(err)
	}
	rescan(t, srv)
	assets := listAssets(t, srv)
	if len(assets) != 1 || assets[0].Title != "Hero" {
		t.Fatalf("assets = %+v, want only Hero", assets)
	}
}

func TestStartupScanPopulatesIndex(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)

	n, err := srv.StartupScan()
	if err != nil {
		t.Fatalf("StartupScan: %v", err)
	}
	if n != 1 {
		t.Fatalf("StartupScan count = %d, want 1", n)
	}
	if len(listAssets(t, srv)) != 1 {
		t.Fatal("index should be populated without manual scan")
	}
}

func TestStartupScanWithoutLibraryIsNoop(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	srv := New(fstest.MapFS{"index.html": {Data: []byte("app")}}, store)

	n, err := srv.StartupScan()
	if err != nil {
		t.Fatalf("StartupScan should be a no-op without library: %v", err)
	}
	if n != 0 {
		t.Fatalf("count = %d, want 0", n)
	}
}

func TestSwitchingLibraryDirUsesNewIndex(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	// 別ライブラリへ切替(空の source)
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, "source"), 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(config.Config{LibraryDir: other, ThumbnailSize: 512, Theme: "system"})
	if rec := doRequest(t, srv, http.MethodPut, "/api/config", string(body)); rec.Code != http.StatusOK {
		t.Fatalf("PUT config = %d: %s", rec.Code, rec.Body.String())
	}
	if n := rescan(t, srv); n != 0 {
		t.Fatalf("new library should scan to 0, got %d", n)
	}
	if len(listAssets(t, srv)) != 0 {
		t.Fatal("new library index should be empty")
	}
}

func TestAssetsSearchFilterSort(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Wooden Chair", true)
	addAsset(t, libDir, "Props", "Wooden Table", true)
	addAsset(t, libDir, "Characters", "wood elf", true)
	rescan(t, srv)

	// 検索 + カテゴリ + ソートの組み合わせ
	rec := doRequest(t, srv, http.MethodGet, "/api/assets?q=wood&category=Props&sort=updated_desc", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var assets []index.Asset
	if err := json.Unmarshal(rec.Body.Bytes(), &assets); err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 {
		t.Fatalf("len = %d, want 2 (Props の wood*)", len(assets))
	}
	for _, a := range assets {
		if a.Category != "Props" {
			t.Errorf("category = %q", a.Category)
		}
	}
}

func TestAssetsRejectsUnknownSort(t *testing.T) {
	srv, _ := newLibraryServer(t)
	rec := doRequest(t, srv, http.MethodGet, "/api/assets?sort=nope", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "validation_failed" {
		t.Fatalf("error.code = %q", code)
	}
}

func TestCategoriesEndpoint(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	addAsset(t, libDir, "Props", "Table", true)
	addAsset(t, libDir, "Characters", "Hero", false)
	rescan(t, srv)

	rec := doRequest(t, srv, http.MethodGet, "/api/categories", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got []index.CategoryCount
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := []index.CategoryCount{{Name: "Characters", Count: 1}, {Name: "Props", Count: 2}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("categories = %+v, want %+v", got, want)
	}
}

func TestAssetsAndScanWrongMethodReturns405(t *testing.T) {
	srv, _ := newLibraryServer(t)
	for _, req := range []struct{ method, path string }{
		{http.MethodDelete, "/api/assets"},
		{http.MethodGet, "/api/scan"},
	} {
		rec := doRequest(t, srv, req.method, req.path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", req.method, req.path, rec.Code)
		}
	}
}
