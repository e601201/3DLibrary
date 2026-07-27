package library

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrAssetExists は作成先のアセットディレクトリが既にある場合のエラー。
var ErrAssetExists = errors.New("asset already exists")

// ErrInvalidInput は利用者の入力(タイトル・カテゴリ・テンプレート名)が
// 不正な場合のエラー。API はこれを 400 に対応づける。
var ErrInvalidInput = errors.New("invalid input")

// ListTemplates は templates/ 直下の .blend ファイル名を名前順で返す。
// 固定リストではなく、ユーザーが .blend を置くだけで増える(requirements.md §9)。
func ListTemplates(libDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(libDir, "templates"))
	if err != nil {
		return nil, err
	}
	templates := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".blend") {
			templates = append(templates, e.Name())
		}
	}
	sort.Strings(templates)
	return templates, nil
}

// CreateAsset は source/{category}/{title}/ にアセット骨格を作る
// (requirements.md §8)。アプリが source へ書き込む 2 経路のうちの 1 つ
// (もう 1 つは WriteTags で、meta.json の書き出しはそちらに委ねている)。
// 既存のアセットディレクトリがある場合は何もせずエラーを返す。
// 引数の並びはパス構造 source/{category}/{title} に合わせている。
// tags は初期タグ(nil 可)で、meta.json へ WriteTags と同じ正規化を経て入る。
func CreateAsset(libDir, category, title, template string, tags []string) error {
	if err := validateDirName("category", category); err != nil {
		return err
	}
	if err := validateDirName("title", title); err != nil {
		return err
	}
	templatePath, err := resolveTemplate(libDir, template)
	if err != nil {
		return err
	}
	// 骨格を作る前にテンプレートを開いておく。読めないテンプレートで
	// 部分的な骨格(model.blend 無し)が残るのを防ぐ
	src, err := os.Open(templatePath)
	if err != nil {
		return err
	}
	defer src.Close()

	assetDir := filepath.Join(libDir, "source", category, title)
	if _, err := os.Stat(assetDir); err == nil {
		return fmt.Errorf("%w: %s/%s", ErrAssetExists, category, title)
	} else if !os.IsNotExist(err) {
		return err
	}

	for _, sub := range []string{"textures", "references", "renders"} {
		if err := os.MkdirAll(filepath.Join(assetDir, sub), 0o755); err != nil {
			return err
		}
	}
	if err := copyFile(src, filepath.Join(assetDir, "model.blend")); err != nil {
		return err
	}
	// meta.json の書式は WriteTags に一本化する(タグ無しなら tags: [] になる)
	if err := WriteTags(libDir, category, title, tags); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(assetDir, "notes.md"), nil, 0o644)
}

// copyFile は src の内容を dst へストリームコピーする(.blend は大きい
// ことがあるためメモリに全読みしない)。
func copyFile(src *os.File, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// validateDirName はカテゴリ・タイトルとして安全なディレクトリ名か検査する。
func validateDirName(field, name string) error {
	switch {
	case name == "":
		return fmt.Errorf("%w: %s must not be empty", ErrInvalidInput, field)
	case strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, 0):
		return fmt.Errorf("%w: %s must not contain path separators: %q", ErrInvalidInput, field, name)
	case name == "." || name == "..":
		return fmt.Errorf("%w: %s must be a valid directory name: %q", ErrInvalidInput, field, name)
	case strings.HasPrefix(name, "."):
		// 隠しディレクトリはスキャン対象外になるため作らせない
		return fmt.Errorf("%w: %s must not start with a dot: %q", ErrInvalidInput, field, name)
	}
	return nil
}

// resolveTemplate は templates/ 直下のテンプレートであることを検証して
// フルパスを返す。
func resolveTemplate(libDir, template string) (string, error) {
	if template == "" || template != filepath.Base(template) || !strings.HasSuffix(template, ".blend") {
		return "", fmt.Errorf("%w: invalid template name: %q", ErrInvalidInput, template)
	}
	path := filepath.Join(libDir, "templates", template)
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%w: template not found: %q", ErrInvalidInput, template)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%w: invalid template name: %q", ErrInvalidInput, template)
	}
	return path, nil
}
