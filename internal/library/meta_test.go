package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMetaMissingFileMeansNoTags(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend"); err != nil {
		t.Fatal(err)
	}
	// meta.json を消しても「タグ空」として扱う
	if err := os.Remove(filepath.Join(dir, "source", "Props", "Chair", "meta.json")); err != nil {
		t.Fatal(err)
	}
	tags, err := ReadTags(dir, "Props", "Chair")
	if err != nil {
		t.Fatalf("ReadTags: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("tags = %v, want empty", tags)
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

	tags, err := ReadTags(dir, "Props", "Chair")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0] != "wood" || tags[1] != "furniture" {
		t.Fatalf("roundtrip tags = %v", tags)
	}
}

func TestWriteTagsEmptyListWritesEmptyArray(t *testing.T) {
	dir := newLibrary(t)
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend"); err != nil {
		t.Fatal(err)
	}
	if err := WriteTags(dir, "Props", "Chair", nil); err != nil {
		t.Fatal(err)
	}
	tags, err := ReadTags(dir, "Props", "Chair")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
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
	if err := CreateAsset(dir, "Props", "Chair", "empty.blend"); err != nil {
		t.Fatal(err)
	}
	// 空白トリム・空要素除去・重複除去(順序は維持)
	if err := WriteTags(dir, "Props", "Chair", []string{" wood ", "", "wood", "metal"}); err != nil {
		t.Fatal(err)
	}
	tags, err := ReadTags(dir, "Props", "Chair")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0] != "wood" || tags[1] != "metal" {
		t.Fatalf("tags = %v, want [wood metal]", tags)
	}
}
