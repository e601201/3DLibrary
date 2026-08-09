package library

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func readTags(t *testing.T, dir, category, title string) []string {
	t.Helper()
	meta, err := ReadMeta(dir, category, title)
	if err != nil {
		t.Fatalf("ReadMeta: %v", err)
	}
	return meta.Tags
}

func TestReadMetaMissingFileMeansPublicNoTags(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	// meta.json を消しても「タグ空・公開」として扱う
	if err := os.Remove(filepath.Join(dir, "source", "Props", "Chair", "meta.json")); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(dir, "Props", "Chair")
	if err != nil {
		t.Fatalf("ReadMeta: %v", err)
	}
	if len(meta.Tags) != 0 {
		t.Fatalf("tags = %v, want empty", meta.Tags)
	}
	if meta.Private {
		t.Fatal("missing meta.json should mean public")
	}
}

func TestWriteTagsCreatesMetaJSONOnFirstTagging(t *testing.T) {
	dir := newLibrary(t)
	// Finder で作られたアセット相当(meta.json なし)
	assetDir := filepath.Join(dir, "source", "Props", "Chair")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := WriteTags(dir, "Props", "Chair", []string{"wood", "furniture"}); err != nil {
		t.Fatalf("WriteTags: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(assetDir, "meta.json"))
	if err != nil {
		t.Fatalf("meta.json should be created: %v", err)
	}
	if string(b) != "{\n  \"tags\": [\n    \"wood\",\n    \"furniture\"\n  ]\n}\n" {
		t.Errorf("meta.json = %q", b)
	}

	tags := readTags(t, dir, "Props", "Chair")
	if len(tags) != 2 || tags[0] != "wood" || tags[1] != "furniture" {
		t.Fatalf("roundtrip tags = %v", tags)
	}
}

func TestWriteTagsEmptyListWritesEmptyArray(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteTags(dir, "Props", "Chair", nil); err != nil {
		t.Fatal(err)
	}
	if tags := readTags(t, dir, "Props", "Chair"); len(tags) != 0 {
		t.Fatalf("tags = %v", tags)
	}
	// null ではなく [] で保存される
	b, _ := os.ReadFile(filepath.Join(dir, "source", "Props", "Chair", "meta.json"))
	if string(b) != "{\n  \"tags\": []\n}\n" {
		t.Errorf("meta.json = %q", b)
	}
}

func TestWriteTagsRejectsMissingAssetDir(t *testing.T) {
	dir := newLibrary(t)
	if err := WriteTags(dir, "Props", "Ghost", []string{"x"}); err == nil {
		t.Fatal("missing asset dir should error")
	}
}

func TestWriteTagsNormalizes(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	// 空白トリム・空要素除去・重複除去(順序は維持)
	if err := WriteTags(dir, "Props", "Chair", []string{" wood ", "", "wood", "metal"}); err != nil {
		t.Fatal(err)
	}
	tags := readTags(t, dir, "Props", "Chair")
	if len(tags) != 2 || tags[0] != "wood" || tags[1] != "metal" {
		t.Fatalf("tags = %v, want [wood metal]", tags)
	}
}

func TestWritePrivateCreatesMetaJSON(t *testing.T) {
	dir := newLibrary(t)
	// Finder で作られたアセット相当(meta.json なし)
	assetDir := filepath.Join(dir, "source", "Props", "Chair")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := WritePrivate(dir, "Props", "Chair", true); err != nil {
		t.Fatalf("WritePrivate: %v", err)
	}
	meta, err := ReadMeta(dir, "Props", "Chair")
	if err != nil {
		t.Fatal(err)
	}
	if !meta.Private {
		t.Fatal("asset should be private")
	}
	// タグは tags: [] のまま(null にしない)
	b, _ := os.ReadFile(filepath.Join(assetDir, "meta.json"))
	if string(b) != "{\n  \"tags\": [],\n  \"private\": true\n}\n" {
		t.Errorf("meta.json = %q", b)
	}
}

func TestWritePrivateFalseOmitsFlag(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivate(dir, "Props", "Chair", true); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivate(dir, "Props", "Chair", false); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(dir, "Props", "Chair")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Private {
		t.Fatal("asset should be public again")
	}
	// 既定(公開)に旗は書かない。旗のない既存 meta.json と同じ姿に戻る
	b, _ := os.ReadFile(filepath.Join(dir, "source", "Props", "Chair", "meta.json"))
	if string(b) != "{\n  \"tags\": []\n}\n" {
		t.Errorf("meta.json = %q", b)
	}
}

func TestWritePrivatePreservesTags(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteTags(dir, "Props", "Chair", []string{"wood"}); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivate(dir, "Props", "Chair", true); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(dir, "Props", "Chair")
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Tags) != 1 || meta.Tags[0] != "wood" {
		t.Fatalf("tags = %v, want [wood]", meta.Tags)
	}
}

func TestWriteTagsPreservesPrivate(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivate(dir, "Props", "Chair", true); err != nil {
		t.Fatal(err)
	}
	if err := WriteTags(dir, "Props", "Chair", []string{"wood"}); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(dir, "Props", "Chair")
	if err != nil {
		t.Fatal(err)
	}
	if !meta.Private {
		t.Fatal("saving tags should not flip the asset back to public")
	}
}

func TestWritePrivateRejectsMissingAssetDir(t *testing.T) {
	dir := newLibrary(t)
	if err := WritePrivate(dir, "Props", "Ghost", true); err == nil {
		t.Fatal("missing asset dir should error")
	}
}

func TestUpdateMetaRewritesCorruptMetaJSON(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(dir, "source", "Props", "Chair", "meta.json")
	if err := os.WriteFile(metaPath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 壊れた meta.json はゼロ値から書き直す
	if err := WriteTags(dir, "Props", "Chair", []string{"wood"}); err != nil {
		t.Fatalf("WriteTags over corrupt meta.json: %v", err)
	}
	meta, err := ReadMeta(dir, "Props", "Chair")
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Tags) != 1 || meta.Tags[0] != "wood" || meta.Private {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestUpdateMetaAbortsWhenMetaJSONIsUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0 では読み取りを禁止できない")
	}
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend", nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteTags(dir, "Props", "Chair", []string{"wood"}); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(dir, "source", "Props", "Chair", "meta.json")
	if err := os.Chmod(metaPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(metaPath, 0o644) })

	// 読めない meta.json に書き込むと、読めなかっただけのタグを消してしまう
	if err := WritePrivate(dir, "Props", "Chair", true); err == nil {
		t.Fatal("unreadable meta.json should abort the write")
	}
	if err := os.Chmod(metaPath, 0o644); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(dir, "Props", "Chair")
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Tags) != 1 || meta.Tags[0] != "wood" {
		t.Fatalf("tags = %v(中断せず消してしまっている)", meta.Tags)
	}
}
