package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFileIn(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAssetFilesListing(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	assetDir := filepath.Join(libDir, "source", "Props", "Chair")
	writeFileIn(t, assetDir, "model.blend", "0123456789")
	writeFileIn(t, assetDir, "notes.md", "note")
	writeFileIn(t, assetDir, "textures/wood.png", "abc")
	writeFileIn(t, assetDir, "textures/metal.png", "de")
	rescan(t, srv)

	// GLB キャッシュあり
	writeFileIn(t, libDir, "cache/glb/Props/Chair.glb", "glbdata")

	rec := doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/files", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Entries []struct {
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			IsDir     bool   `json:"isDir"`
			FileCount int    `json:"fileCount"`
		} `json:"entries"`
		GlbSize *int64 `json:"glbSize"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	byName := map[string]struct {
		Size      int64
		IsDir     bool
		FileCount int
	}{}
	for _, e := range got.Entries {
		byName[e.Name] = struct {
			Size      int64
			IsDir     bool
			FileCount int
		}{e.Size, e.IsDir, e.FileCount}
	}
	if f := byName["model.blend"]; f.Size != 10 || f.IsDir {
		t.Errorf("model.blend = %+v", f)
	}
	if f := byName["textures"]; !f.IsDir || f.FileCount != 2 || f.Size != 5 {
		t.Errorf("textures = %+v(再帰サイズ・件数)", f)
	}
	if got.GlbSize == nil || *got.GlbSize != 7 {
		t.Errorf("glbSize = %v, want 7", got.GlbSize)
	}
}

func TestAssetFilesHidesOSMetadata(t *testing.T) {
	// .DS_Store は一覧に出さないだけでなく、サブディレクトリの
	// 件数・サイズにも数えない(空の textures が「1 ファイル」に見えてしまう)
	srv, libDir := newLibraryServer(t)
	assetDir := filepath.Join(libDir, "source", "Props", "Chair")
	writeFileIn(t, assetDir, "model.blend", "0123456789")
	writeFileIn(t, assetDir, ".DS_Store", "junk")
	writeFileIn(t, assetDir, "textures/.DS_Store", "junk")
	rescan(t, srv)

	rec := doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/files", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Entries []struct {
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			IsDir     bool   `json:"isDir"`
			FileCount int    `json:"fileCount"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for _, e := range got.Entries {
		if e.Name == ".DS_Store" {
			t.Error(".DS_Store should not be listed")
		}
		if e.Name == "textures" && (e.FileCount != 0 || e.Size != 0) {
			t.Errorf("textures = %+v, want empty(.DS_Store を数えない)", e)
		}
	}
}

func TestAssetFilesNotFound(t *testing.T) {
	srv, _ := newLibraryServer(t)
	rec := doRequest(t, srv, http.MethodGet, "/api/assets/Props/Ghost/files", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestOpenInBlender(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	// 起動された引数を記録するダミー Blender
	marker := filepath.Join(t.TempDir(), "opened.txt")
	script := filepath.Join(t.TempDir(), "fake-blender.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$1\" > "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{
		"blenderPath": script, "libraryDir": libDir, "thumbnailSize": 512, "theme": "system",
	})
	if rec := doRequest(t, srv, http.MethodPut, "/api/config", string(body)); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}

	rec := doRequest(t, srv, http.MethodPost, "/api/assets/Props/Chair/open", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		if b, err := os.ReadFile(marker); err == nil {
			want := filepath.Join(libDir, "source", "Props", "Chair", "model.blend")
			if got := string(b); got != want+"\n" {
				t.Fatalf("blender launched with %q, want %q", got, want)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("blender was not launched")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestOpenInBlenderValidation(t *testing.T) {
	srv, libDir := newLibraryServer(t) // blenderPath 未設定
	addAsset(t, libDir, "Props", "Chair", true)
	addAsset(t, libDir, "Props", "NoBlend", false)
	rescan(t, srv)

	rec := doRequest(t, srv, http.MethodPost, "/api/assets/Props/Chair/open", "")
	if rec.Code != http.StatusConflict {
		t.Errorf("no blender = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "blender_not_configured" {
		t.Errorf("error.code = %q", code)
	}

	rec = doRequest(t, srv, http.MethodPost, "/api/assets/Props/NoBlend/open", "")
	if rec.Code != http.StatusConflict {
		t.Errorf("incomplete = %d, want 409", rec.Code)
	}
	rec = doRequest(t, srv, http.MethodPost, "/api/assets/Props/Ghost/open", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown = %d, want 404", rec.Code)
	}
}

func TestDetailRoutesWrongMethodReturns405(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	for _, req := range []struct{ method, path string }{
		{http.MethodPost, "/api/assets/Props/Chair/files"},
		{http.MethodGet, "/api/assets/Props/Chair/open"},
	} {
		rec := doRequest(t, srv, req.method, req.path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", req.method, req.path, rec.Code)
		}
		if code := errorCode(t, rec); code != "method_not_allowed" {
			t.Errorf("%s %s error.code = %q", req.method, req.path, code)
		}
	}
}

func TestGlbAndMetadataServing(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	writeFileIn(t, libDir, "cache/glb/Props/Chair.glb", "glb-bytes")
	writeFileIn(t, libDir, "cache/metadata/Props/Chair.json", `{"polygonCount":6}`)

	rec := doRequest(t, srv, http.MethodGet, "/api/glb/Props/Chair.glb", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "glb-bytes" {
		t.Fatalf("glb = %d %q", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, srv, http.MethodGet, "/api/extracted-metadata/Props/Chair.json", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("metadata = %d", rec.Code)
	}
	rec = doRequest(t, srv, http.MethodGet, "/api/glb/Props/Ghost.glb", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing glb = %d, want 404", rec.Code)
	}
}
