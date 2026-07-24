package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "3DLibrary", "config.json"))
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Default()
	if got != want {
		t.Fatalf("Load = %+v, want defaults %+v", got, want)
	}
}

func TestDefaults(t *testing.T) {
	d := Default()
	if d.ThumbnailSize != 512 {
		t.Errorf("ThumbnailSize = %d, want 512", d.ThumbnailSize)
	}
	if d.Theme != "system" {
		t.Errorf("Theme = %q, want %q", d.Theme, "system")
	}
	if d.BlenderPath != "" || d.LibraryDir != "" {
		t.Errorf("paths should default to empty: %+v", d)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	// 親ディレクトリが無い状態からの Save が成功すること(初回起動相当)
	path := filepath.Join(t.TempDir(), "nested", "3DLibrary", "config.json")
	s := NewStore(path)
	want := Config{
		BlenderPath:   "/usr/bin/blender",
		LibraryDir:    "/data/3DLibrary",
		ThumbnailSize: 1024,
		Theme:         "dark",
	}
	if err := s.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("roundtrip = %+v, want %+v", got, want)
	}
}

func TestLoadFillsMissingFieldsWithDefaults(t *testing.T) {
	// 手編集や旧バージョンの config.json でフィールドが欠けていても壊れない
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"blenderPath":"/opt/blender"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.BlenderPath != "/opt/blender" {
		t.Errorf("BlenderPath = %q", got.BlenderPath)
	}
	if got.ThumbnailSize != 512 || got.Theme != "system" {
		t.Errorf("missing fields should default: %+v", got)
	}
}

func TestLoadCorruptFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).Load(); err == nil {
		t.Fatal("Load of corrupt file should error")
	}
}

func TestValidate(t *testing.T) {
	valid := Default()
	if err := valid.Validate(); err != nil {
		t.Fatalf("defaults should validate: %v", err)
	}
	for name, mutate := range map[string]func(*Config){
		"bad thumbnail size": func(c *Config) { c.ThumbnailSize = 300 },
		"zero thumbnail":     func(c *Config) { c.ThumbnailSize = 0 },
		"bad theme":          func(c *Config) { c.Theme = "sepia" },
		"empty theme":        func(c *Config) { c.Theme = "" },
	} {
		c := Default()
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: Validate should fail for %+v", name, c)
		}
	}
}
