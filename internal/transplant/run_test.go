package transplant

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/urfave/cli/v3"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, data)
	}
	return strings.TrimSpace(string(data))
}

func write(t *testing.T, root, rel, data string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}

func initRepo(t *testing.T, root string) {
	t.Helper()
	git(t, root, "init", "-b", "main")
	git(t, root, "config", "user.name", "Test Gardener")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "commit.gpgsign", "false")
	git(t, root, "config", "core.hooksPath", "/dev/null")
	git(t, root, "remote", "add", "origin", "git@github.com:Some-Owner/Fern.git")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "Template copy")
}

func fixture(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Sprout development is Linux only")
	}
	root := t.TempDir()
	write(t, root, "go.mod", "module sprout\n\ngo 1.26.7\n", 0o644)
	write(t, root, "scripts/build.sh", buildFixture+"printf 'build ran\\n'\n", 0o755)
	write(t, root, "scripts/test.sh", "#!/usr/bin/env bash\nprintf 'tests ran\\n'\n", 0o755)
	write(t, root, "scripts/cut", `#!/usr/bin/env bash
set -eu
case "$1" in
  --list-features-json) printf '%s\n' '`+contractFixture+`';;
  --finalize)
    printf 'finalizing\n'
    printf 'module %s\n\ngo 1.26.7\n' "$3" > go.mod
    case " $* " in
      *' service '*) sed -i '/^SERVICE_DESC="Sprout daemon"$/d; /^SERVICE_DEFAULT_PORT="8484"$/d' scripts/build.sh;;
      *' service.https '*) sed -i '/^SERVICE_DEFAULT_PORT="8484"$/d' scripts/build.sh;;
    esac
    rm scripts/cut
    ;;
  *) printf 'preview from cutter\n';;
esac
`, 0o755)
	write(t, root, "README.md", "Sprout readme\n", 0o644)
	write(t, root, "LICENSE.md", "Copyright 2026 Original Author\r\n\r\nPermission is hereby granted...\r\n", 0o644)
	write(t, root, "CONTRIBUTING.md", "Please contribute.\n", 0o644)
	write(t, root, "docs/MAINTENANCE.md", "maintenance\n", 0o644)
	write(t, root, "docs/content/docs/getting-started/create.md", "first step\n", 0o644)
	write(t, root, "docs/content/_index.md", "landing\n", 0o644)
	write(t, root, "docs/hugo.yaml", "branding\n", 0o644)
	write(t, root, "docs/assets/logo.svg", "logo\n", 0o644)
	initRepo(t, root)
	return root
}

func runCLI(ctx context.Context, root, input string, output, stderr io.Writer, args ...string) error {
	cmd := &cli.Command{Name: "transplant", Flags: Flags(), Action: func(ctx context.Context, cmd *cli.Command) error {
		return (&Wizard{Root: root, In: strings.NewReader(input), Out: output, Err: stderr}).Run(ctx, FromCommand(cmd))
	}}
	return cmd.Run(ctx, append([]string{"transplant"}, args...))
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestWizardFinalizeAndCommit(t *testing.T) {
	t.Setenv("CI", "true")
	for _, docs := range []string{"markdown", "keep", "none"} {
		t.Run(docs, func(t *testing.T) {
			root := fixture(t)
			write(t, root, "scripts/build.sh", buildFixture+"\nif [[ ${CI:-} == true ]]; then exit 9; fi\nprintf 'build ran\\n'\n", 0o755)
			git(t, root, "commit", "-am", "build refuses publication mode")
			sha := git(t, root, "rev-parse", "HEAD")
			var output bytes.Buffer
			err := runCLI(context.Background(), root, "", &output, &output, "--yes", "--service=headless", "--update=manual", "--docs="+docs)
			if err != nil {
				t.Fatalf("%v\n%s", err, &output)
			}
			if got := git(t, root, "branch", "--show-current"); got != "setup" {
				t.Fatal(got)
			}
			if got := git(t, root, "status", "--porcelain"); got != "" {
				t.Fatal(got)
			}
			if got := git(t, root, "log", "-1", "--format=%s"); !strings.Contains(got, "Data-Corruption/Sprout@"+sha[:12]) {
				t.Fatal(got)
			}
			if got := read(t, root, "go.mod"); !strings.Contains(got, "github.com/Some-Owner/Fern") {
				t.Fatal(got)
			}
			if got := read(t, root, "scripts/build.sh"); !strings.Contains(got, `APP_NAME="fern"`) || strings.Contains(got, `SERVICE_DEFAULT_PORT="8484"`) {
				t.Fatal(got)
			}
			if got := read(t, root, "LICENSE.md"); !strings.HasPrefix(got, "Copyright ") || !strings.Contains(got, "Test Gardener\r\n\r\nCopyright 2026 Original Author") {
				t.Fatal(got)
			}
			if got := read(t, root, "CONTRIBUTING.md"); got != "Please contribute.\n" {
				t.Fatal(got)
			}
			if _, err := os.Stat(filepath.Join(root, "scripts/cut")); !os.IsNotExist(err) {
				t.Fatal("cutter survived")
			}
			if docs == "none" {
				if _, err := os.Stat(filepath.Join(root, "docs")); !os.IsNotExist(err) {
					t.Fatal("docs survived")
				}
			} else {
				read(t, root, "docs/content/docs/getting-started/create.md")
				_, err := os.Stat(filepath.Join(root, "docs/hugo.yaml"))
				if docs == "keep" && err != nil || docs == "markdown" && !os.IsNotExist(err) {
					t.Fatalf("docs prune: %v", err)
				}
			}
			for _, want := range []string{"preview from cutter", "finalizing", "tests ran", "build ran", "https://sproutcli.dev/docs/getting-started/build/"} {
				if !strings.Contains(output.String(), want) {
					t.Errorf("output missing %q", want)
				}
			}
		})
	}
}

func TestWizardPreviewAndDeclineDoNotEdit(t *testing.T) {
	for _, preview := range []bool{true, false} {
		t.Run(stringBool(preview), func(t *testing.T) {
			root := fixture(t)
			args := []string{"--module=example.com/me/fern", "--app-name=fern", "--service=none", "--update=manual", "--docs=markdown", "--release-url=https://example.com/", "--contact-url=https://example.com/", "--default-log-level=warn", "--author=Gardener", "--year=2026", "--branch=false", "--commit=false"}
			if preview {
				args = append(args, "--preview", "--yes")
			}
			if err := runCLI(context.Background(), root, "n\n", io.Discard, io.Discard, args...); err != nil {
				t.Fatal(err)
			}
			if got := git(t, root, "status", "--porcelain"); got != "" {
				t.Fatal(got)
			}
			if got := git(t, root, "branch", "--show-current"); got != "main" {
				t.Fatal(got)
			}
			read(t, root, "scripts/cut")
		})
	}
}

func stringBool(value bool) string {
	if value {
		return "preview"
	}
	return "decline"
}

func TestWizardPipedAnswersAndEOF(t *testing.T) {
	root := fixture(t)
	// Defaults for module/name/URLs/log/author/year; numbered choices for shape.
	input := "\n\n1\n3\n\n\n\n\n\n2\ny\n"
	if err := runCLI(context.Background(), root, input, io.Discard, io.Discard, "--branch=false", "--commit=false", "--skip-verify"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, root, "scripts/build.sh"); strings.Contains(got, `SERVICE_DESC="Sprout daemon"`) {
		t.Fatal(got)
	}
	root = fixture(t)
	err := runCLI(context.Background(), root, "", io.Discard, io.Discard)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("want EOF, got %v", err)
	}
	if got := git(t, root, "status", "--porcelain"); got != "" {
		t.Fatal(got)
	}
}

func TestWizardPreflightRefusesBeforeBranchOrEdits(t *testing.T) {
	for name, mutate := range map[string]func(*testing.T, string){
		"dirty": func(t *testing.T, root string) { write(t, root, "untracked", "keep me", 0o644) },
		"wrong module": func(t *testing.T, root string) {
			write(t, root, "go.mod", "module example.com/already/done\n\ngo 1.26.7\n", 0o644)
			git(t, root, "commit", "-am", "changed")
		},
		"future Go": func(t *testing.T, root string) {
			write(t, root, "go.mod", "module sprout\n\ngo 9.99.0\n", 0o644)
			git(t, root, "commit", "-am", "changed")
		},
		"contract": func(t *testing.T, root string) {
			source := read(t, root, "scripts/cut")
			write(t, root, "scripts/cut", strings.Replace(source, `"version":1`, `"version":2`, 1), 0o755)
			git(t, root, "commit", "-am", "changed")
		},
		"symlink": func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("LICENSE.md", filepath.Join(root, "README.md")); err != nil {
				t.Fatal(err)
			}
			git(t, root, "commit", "-am", "changed")
		},
		"missing anchor": func(t *testing.T, root string) {
			write(t, root, "scripts/build.sh", strings.Replace(buildFixture, `APP_NAME="sprout"`, "", 1), 0o755)
			git(t, root, "commit", "-am", "changed")
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := fixture(t)
			mutate(t, root)
			before := git(t, root, "status", "--porcelain")
			if err := runCLI(context.Background(), root, "", io.Discard, io.Discard, "--yes", "--skip-verify"); err == nil {
				t.Fatal("accepted bad checkout")
			}
			if after := git(t, root, "status", "--porcelain"); after != before {
				t.Fatal("changed checkout")
			}
			if got := git(t, root, "branch", "--show-current"); got != "main" {
				t.Fatal(got)
			}
		})
	}
}

func TestWizardFailureKeepsChangesAndPrintsRecovery(t *testing.T) {
	for _, stage := range []string{"cut", "verify"} {
		t.Run(stage, func(t *testing.T) {
			root := fixture(t)
			if stage == "cut" {
				source := read(t, root, "scripts/cut")
				write(t, root, "scripts/cut", strings.Replace(source, "rm scripts/cut", "rm scripts/cut\n    echo 'cut failed' >&2\n    exit 7", 1), 0o755)
			} else {
				write(t, root, "scripts/test.sh", "#!/usr/bin/env bash\necho 'test failed' >&2\nexit 8\n", 0o755)
			}
			git(t, root, "commit", "-am", "failure fixture")
			sha := git(t, root, "rev-parse", "HEAD")
			var output bytes.Buffer
			if err := runCLI(context.Background(), root, "", &output, &output, "--yes"); err == nil {
				t.Fatal("accepted failure")
			}
			if !strings.Contains(output.String(), "git checkout -- . && git clean -fd") {
				t.Fatal(output.String())
			}
			if got := git(t, root, "rev-parse", "HEAD"); got != sha {
				t.Fatal("committed failure")
			}
			if got := git(t, root, "status", "--porcelain"); got == "" {
				t.Fatal("rolled back edits")
			}
			if strings.Contains(output.String(), "build ran") {
				t.Fatal("continued after failure")
			}
		})
	}
}

func TestCommandCancellationStopsDescendants(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux process groups")
	}
	root := t.TempDir()
	write(t, root, "child.sh", "#!/usr/bin/env bash\n(sleep 0.3; touch escaped) &\nwait\n", 0o755)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	w := &Wizard{Root: root, Out: io.Discard, Err: io.Discard}
	if _, err := w.command(ctx, false, "./child.sh"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	time.Sleep(350 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(root, "escaped")); !os.IsNotExist(err) {
		t.Fatal("child survived cancellation")
	}
}
