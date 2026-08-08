package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setPrivate(t *testing.T, srv http.Handler, category, title string, private bool) {
	t.Helper()
	body := `{"private":false}`
	if private {
		body = `{"private":true}`
	}
	rec := doRequest(t, srv, http.MethodPut, "/api/assets/"+category+"/"+title+"/private", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT private %s/%s = %d: %s", category, title, rec.Code, rec.Body.String())
	}
}

func TestPutPrivateWritesMetaJSONAndProjectsToIndex(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)

	rec := doRequest(t, srv, http.MethodPut, "/api/assets/Props/Chair/private", `{"private":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "{\"private\":true}\n" {
		t.Errorf("body = %q", got)
	}

	// meta.json に保存されている(ADR-0001: ユーザーデータは meta.json へ)
	b, err := os.ReadFile(filepath.Join(libDir, "source", "Props", "Chair", "meta.json"))
	if err != nil {
		t.Fatalf("meta.json: %v", err)
	}
	var meta struct {
		Private bool `json:"private"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatal(err)
	}
	if !meta.Private {
		t.Fatalf("meta.json = %s(private が保存されていない)", b)
	}

	// 再スキャン無しでインデックスに射影済み
	assets := listAssets(t, srv)
	if len(assets) != 1 || !assets[0].IsPrivate {
		t.Fatalf("assets = %+v(isPrivate が射影されていない)", assets)
	}

	// 公開へ戻す
	setPrivate(t, srv, "Props", "Chair", false)
	if assets := listAssets(t, srv); assets[0].IsPrivate {
		t.Fatal("asset should be public again")
	}
}

func TestPutPrivateUnknownAssetReturns404(t *testing.T) {
	srv, _ := newLibraryServer(t)
	rec := doRequest(t, srv, http.MethodPut, "/api/assets/Props/Ghost/private", `{"private":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestPutPrivateInvalidBodyReturns400(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)
	rec := doRequest(t, srv, http.MethodPut, "/api/assets/Props/Chair/private", `not json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if code := errorCode(t, rec); code != "validation_failed" {
		t.Errorf("error.code = %q", code)
	}
}

func TestPrivateRouteWrongMethodReturns405(t *testing.T) {
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	rescan(t, srv)
	rec := doRequest(t, srv, http.MethodGet, "/api/assets/Props/Chair/private", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// newPrivateFixture は公開 Chair / 非公開 Draft(キャッシュ 4 点あり)/
// 非公開だけのカテゴリ Experiments を持つライブラリを組む。
func newPrivateFixture(t *testing.T) (*Server, string) {
	t.Helper()
	srv, libDir := newLibraryServer(t)
	addAsset(t, libDir, "Props", "Chair", true)
	addAsset(t, libDir, "Props", "Draft", true)
	addAsset(t, libDir, "Experiments", "Secret", true)
	writeFileIn(t, libDir, "cache/glb/Props/Draft.glb", "glb-bytes")
	writeFileIn(t, libDir, "cache/thumbnails/Props/Draft.png", "png-bytes")
	writeFileIn(t, libDir, "cache/sprites/Props/Draft.webp", "webp-bytes")
	writeFileIn(t, libDir, "cache/metadata/Props/Draft.json", `{"polygonCount":1}`)
	rescan(t, srv)
	doRequest(t, srv, http.MethodPut, "/api/assets/Props/Chair/tags", `{"tags":["wood"]}`)
	doRequest(t, srv, http.MethodPut, "/api/assets/Props/Draft/tags", `{"tags":["wood","wip"]}`)
	setPrivate(t, srv, "Props", "Draft", true)
	setPrivate(t, srv, "Experiments", "Secret", true)
	return srv, libDir
}

func TestRemoteViewingHidesPrivateFromListAndSearch(t *testing.T) {
	srv, _ := newPrivateFixture(t)
	remote := srv.RemoteViewingHandler()

	// ローカルは全件(非公開も普通に出る)
	if assets := listAssets(t, srv); len(assets) != 3 {
		t.Fatalf("local assets = %d, want 3", len(assets))
	}
	// リモートには公開だけ
	assets := listAssets(t, remote)
	if len(assets) != 1 || assets[0].Title != "Chair" {
		t.Fatalf("remote assets = %+v, want only Chair", assets)
	}

	// 検索 q でも出ない
	rec := doRequest(t, remote, http.MethodGet, "/api/assets?q=Draft", "")
	var hits []struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("remote q=Draft = %+v, want none", hits)
	}
}

func TestRemoteViewingHidesPrivateFromTagAndCategoryCounts(t *testing.T) {
	srv, _ := newPrivateFixture(t)
	remote := srv.RemoteViewingHandler()

	// タグ集計: wood は公開分だけ、非公開にしか付いていない wip は名前ごと消える
	rec := doRequest(t, remote, http.MethodGet, "/api/tags", "")
	if got := rec.Body.String(); got != "[{\"name\":\"wood\",\"count\":1}]\n" {
		t.Errorf("remote tags = %q", got)
	}
	// ローカルは全部見える
	rec = doRequest(t, srv, http.MethodGet, "/api/tags", "")
	if got := rec.Body.String(); got != "[{\"name\":\"wip\",\"count\":1},{\"name\":\"wood\",\"count\":2}]\n" {
		t.Errorf("local tags = %q", got)
	}

	// カテゴリ集計: 非公開しか居ない Experiments は名前ごと消える
	rec = doRequest(t, remote, http.MethodGet, "/api/categories", "")
	if got := rec.Body.String(); got != "[{\"name\":\"Props\",\"count\":1}]\n" {
		t.Errorf("remote categories = %q", got)
	}
}

func TestRemoteViewingPrivateAssetDoesNotExist(t *testing.T) {
	srv, _ := newPrivateFixture(t)
	remote := srv.RemoteViewingHandler()

	// 詳細・ファイル一覧・raw・キャッシュ 4 点の直接 GET まで一貫して 404
	// (直接 URL・ブラウザ履歴からの半端な漏れを残さない)
	for _, path := range []string{
		"/api/assets/Props/Draft/files",
		"/api/assets/Props/Draft/dir/textures",
		"/api/assets/Props/Draft/raw/model.blend",
		"/api/glb/Props/Draft.glb",
		"/api/thumbnails/Props/Draft.png",
		"/api/sprites/Props/Draft.webp",
		"/api/extracted-metadata/Props/Draft.json",
	} {
		rec := doRequest(t, remote, http.MethodGet, path, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("remote GET %s = %d, want 404", path, rec.Code)
			continue
		}
		if code := errorCode(t, rec); code != "not_found" {
			t.Errorf("remote GET %s error.code = %q", path, code)
		}
	}

	// ローカルでは同じ URL がそのまま見える
	for _, path := range []string{
		"/api/assets/Props/Draft/files",
		"/api/assets/Props/Draft/raw/model.blend",
		"/api/glb/Props/Draft.glb",
		"/api/thumbnails/Props/Draft.png",
		"/api/sprites/Props/Draft.webp",
		"/api/extracted-metadata/Props/Draft.json",
	} {
		if rec := doRequest(t, srv, http.MethodGet, path, ""); rec.Code != http.StatusOK {
			t.Errorf("local GET %s = %d, want 200: %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestRemoteViewingPrivate404MatchesMissingAsset(t *testing.T) {
	// 404 の応答は実在しないアセットと同文(存在の有無を推測させない)
	srv, _ := newPrivateFixture(t)
	remote := srv.RemoteViewingHandler()

	private := doRequest(t, remote, http.MethodGet, "/api/assets/Props/Draft/files", "")
	missing := doRequest(t, remote, http.MethodGet, "/api/assets/Props/Ghost/files", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing asset = %d, want 404", missing.Code)
	}
	privateBody := strings.ReplaceAll(private.Body.String(), "Draft", "X")
	missingBody := strings.ReplaceAll(missing.Body.String(), "Ghost", "X")
	if privateBody != missingBody {
		t.Errorf("bodies differ: private=%q missing=%q", private.Body.String(), missing.Body.String())
	}
}
