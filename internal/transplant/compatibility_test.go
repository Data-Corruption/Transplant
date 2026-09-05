package transplant

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// CI points this at a shallow clone of Sprout main. A local checkout also
// works, including its pending edits, so changes to both repos can be tested
// together without publishing either one.
func TestSproutCompatibility(t *testing.T) {
	source := os.Getenv("TRANSPLANT_SPROUT")
	if source == "" {
		t.Skip("set TRANSPLANT_SPROUT to a Sprout checkout to run the compatibility canary")
	}
	if runtime.GOOS != "linux" {
		t.Skip("Sprout development is Linux only")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		t.Fatal(err)
	}
	list := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	list.Dir = source
	files, err := list.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, shape := range []struct{ service, update, docs string }{
		{"https", "auto", "markdown"},
		{"none", "none", "none"},
		{"headless", "manual", "keep"},
	} {
		t.Run(shape.service+"-"+shape.update, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range bytes.Split(files, []byte{0}) {
				if len(name) == 0 {
					continue
				}
				rel := string(name)
				p := filepath.Join(source, rel)
				info, err := os.Lstat(p)
				if err != nil {
					t.Fatal(err)
				}
				if !info.Mode().IsRegular() {
					t.Fatalf("expected regular source file: %s", p)
				}
				data, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				write(t, root, rel, string(data), info.Mode().Perm())
			}
			initRepo(t, root)
			var output bytes.Buffer
			ctx := context.Background()
			// Service removal must be expanded by the real cutter even when the
			// selected update capability only names the automatic leaf.
			if shape.service == "none" {
				err := runCLI(ctx, root, "", &output, &output, "--yes", "--preview", "--service=none", "--update=manual")
				if err != nil {
					t.Fatalf("preview: %v\n%s", err, &output)
				}
				if !strings.Contains(output.String(), "update.apply.auto") {
					t.Fatalf("preview didn't report automatic removal:\n%s", &output)
				}
				if got := git(t, root, "status", "--porcelain"); got != "" {
					t.Fatalf("preview edited tree: %s", got)
				}
				output.Reset()
			}
			err := runCLI(ctx, root, "", &output, &output,
				"--yes", "--module=example.com/garden/fern", "--app-name=fern",
				"--service="+shape.service, "--update="+shape.update, "--docs="+shape.docs,
				"--author=Test Gardener", "--year=2026")
			if err != nil {
				t.Fatalf("transplant: %v\n%s", err, &output)
			}
			if got := git(t, root, "status", "--porcelain"); got != "" {
				t.Fatalf("uncommitted output: %s", got)
			}
			if _, err := os.Stat(filepath.Join(root, "scripts/cut")); !os.IsNotExist(err) {
				t.Fatal("template cutter survived")
			}
			if _, err := os.Stat(filepath.Join(root, "out", "linux-"+runtime.GOARCH)); err != nil {
				t.Fatalf("missing verified dev build: %v", err)
			}
			t.Log("finalization, tests, and dev build passed")
		})
	}
}
