package transplant

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/version"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Data-Corruption/Transplant/pkg/xterm/prompt"
	"golang.org/x/mod/modfile"
)

const WindowsMessage = "sprout dev is linux only, use WSL to run transplant. windows binaries are shipped cause the tool itself is a deployed example of Sprout."

func CheckPlatform() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("%s", WindowsMessage)
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("sprout dev is linux only; run transplant in a Linux checkout (WSL works too)")
	}
	return nil
}

type Wizard struct {
	Root     string
	In       io.Reader
	Out, Err io.Writer
	reader   *prompt.Reader
}

func (w *Wizard) command(ctx context.Context, capture bool, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = w.Root
	if name == "go" && len(args) == 1 && args[0] == "version" {
		// Inspect the tool on PATH without go.mod silently downloading a newer
		// toolchain before we've checked the development prerequisites.
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	}
	if name == "./scripts/build.sh" {
		// Verification is always a local dev build, even when the wizard runs
		// on a CI runner. Sprout reserves CI=true for release publication.
		cmd.Env = append(os.Environ(), "CI=false")
	}
	// Prompts belong to the wizard. Child tools shouldn't steal buffered piped
	// answers or hang unattended waiting for input.
	cmd.Stdin = nil
	cmd.Stderr = w.Err
	cmd.WaitDelay = 2 * time.Second
	configureProcess(cmd)
	var output bytes.Buffer
	if capture {
		cmd.Stdout = &output
	} else {
		cmd.Stdout = w.Out
	}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return output.String(), nil
}

func (w *Wizard) clean(ctx context.Context) error {
	status, err := w.command(ctx, true, "git", "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("this checkout has changes; commit or stash them first (including untracked files)")
	}
	return nil
}

func (w *Wizard) preflight(ctx context.Context, o Options) (string, error) {
	if err := CheckPlatform(); err != nil {
		return "", err
	}
	cut, err := w.safePath("scripts/cut")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("scripts/cut is missing — this tree is already finalized, or isn't a fresh Sprout copy")
		}
		return "", err
	}
	info, err := os.Stat(cut)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("scripts/cut isn't an executable file; this tree may already be finalized")
	}
	data, err := w.readFile("go.mod")
	if err != nil {
		return "", err
	}
	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	if mod.Module == nil || mod.Module.Mod.Path != "sprout" {
		return "", fmt.Errorf("go.mod needs 'module sprout'; run this from a fresh template copy")
	}
	if mod.Go == nil || !version.IsValid("go"+mod.Go.Version) {
		return "", fmt.Errorf("go.mod needs a valid go directive")
	}
	for _, tool := range []string{"git", "go", "bash", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			return "", fmt.Errorf("please install %s and put it on PATH: %w", tool, err)
		}
	}
	if !o.SkipVerify && !o.Preview {
		if _, err := exec.LookPath("gcc"); err != nil {
			return "", fmt.Errorf("the race tests need gcc; install it or use --skip-verify: %w", err)
		}
	}
	top, err := w.command(ctx, true, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top, err = filepath.EvalSymlinks(strings.TrimSpace(top))
	if err != nil {
		return "", err
	}
	if top != w.Root {
		return "", fmt.Errorf("run transplant from the checkout root: %s", top)
	}
	if err := w.clean(ctx); err != nil {
		return "", err
	}
	goVersion, err := w.command(ctx, true, "go", "version")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(goVersion)
	if len(fields) < 3 || !version.IsValid(fields[2]) || version.Compare(fields[2], "go"+mod.Go.Version) < 0 {
		return "", fmt.Errorf("this tree needs Go %s or newer; found %s", mod.Go.Version, strings.TrimSpace(goVersion))
	}
	sha, err := w.command(ctx, true, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	contract, err := w.command(ctx, true, "./scripts/cut", "--list-features-json")
	if err != nil {
		return "", fmt.Errorf("can't read this tree's Sprout contract; use a template with --list-features-json support: %w", err)
	}
	if err := validateContract([]byte(contract)); err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

func (w *Wizard) ask(ctx context.Context, o Options, flag, label string, value *string) error {
	if o.Set[flag] || o.Yes {
		if *value == "" {
			return fmt.Errorf("couldn't find a default for --%s; please pass it explicitly", flag)
		}
		return nil
	}
	if *value != "" {
		label += " [" + *value + "]"
	}
	answer, err := w.reader.String(ctx, label)
	if err != nil {
		return fmt.Errorf("read --%s (use --yes and flags for unattended runs): %w", flag, err)
	}
	if answer != "" {
		*value = answer
	}
	return nil
}

func (w *Wizard) confirm(ctx context.Context, label string, defaultYes bool) (bool, error) {
	hint := " [y/N]"
	if defaultYes {
		hint = " [Y/n]"
	}
	for {
		answer, err := w.reader.String(ctx, label+hint)
		if err != nil {
			return false, err
		}
		switch strings.ToLower(answer) {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(w.Out, "just y or n is good :)")
		}
	}
}

func (w *Wizard) answers(ctx context.Context, o *Options) error {
	if o.Module == "" && !o.Set["module"] {
		// A missing origin is fine; asking for the module is the fallback.
		remote, err := w.command(ctx, true, "git", "remote", "get-url", "origin")
		if err == nil {
			o.Module = moduleFromRemote(remote)
		}
	}
	if err := w.ask(ctx, *o, "module", "what's your Go module path?", &o.Module); err != nil {
		return err
	}
	if o.AppName == "" && !o.Set["app-name"] {
		o.AppName = appNameFromModule(o.Module)
	}
	if err := w.ask(ctx, *o, "app-name", "what should the binary be called?", &o.AppName); err != nil {
		return err
	}
	if !o.Yes && !o.Set["service"] {
		fmt.Fprintln(w.Out, "\nservice shape:\n  1. none — CLI only\n  2. headless — background worker\n  3. https — worker + HTTPS dashboard")
	}
	if err := w.ask(ctx, *o, "service", "pick a service shape (name or number)", &o.Service); err != nil {
		return err
	}
	if !o.Set["service"] && !o.Yes {
		o.Service = numbered(o.Service, []string{"none", "headless", "https"})
	}
	updates := []string{"none", "check", "manual"}
	if o.Service != "none" {
		updates = append(updates, "auto")
	}
	if !o.Yes && !o.Set["update"] {
		fmt.Fprintln(w.Out, "\nupdate support:\n  1. none — update by rerunning the installer\n  2. check — discovery and update notices\n  3. manual — also apply updates when asked")
		if o.Service != "none" {
			fmt.Fprintln(w.Out, "  4. auto — also support unattended updates (off until the app's operator enables them)")
		}
	}
	if err := w.ask(ctx, *o, "update", "how should updates work? (name or number)", &o.Update); err != nil {
		return err
	}
	if !o.Set["update"] && !o.Yes {
		o.Update = numbered(o.Update, updates)
	}
	if _, err := featureCuts(o.Service, o.Update); err != nil {
		return err
	}
	if o.ContactURL == "" && !o.Set["contact-url"] {
		o.ContactURL = "https://" + o.Module
	}
	if o.ServiceDesc == "" && !o.Set["service-desc"] {
		o.ServiceDesc = o.AppName + " service"
	}
	for _, field := range []struct {
		flag, label string
		value       *string
	}{
		{"release-url", "where will releases live? (a placeholder is fine)", &o.ReleaseURL},
		{"contact-url", "where can people find you or the project?", &o.ContactURL},
		{"default-log-level", "default log level (debug/info/warn/error/none)", &o.LogLevel},
	} {
		if err := w.ask(ctx, *o, field.flag, field.label, field.value); err != nil {
			return err
		}
	}
	if o.Service != "none" {
		if err := w.ask(ctx, *o, "service-desc", "a short description for the service?", &o.ServiceDesc); err != nil {
			return err
		}
	}
	if o.Service == "https" {
		if err := w.ask(ctx, *o, "service-default-port", "which HTTPS port?", &o.Port); err != nil {
			return err
		}
	}
	if o.Author == "" && !o.Set["author"] {
		name, err := w.command(ctx, true, "git", "config", "user.name")
		if err == nil {
			o.Author = strings.TrimSpace(name)
		}
	}
	if err := w.ask(ctx, *o, "author", "whose name goes on the copyright notice?", &o.Author); err != nil {
		return err
	}
	if err := w.ask(ctx, *o, "year", "copyright year?", &o.Year); err != nil {
		return err
	}
	if !o.Yes && !o.Set["docs"] {
		fmt.Fprintln(w.Out, "\ndocs:\n  1. keep — the whole Sprout Hugo site, branding and all\n  2. markdown — keep the guides and maintenance notes\n  3. none — remove docs/")
	}
	if err := w.ask(ctx, *o, "docs", "what should happen to the docs? (name or number)", &o.Docs); err != nil {
		return err
	}
	if !o.Set["docs"] && !o.Yes {
		o.Docs = numbered(o.Docs, []string{"keep", "markdown", "none"})
	}
	return o.validate()
}

func numbered(value string, choices []string) string {
	for i, choice := range choices {
		if value == fmt.Sprint(i+1) {
			return choice
		}
	}
	return value
}

func (w *Wizard) Run(ctx context.Context, o Options) (runErr error) {
	if w.In == nil {
		w.In = os.Stdin
	}
	if w.Out == nil {
		w.Out = os.Stdout
	}
	if w.Err == nil {
		w.Err = os.Stderr
	}
	w.reader = prompt.NewReader(w.In, w.Out)
	if w.Root == "" {
		w.Root = "."
	}
	root, err := filepath.Abs(w.Root)
	if err != nil {
		return err
	}
	w.Root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	sha, err := w.preflight(ctx, o)
	if err != nil {
		return err
	}
	fmt.Fprintln(w.Out, "let's give this Sprout its own home :)")
	if !o.Yes && !o.Set["branch"] && !o.Preview {
		o.Branch, err = w.confirm(ctx, "make a setup branch? it's your uh oh button", true)
		if err != nil {
			return err
		}
	}
	if err := w.answers(ctx, &o); err != nil {
		return err
	}
	if err := w.checkFiles(o); err != nil {
		return err
	}
	cuts, _ := featureCuts(o.Service, o.Update)
	args := append([]string{"--module", o.Module}, cuts...)
	fmt.Fprintf(w.Out, "\n%s → %s\nservice: %s; updates: %s; docs: %s\n", o.Module, o.AppName, o.Service, o.Update, o.Docs)
	fmt.Fprintf(w.Out, "releases: %s\ncontact: %s\nlog level: %s\ncopyright: %s %s\n", o.ReleaseURL, o.ContactURL, o.LogLevel, o.Year, o.Author)
	if o.Service != "none" {
		fmt.Fprintf(w.Out, "service description: %s\n", o.ServiceDesc)
	}
	if o.Service == "https" {
		fmt.Fprintf(w.Out, "HTTPS port: %s\n", o.Port)
	}
	fmt.Fprintf(w.Out, "setup branch: %t; offer commit: %t; skip verification: %t\n", o.Branch, o.Commit, o.SkipVerify)
	fmt.Fprintln(w.Out, "README gets a fresh stub; your license notice is added and the original stays. CONTRIBUTING stays as-is.\n\nhere's the cutter's plan:")
	if _, err := w.command(ctx, false, "./scripts/cut", args...); err != nil {
		return err
	}
	if o.Preview {
		fmt.Fprintln(w.Out, "preview done — nothing changed.")
		return nil
	}
	if !o.Yes {
		ok, err := w.confirm(ctx, "look good? finalize this tree?", false)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(w.Out, "all good, leaving the checkout alone.")
			return nil
		}
	}
	// The user may have edited the checkout while answering questions.
	if err := w.clean(ctx); err != nil {
		return err
	}
	current, err := w.command(ctx, true, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(current) != sha {
		return fmt.Errorf("HEAD changed while we were planning; rerun transplant")
	}
	if o.Branch {
		if _, err := w.command(ctx, false, "git", "switch", "-c", "setup"); err != nil {
			return fmt.Errorf("couldn't make the setup branch (already have one? use --branch=false): %w", err)
		}
	}
	// After finalization starts, only the operator can decide to discard work.
	defer func() {
		if runErr != nil {
			fmt.Fprintln(w.Err, "\nstopped here; your changes are still in the checkout. to discard the setup changes, review them first, then run:\n  git checkout -- . && git clean -fd\nthat also deletes untracked files, so save anything you want to keep.")
		}
	}()
	if _, err := w.command(ctx, false, "./scripts/cut", append([]string{"--finalize"}, args...)...); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.finishFiles(o); err != nil {
		return err
	}
	if !o.SkipVerify {
		for _, script := range []string{"./scripts/test.sh", "./scripts/build.sh"} {
			if _, err := w.command(ctx, false, script); err != nil {
				return fmt.Errorf("verification stopped; fix the checkout and rerun %s: %w", script, err)
			}
		}
	} else {
		fmt.Fprintln(w.Out, "skipped verification — run ./scripts/test.sh and ./scripts/build.sh when you're ready.")
	}
	commit := o.Commit
	if commit && !o.Yes && !o.Set["commit"] {
		commit, err = w.confirm(ctx, "want me to commit the setup?", false)
		if err != nil {
			return err
		}
	}
	short := sha
	if len(short) > 12 {
		short = short[:12]
	}
	message := "Set up project from Sprout (Data-Corruption/Sprout@" + short + ")"
	if commit {
		if _, err := w.command(ctx, false, "git", "add", "-A"); err != nil {
			return err
		}
		if _, err := w.command(ctx, false, "git", "commit", "-m", message); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(w.Out, "left the changes for you to review. suggested commit message:\n  %s\n", message)
	}
	fmt.Fprintln(w.Out, "\nyou're all set <3\nbuild and run: https://sproutcli.dev/docs/getting-started/build/\npublish: https://sproutcli.dev/docs/getting-started/release/")
	return nil
}
