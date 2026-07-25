package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
)

func newBrowseServer(t *testing.T) (*Server, string) {
	t.Helper()
	srv, libDir := newLibraryServer(t)
	assetDir := filepath.Join(libDir, "source", "Props", "Chair")
	writeFileIn(t, assetDir, "model.blend", "blend")
	writeFileIn(t, assetDir, "notes.md", "# メモ\n\n**太字**")
	writeFileIn(t, assetDir, "textures/wood.png", "png-wood")
	writeFileIn(t, assetDir, "textures/metal.jpg", "jpg-metal")
	rescan(t, srv)
	return srv, libDir
}

func TestAssetDirListing(t *testing.T) {
	srv, _ := newBrowseServer(t)

	rec := doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/dir/textures", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var files []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Name != "metal.jpg" || files[1].Name != "wood.png" {
		t.Fatalf("files = %+v(名前順)", files)
	}
	if files[1].Size != int64(len("png-wood")) {
		t.Errorf("size = %d", files[1].Size)
	}
}

func TestAssetDirListingHidesOSMetadata(t *testing.T) {
	// Finder が textures/ を開くと .DS_Store が生まれる。利用者からは
	// 不可視のファイルなので、アプリの一覧にも出さない
	srv, libDir := newBrowseServer(t)
	writeFileIn(t, filepath.Join(libDir, "source", "Props", "Chair"), "textures/.DS_Store", "junk")

	rec := doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/dir/textures", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var files []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %+v, want 2 (.DS_Store を除く)", files)
	}
	for _, f := range files {
		if f.Name == ".DS_Store" {
			t.Error(".DS_Store should not be listed")
		}
	}
}

func TestAssetDirListingMissingDirIsEmpty(t *testing.T) {
	srv, _ := newBrowseServer(t)
	// references/ が無いアセット(Finder 作成)でも空一覧で返す
	rec := doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/dir/references", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "[]\n" {
		t.Fatalf("body = %q, want empty array", rec.Body.String())
	}
}

func TestAssetDirListingRejectsTraversal(t *testing.T) {
	srv, _ := newBrowseServer(t)
	rec := doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/dir/..", "")
	if rec.Code == http.StatusOK {
		t.Fatalf("traversal must not succeed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAssetRawServing(t *testing.T) {
	srv, _ := newBrowseServer(t)

	rec := doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/raw/textures/wood.png", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "png-wood" {
		t.Fatalf("raw = %d %q", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/raw/notes.md", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "# メモ\n\n**太字**" {
		t.Fatalf("notes = %d %q", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/raw/ghost.png", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing = %d, want 404", rec.Code)
	}
	// ディレクトリ自体は配信しない
	rec = doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/raw/textures", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("dir = %d, want 404", rec.Code)
	}
}

func TestRevealInFileManager(t *testing.T) {
	srv, libDir := newBrowseServer(t)

	var mu sync.Mutex
	var gotName string
	var gotArgs []string
	orig := execStart
	execStart = func(name string, args ...string) error {
		mu.Lock()
		defer mu.Unlock()
		gotName = name
		gotArgs = args
		return nil
	}
	t.Cleanup(func() { execStart = orig })

	rec := doRequest(t, srv, http.MethodPost, "/api/assets/Props/Chair/reveal", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	wantDir := filepath.Join(libDir, "source", "Props", "Chair")
	if len(gotArgs) != 1 || gotArgs[0] != wantDir {
		t.Fatalf("launched %s %v, want dir %s", gotName, gotArgs, wantDir)
	}
}

func TestRevealCommandPerOS(t *testing.T) {
	for goos, want := range map[string]string{
		"darwin":  "open",     // macOS: Finder
		"windows": "explorer", // Windows: エクスプローラー
		"linux":   "xdg-open",
	} {
		name, args := revealCommand(goos, "/lib/source/Props/Chair")
		if name != want || len(args) != 1 || args[0] != "/lib/source/Props/Chair" {
			t.Errorf("%s: %s %v", goos, name, args)
		}
	}
}
