package generate

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/e601201/3DLibrary/internal/config"
	"github.com/e601201/3DLibrary/internal/library"
)

// ErrBlenderNotConfigured は設定に Blender 実行ファイルが無い場合のエラー。
var ErrBlenderNotConfigured = errors.New("blender executable is not configured")

// 1 ジョブの上限。ハングした Blender がキューを永久に塞がないための保険。
const jobTimeout = 10 * time.Minute

//go:embed generate.py
var generateScript []byte

// Runner は Blender CLI を 1 回起動して GLB・サムネイル・抽出メタデータの
// 3 点を書き出す。設定(Blender パス・サムネイルサイズ)は実行時に読む。
type Runner struct {
	store *config.Store
}

func NewRunner(store *config.Store) *Runner {
	return &Runner{store: store}
}

// Run は job のキャッシュ 3 点を生成する。source には一切書き込まない。
func (r *Runner) Run(job Job) error {
	cfg, err := r.store.Load()
	if err != nil {
		return err
	}
	if cfg.BlenderPath == "" {
		return ErrBlenderNotConfigured
	}

	paths := library.CachePaths(job.LibDir, job.Category, job.Title)
	for _, p := range []string{paths.GLB, paths.Thumbnail, paths.Metadata} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
	}

	scriptPath, err := writeTempScript()
	if err != nil {
		return err
	}
	defer os.Remove(scriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cfg.BlenderPath,
		"-b", job.BlendPath,
		"--factory-startup",
		"--python", scriptPath,
		"--",
		"--glb", paths.GLB,
		"--thumb", paths.Thumbnail,
		"--meta", paths.Metadata,
		"--size", strconv.Itoa(cfg.ThumbnailSize),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("blender failed: %w: %s", err, tail(out, 2000))
	}

	for _, p := range []string{paths.GLB, paths.Thumbnail, paths.Metadata} {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("generation finished but output is missing: %s: %s", p, tail(out, 1000))
		}
	}
	return nil
}

// writeTempScript は埋め込みの generate.py を一時ファイルに書き出す
// (Blender にはファイルパスでしか渡せないため)。
func writeTempScript() (string, error) {
	f, err := os.CreateTemp("", "3dlibrary-generate-*.py")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(generateScript); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func tail(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return "..." + string(b[len(b)-n:])
}
