package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestPutTagsWritesMetaJSONAndProjectsToIndex(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	rec := doRequest(t, srv, http.MethodPut, "/api/assets/Props/Chair/tags",
		`{"tags":[" wood ","furniture","wood",""]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "{\"tags\":[\"wood\",\"furniture\"]}\n" {
		t.Errorf("body = %q(正規化されて返る)", got)
	}

	// meta.json が作られている(初回タグ付け)
	b, err := os.ReadFile(filepath.Join(libDir, "source", "Props", "Chair", "meta.json"))
	if err != nil {
		t.Fatalf("meta.json: %v", err)
	}
	var meta struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatal(err)
	}
	if len(meta.Tags) != 2 || meta.Tags[0] != "wood" {
		t.Fatalf("meta.json tags = %v", meta.Tags)
	}

	// 再スキャン無しでインデックスに射影済み
	assets := listAssets(t, srv)
	if len(assets[0].Tags) != 2 {
		t.Fatalf("index tags = %+v", assets[0].Tags)
	}
}

func TestTagsSurviveIndexRebuild(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)
	doRequest(t, srv, http.MethodPut, "/api/assets/Props/Chair/tags", `{"tags":["wood"]}`)

	// インデックスを消して再スキャン → meta.json から復元される
	srv.CloseLibrary()
	if err := os.Remove(filepath.Join(libDir, "database.db")); err != nil {
		t.Fatal(err)
	}
	rescan(t, srv)
	assets := listAssets(t, srv)
	if len(assets) != 1 || len(assets[0].Tags) != 1 || assets[0].Tags[0].Name != "wood" {
		t.Fatalf("assets = %+v(タグが復元されていない)", assets)
	}
}

func TestTagFilterAndTagList(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	addAsset(t, libDir, "Props", "Table", true)
	rescan(t, srv)
	doRequest(t, srv, http.MethodPut, "/api/assets/Props/Chair/tags", `{"tags":["wood"]}`)

	// タグ絞り込み
	rec := doRequest(t, srv, http.MethodGet, "/api/assets?tag=wood", "")
	var assets []struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &assets); err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Title != "Chair" {
		t.Fatalf("assets = %+v", assets)
	}

	// タグ検索(q がタグにも効く)
	rec = doRequest(t, srv, http.MethodGet, "/api/assets?q=woo", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &assets); err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Title != "Chair" {
		t.Fatalf("q=woo assets = %+v", assets)
	}

	// タグ一覧
	rec = doRequest(t, srv, http.MethodGet, "/api/tags", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/tags = %d", rec.Code)
	}
	if got := rec.Body.String(); got != "[{\"name\":\"wood\",\"count\":1}]\n" {
		t.Fatalf("tags = %q", got)
	}
}

func TestPutTagsUnknownAssetReturns404(t *testing.T) {
	srv, _ := newLibraryServer(t)
	rec := doRequest(t, srv, http.MethodPut, "/api/assets/Props/Ghost/tags", `{"tags":["x"]}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestTagRoutesWrongMethodReturns405(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)
	for _, req := range []struct{ method, path string }{
		{http.MethodPost, "/api/assets/Props/Chair/tags"},
		{http.MethodPost, "/api/tags"},
	} {
		rec := doRequest(t, srv, req.method, req.path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", req.method, req.path, rec.Code)
		}
	}
}
