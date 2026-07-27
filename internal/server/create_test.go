package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func addTemplate(t *testing.T, libDir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(libDir, "templates", name), []byte("tmpl-"+name), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGetTemplates(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addTemplate(t, libDir, "empty.blend")
	addTemplate(t, libDir, "studio.blend")

	rec := doRequest(t, srv, http.MethodGet, "/api/templates", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "[\"empty.blend\",\"studio.blend\"]\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestCreateAssetAppearsInListImmediately(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addTemplate(t, libDir, "empty.blend")

	rec := doRequest(t, srv, http.MethodPost, "/api/assets",
		`{"title":"Wooden Chair","category":"Props","template":"empty.blend"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	// 作成直後に(手動スキャンなしで)一覧へ反映される
	assets := listAssets(t, srv)
	if len(assets) != 1 || assets[0].Title != "Wooden Chair" || assets[0].IsIncomplete {
		t.Fatalf("assets = %+v", assets)
	}
	if _, err := os.Stat(filepath.Join(libDir, "source", "Props", "Wooden Chair", "meta.json")); err != nil {
		t.Errorf("meta.json: %v", err)
	}
}

func TestCreateAssetWithTags(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addTemplate(t, libDir, "empty.blend")

	rec := doRequest(t, srv, http.MethodPost, "/api/assets",
		`{"title":"Chair","category":"Props","template":"empty.blend","tags":[" Wood ","","Seating","Wood"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	// レスポンスは PUT .../tags と同じく正規化後のタグを返す
	var got struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "Wood" || got.Tags[1] != "Seating" {
		t.Fatalf("response tags = %v, want [Wood Seating]", got.Tags)
	}

	// 作成後の再スキャンでインデックスにも入る(1 リクエストで完結する)
	assets := listAssets(t, srv)
	if len(assets) != 1 {
		t.Fatalf("assets = %+v", assets)
	}
	if len(assets[0].Tags) != 2 || assets[0].Tags[0].Name != "Wood" {
		t.Fatalf("indexed tags = %v, want [Wood Seating]", assets[0].Tags)
	}
}

func TestCreateAssetWithoutTagsStaysEmpty(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addTemplate(t, libDir, "empty.blend")

	rec := doRequest(t, srv, http.MethodPost, "/api/assets",
		`{"title":"Chair","category":"Props","template":"empty.blend"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if assets := listAssets(t, srv); len(assets) != 1 || len(assets[0].Tags) != 0 {
		t.Fatalf("assets = %+v, want one asset with no tags", assets)
	}
}

func TestCreateAssetValidationFailures(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addTemplate(t, libDir, "empty.blend")

	for name, body := range map[string]string{
		"empty title":      `{"title":"","category":"Props","template":"empty.blend"}`,
		"traversal title":  `{"title":"../evil","category":"Props","template":"empty.blend"}`,
		"missing template": `{"title":"Chair","category":"Props","template":"nope.blend"}`,
		"broken JSON":      `{`,
	} {
		rec := doRequest(t, srv, http.MethodPost, "/api/assets", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400: %s", name, rec.Code, rec.Body.String())
		}
		if code := errorCode(t, rec); code != "validation_failed" {
			t.Errorf("%s: error.code = %q", name, code)
		}
	}
}

func TestCreateAssetDuplicateReturns409(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addTemplate(t, libDir, "empty.blend")
	body := `{"title":"Chair","category":"Props","template":"empty.blend"}`

	if rec := doRequest(t, srv, http.MethodPost, "/api/assets", body); rec.Code != http.StatusCreated {
		t.Fatalf("first create = %d", rec.Code)
	}
	rec := doRequest(t, srv, http.MethodPost, "/api/assets", body)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "asset_already_exists" {
		t.Fatalf("error.code = %q", code)
	}
}

func TestTemplatesWrongMethodReturns405(t *testing.T) {
	srv, _ := newLibraryServer(t)
	rec := doRequest(t, srv, http.MethodPost, "/api/templates", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
