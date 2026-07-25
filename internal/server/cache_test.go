package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/e601201/3DLibrary/internal/library"
)

func TestCacheSizeEndpoint(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	writeFileIn(t, libDir, "cache/glb/Props/Chair.glb", "0123456789")
	writeFileIn(t, libDir, "cache/thumbnails/Props/Chair.png", "01234")

	rec := doRequest(t, srv, http.MethodGet, "/api/cache", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var info struct {
		SizeBytes int64 `json:"sizeBytes"`
		FileCount int   `json:"fileCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.SizeBytes != 15 || info.FileCount != 2 {
		t.Fatalf("info = %+v, want 15 bytes / 2 files", info)
	}
}

func TestCacheDeleteClearsAndResyncsIndex(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	installFakeBlender(t, srv, libDir)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	// 生成してキャッシュとインデックス参照を作る
	doRequest(t, srv, http.MethodPost, "/api/jobs", `{"category":"Props","title":"Chair"}`)
	waitQueueIdle(t, srv)
	if a := listAssets(t, srv)[0]; a.ThumbnailPath == nil {
		t.Fatal("precondition: cache should exist")
	}

	rec := doRequest(t, srv, http.MethodDelete, "/api/cache", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d: %s", rec.Code, rec.Body.String())
	}

	// インデックスのキャッシュ参照が消えている(プレースホルダー状態)
	a := listAssets(t, srv)[0]
	if a.ThumbnailPath != nil || a.GlbPath != nil || a.PolygonCount != nil {
		t.Fatalf("cache refs should be cleared: %+v", a)
	}
	if a.IsStale {
		t.Error("未生成は陳腐化ではない")
	}
	// source は無傷
	if _, err := os.Stat(filepath.Join(libDir, "source", "Props", "Chair", "model.blend")); err != nil {
		t.Errorf("source touched: %v", err)
	}
	// 容量は 0 になる
	size, count, err := library.CacheSize(libDir)
	if err != nil || size != 0 || count != 0 {
		t.Errorf("cache not empty: %d bytes / %d files / %v", size, count, err)
	}

	// 一括生成で復旧できる
	rec = doRequest(t, srv, http.MethodPost, "/api/jobs/bulk", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("bulk = %d", rec.Code)
	}
	waitQueueIdle(t, srv)
	if a := listAssets(t, srv)[0]; a.ThumbnailPath == nil || a.PolygonCount == nil {
		t.Fatalf("bulk generate should restore cache: %+v", a)
	}
}

func TestCacheWrongMethodReturns405(t *testing.T) {
	srv, _ := newLibraryServer(t)
	rec := doRequest(t, srv, http.MethodPost, "/api/cache", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if code := errorCode(t, rec); code != "method_not_allowed" {
		t.Fatalf("error.code = %q", code)
	}
}
