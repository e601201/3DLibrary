package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/e601201/3DLibrary/internal/generate"
	"github.com/e601201/3DLibrary/internal/library"
)

// runner_test.go の fakeBlender と同じ規約のダミー Blender を設定に登録する。
func installFakeBlender(t *testing.T, srv *Server, libDir string) {
	t.Helper()
	script := filepath.Join(t.TempDir(), "fake-blender.sh")
	content := `#!/bin/sh
while [ $# -gt 0 ]; do
  case "$1" in
    --glb) GLB="$2"; shift 2 ;;
    --thumb) THUMB="$2"; shift 2 ;;
    --meta) META="$2"; shift 2 ;;
    --sprite) SPRITE="$2"; shift 2 ;;
    *) shift ;;
  esac
done
echo "glb" > "$GLB"
echo "png-bytes" > "$THUMB"
echo "webp-bytes" > "$SPRITE"
echo '{"objectCount":1,"collectionCount":1,"materialCount":0,"polygonCount":42,"textureCount":0,"hasAnimation":false}' > "$META"
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{
		"blenderPath": script, "libraryDir": libDir, "thumbnailSize": 512, "theme": "system",
	})
	if rec := doRequest(t, srv, http.MethodPut, "/api/config", string(body)); rec.Code != http.StatusOK {
		t.Fatalf("PUT config = %d: %s", rec.Code, rec.Body.String())
	}
}

func jobStatus(t *testing.T, srv http.Handler) generate.Status {
	t.Helper()
	rec := doRequest(t, srv, http.MethodGet, "/api/jobs", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/jobs = %d", rec.Code)
	}
	var s generate.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func waitQueueIdle(t *testing.T, srv http.Handler) generate.Status {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s := jobStatus(t, srv)
		if s.Running == nil && s.PendingCount == 0 {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("queue did not become idle")
	return generate.Status{}
}

func TestGenerateJobProducesCacheAndUpdatesIndex(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	installFakeBlender(t, srv, libDir)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	rec := doRequest(t, srv, http.MethodPost, "/api/jobs", `{"category":"Props","title":"Chair"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/jobs = %d: %s", rec.Code, rec.Body.String())
	}
	s := waitQueueIdle(t, srv)
	if s.LastError != nil {
		t.Fatalf("lastError = %+v", s.LastError)
	}

	// キャッシュ 4 点が生成されている
	paths := library.CachePaths(libDir, "Props", "Chair")
	for _, p := range paths.All() {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s", p)
		}
	}

	// onDone の再スキャンでインデックスに反映されている(手動スキャン不要)
	deadline := time.Now().Add(5 * time.Second)
	for {
		assets := listAssets(t, srv)
		a := assets[0]
		if a.ThumbnailPath != nil && a.GlbPath != nil && a.SpritePath != nil &&
			a.PolygonCount != nil && *a.PolygonCount == 42 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("index not updated: %+v", a)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestGenerateJobValidation(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	installFakeBlender(t, srv, libDir)
	addAsset(t, libDir, "Props", "NoBlend", false)
	rescan(t, srv)

	rec := doRequest(t, srv, http.MethodPost, "/api/jobs", `{"category":"Props","title":"Ghost"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown asset = %d, want 404: %s", rec.Code, rec.Body.String())
	}

	rec = doRequest(t, srv, http.MethodPost, "/api/jobs", `{"category":"Props","title":"NoBlend"}`)
	if rec.Code != http.StatusConflict {
		t.Errorf("incomplete asset = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "asset_incomplete" {
		t.Errorf("error.code = %q", code)
	}
}

func TestGenerateJobWithoutBlenderRecordsError(t *testing.T) {
	srv, libDir := newLibraryServer(t) // blenderPath 未設定
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	rec := doRequest(t, srv, http.MethodPost, "/api/jobs", `{"category":"Props","title":"Chair"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST = %d", rec.Code)
	}
	s := waitQueueIdle(t, srv)
	if s.LastError == nil {
		t.Fatal("lastError should be set when blender is not configured")
	}
}

func TestBulkGenerateTargetsMissingAndStaleOnly(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	installFakeBlender(t, srv, libDir)
	addAsset(t, libDir, "Props", "Fresh", true)   // キャッシュ最新
	addAsset(t, libDir, "Props", "Missing", true) // 未生成
	addAsset(t, libDir, "Props", "Stale", true)   // 陳腐化
	addAsset(t, libDir, "Props", "NoBlend", false)

	// Fresh: blend より新しいキャッシュ / Stale: blend より古いキャッシュ
	now := time.Now()
	for title, mtime := range map[string]time.Time{
		"Fresh": now.Add(time.Hour),
		"Stale": now.Add(-time.Hour),
	} {
		paths := library.CachePaths(libDir, "Props", title)
		for _, p := range paths.All() {
			writeFileIn(t, libDir, p[len(libDir)+1:], `{"polygonCount":1}`)
			if err := os.Chtimes(p, mtime, mtime); err != nil {
				t.Fatal(err)
			}
		}
	}
	rescan(t, srv)

	freshThumb := library.CachePaths(libDir, "Props", "Fresh").Thumbnail
	freshBefore, _ := os.Stat(freshThumb)

	rec := doRequest(t, srv, http.MethodPost, "/api/jobs/bulk", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Enqueued int             `json:"enqueued"`
		Status   generate.Status `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Enqueued != 2 {
		t.Fatalf("enqueued = %d, want 2(Missing と Stale のみ)", resp.Enqueued)
	}
	waitQueueIdle(t, srv)

	// Fresh はスキップされた(キャッシュが触られていない)
	freshAfter, _ := os.Stat(freshThumb)
	if !freshAfter.ModTime().Equal(freshBefore.ModTime()) {
		t.Error("fresh asset must be skipped")
	}
	// Stale は再生成されて要更新が消える
	for _, a := range listAssets(t, srv) {
		if a.Title == "Stale" && a.IsStale {
			t.Error("Stale should be regenerated and no longer stale")
		}
		if a.Title == "Missing" && a.ThumbnailPath == nil {
			t.Error("Missing should be generated")
		}
	}
}

// TestBulkGenerateTargetsMissingSprite はスプライト導入前に生成された
// アセット(他の 3 点は最新)が一括生成で埋まることを確かめる(ADR-0003)。
func TestBulkGenerateTargetsMissingSprite(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	installFakeBlender(t, srv, libDir)
	addAsset(t, libDir, "Props", "NoSprite", true)

	fresh := time.Now().Add(time.Hour)
	paths := library.CachePaths(libDir, "Props", "NoSprite")
	for _, p := range []string{paths.GLB, paths.Thumbnail, paths.Metadata} {
		writeFileIn(t, libDir, p[len(libDir)+1:], `{"polygonCount":1}`)
		if err := os.Chtimes(p, fresh, fresh); err != nil {
			t.Fatal(err)
		}
	}
	rescan(t, srv)

	for _, a := range listAssets(t, srv) {
		if a.Title == "NoSprite" && !a.IsStale {
			t.Error("asset without a sprite should be stale")
		}
	}

	rec := doRequest(t, srv, http.MethodPost, "/api/jobs/bulk", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Enqueued int `json:"enqueued"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Enqueued != 1 {
		t.Fatalf("enqueued = %d, want 1", resp.Enqueued)
	}
	waitQueueIdle(t, srv)
	if _, err := os.Stat(paths.Sprite); err != nil {
		t.Errorf("sprite should be generated: %v", err)
	}
}

func TestBulkGenerateWithNothingToDo(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	installFakeBlender(t, srv, libDir)
	rescan(t, srv)
	rec := doRequest(t, srv, http.MethodPost, "/api/jobs/bulk", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct {
		Enqueued int `json:"enqueued"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Enqueued != 0 {
		t.Fatalf("enqueued = %d, want 0", resp.Enqueued)
	}
}

func TestThumbnailServing(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	paths := library.CachePaths(libDir, "Props", "Chair")
	if err := os.MkdirAll(filepath.Dir(paths.Thumbnail), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Thumbnail, []byte("png-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/thumbnails/Props/Chair.png", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "png-bytes" {
		t.Fatalf("body = %q", rec.Body.String())
	}

	// パストラバーサルでファイルが読めないこと(ServeMux が .. を
	// リダイレクトで無害化するか、ハンドラの封じ込めで 404 になる)
	rec = doRequest(t, srv, http.MethodGet, "/api/thumbnails/../../database.db", "")
	if rec.Code == http.StatusOK {
		t.Fatalf("traversal must not serve content: %d", rec.Code)
	}
	rec = doRequest(t, srv, http.MethodGet, "/api/thumbnails/Props/Ghost.png", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing = %d, want 404", rec.Code)
	}
}

// TestAssetListExposesSpritePath は一覧 JSON でスプライトの有無が
// 有 / null として見えることを HTTP のシームで確かめる。
func TestAssetListExposesSpritePath(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "WithSprite", true)
	addAsset(t, libDir, "Props", "WithoutSprite", true)
	paths := library.CachePaths(libDir, "Props", "WithSprite")
	writeFileIn(t, libDir, paths.Sprite[len(libDir)+1:], "webp")
	rescan(t, srv)

	got := map[string]*string{}
	for _, a := range listAssets(t, srv) {
		got[a.Title] = a.SpritePath
	}
	if got["WithSprite"] == nil || *got["WithSprite"] != paths.Sprite {
		t.Errorf("WithSprite spritePath = %v, want %q", got["WithSprite"], paths.Sprite)
	}
	if got["WithoutSprite"] != nil {
		t.Errorf("WithoutSprite spritePath = %v, want null", got["WithoutSprite"])
	}
}

func TestSpriteServing(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	paths := library.CachePaths(libDir, "Props", "Chair")
	if err := os.MkdirAll(filepath.Dir(paths.Sprite), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Sprite, []byte("webp-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/sprites/Props/Chair.webp", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "webp-bytes" {
		t.Fatalf("body = %q", rec.Body.String())
	}

	rec = doRequest(t, srv, http.MethodGet, "/api/sprites/../../database.db", "")
	if rec.Code == http.StatusOK {
		t.Fatalf("traversal must not serve content: %d", rec.Code)
	}
	rec = doRequest(t, srv, http.MethodGet, "/api/sprites/Props/Ghost.webp", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing = %d, want 404", rec.Code)
	}
}
