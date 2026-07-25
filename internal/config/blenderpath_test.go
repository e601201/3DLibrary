package config

import (
	"os"
	"path/filepath"
	"testing"
)

// makeAppBundle は macOS の Blender.app 相当(Contents/MacOS/Blender を
// 持つディレクトリ)を作り、バンドルと中の実行ファイルのパスを返す。
func makeAppBundle(t *testing.T) (bundle, binary string) {
	t.Helper()
	bundle = filepath.Join(t.TempDir(), "Blender.app")
	binary = filepath.Join(bundle, "Contents", "MacOS", "Blender")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bundle, binary
}

func TestNormalizeBlenderPathEmptyIsAllowed(t *testing.T) {
	// 未設定のまま他の設定だけ保存できる必要がある
	got, err := NormalizeBlenderPath("")
	if err != nil || got != "" {
		t.Fatalf("NormalizeBlenderPath(\"\") = %q, %v", got, err)
	}
}

func TestNormalizeBlenderPathResolvesMacAppBundle(t *testing.T) {
	// Finder 上は Blender.app が 1 個のアプリに見えるので、利用者は
	// バンドル自体を指定しがち。そのまま exec すると permission denied になる
	bundle, binary := makeAppBundle(t)
	got, err := NormalizeBlenderPath(bundle)
	if err != nil {
		t.Fatalf("NormalizeBlenderPath: %v", err)
	}
	if got != binary {
		t.Errorf("= %q, want %q", got, binary)
	}
}

func TestNormalizeBlenderPathKeepsExecutableAsIs(t *testing.T) {
	_, binary := makeAppBundle(t)
	got, err := NormalizeBlenderPath(binary)
	if err != nil {
		t.Fatalf("NormalizeBlenderPath: %v", err)
	}
	if got != binary {
		t.Errorf("= %q, want unchanged %q", got, binary)
	}
}

func TestNormalizeBlenderPathRejectsMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-blender")
	if _, err := NormalizeBlenderPath(missing); err == nil {
		t.Fatal("missing path should error")
	}
}

func TestNormalizeBlenderPathRejectsPlainDirectory(t *testing.T) {
	// 実行ファイルではなくフォルダを指定した場合は、生成時ではなく保存時に弾く
	if _, err := NormalizeBlenderPath(t.TempDir()); err == nil {
		t.Fatal("directory should error")
	}
}

func TestNormalizeBlenderPathRejectsBrokenAppBundle(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "Blender.app")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeBlenderPath(bundle); err == nil {
		t.Fatal("app bundle without an executable should error")
	}
}
