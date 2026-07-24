package generate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/e601201/3DLibrary/internal/config"
	"github.com/e601201/3DLibrary/internal/library"
)

// fakeBlender は generate.py と同じ引数規約で 3 点を書き出すダミー実行ファイル。
const fakeBlender = `#!/bin/sh
# 引数から --glb/--thumb/--meta を拾って書き出す
while [ $# -gt 0 ]; do
  case "$1" in
    --glb) GLB="$2"; shift 2 ;;
    --thumb) THUMB="$2"; shift 2 ;;
    --meta) META="$2"; shift 2 ;;
    --size) SIZE="$2"; shift 2 ;;
    *) shift ;;
  esac
done
echo "glb" > "$GLB"
echo "png" > "$THUMB"
echo '{"objectCount":1,"collectionCount":1,"materialCount":0,"polygonCount":6,"textureCount":0,"hasAnimation":false}' > "$META"
`

func writeScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-blender.sh")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func newRunner(t *testing.T, blenderPath string) (*Runner, string) {
	t.Helper()
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	cfg := config.Default()
	cfg.BlenderPath = blenderPath
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	libDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(libDir, "source", "Props", "Chair"), 0o755); err != nil {
		t.Fatal(err)
	}
	blend := filepath.Join(libDir, "source", "Props", "Chair", "model.blend")
	if err := os.WriteFile(blend, []byte("blend"), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewRunner(store), libDir
}

func chairJob(libDir string) Job {
	return Job{
		Category:  "Props",
		Title:     "Chair",
		BlendPath: filepath.Join(libDir, "source", "Props", "Chair", "model.blend"),
		LibDir:    libDir,
	}
}

func TestRunnerProducesAllThreeCacheFiles(t *testing.T) {
	r, libDir := newRunner(t, writeScript(t, fakeBlender))
	if err := r.Run(chairJob(libDir)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	paths := library.CachePaths(libDir, "Props", "Chair")
	for _, p := range []string{paths.GLB, paths.Thumbnail, paths.Metadata} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing output %s: %v", p, err)
		}
	}
}

func TestRunnerFailsWithoutBlenderPath(t *testing.T) {
	r, libDir := newRunner(t, "")
	err := r.Run(chairJob(libDir))
	if !errors.Is(err, ErrBlenderNotConfigured) {
		t.Fatalf("err = %v, want ErrBlenderNotConfigured", err)
	}
}

func TestRunnerReportsBlenderFailureWithOutput(t *testing.T) {
	r, libDir := newRunner(t, writeScript(t, "#!/bin/sh\necho boom-details\nexit 1\n"))
	err := r.Run(chairJob(libDir))
	if err == nil {
		t.Fatal("Run should fail when blender exits non-zero")
	}
	if !strings.Contains(err.Error(), "boom-details") {
		t.Errorf("error should include blender output: %v", err)
	}
}

func TestRunnerFailsWhenOutputsMissing(t *testing.T) {
	// 正常終了しても 3 点が揃っていなければ失敗扱い
	r, libDir := newRunner(t, writeScript(t, "#!/bin/sh\nexit 0\n"))
	err := r.Run(chairJob(libDir))
	if err == nil {
		t.Fatal("Run should fail when outputs are missing")
	}
}
