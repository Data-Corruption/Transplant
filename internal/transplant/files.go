package transplant

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var projectHeading = regexp.MustCompile(`(?m)^# Project config -+\r?$`)
var sectionHeading = regexp.MustCompile(`(?m)^# [A-Za-z][^\r\n]* -{3,}\r?$`)

// Replace only exact assignments inside the project block. In particular, the
// service fallbacks above the block belong to the build implementation.
func editProject(data []byte, values map[string]string) ([]byte, error) {
	starts := projectHeading.FindAllIndex(data, -1)
	if len(starts) != 1 {
		return nil, fmt.Errorf("build.sh needs exactly one '# Project config' heading")
	}
	start := starts[0][1]
	endHeading := sectionHeading.FindIndex(data[start:])
	if endHeading == nil {
		return nil, fmt.Errorf("can't find the end of build.sh's project config")
	}
	end := start + endHeading[0]
	block := append([]byte(nil), data[start:end]...)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := values[key]
		if err := plainValue(value); err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		anchor := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `="[^"\r\n]*"\r?$`)
		matches := anchor.FindAllIndex(block, -1)
		if len(matches) != 1 {
			return nil, fmt.Errorf("build.sh project config needs exactly one %s=\"value\" line (found %d)", key, len(matches))
		}
		block = anchor.ReplaceAllFunc(block, func(line []byte) []byte {
			replacement := key + "=\"" + value + "\""
			if bytes.HasSuffix(line, []byte("\r")) {
				replacement += "\r"
			}
			return []byte(replacement)
		})
	}
	result := append([]byte(nil), data[:start]...)
	result = append(result, block...)
	return append(result, data[end:]...), nil
}

func (o Options) projectValues() map[string]string {
	values := map[string]string{"APP_NAME": o.AppName, "RELEASE_URL": o.ReleaseURL, "CONTACT_URL": o.ContactURL, "DEFAULT_LOG_LEVEL": o.LogLevel}
	if o.Service != "none" {
		values["SERVICE_DESC"] = o.ServiceDesc
	}
	if o.Service == "https" {
		values["SERVICE_DEFAULT_PORT"] = o.Port
	}
	return values
}

func (w *Wizard) safePath(rel string) (string, error) {
	current := w.Root
	for _, part := range strings.Split(filepath.FromSlash(rel), string(filepath.Separator)) {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid checkout path %q", rel)
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", fmt.Errorf("inspect %s: %w", rel, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s is a symlink; please sort that out first", rel)
		}
	}
	return current, nil
}

func (w *Wizard) readFile(rel string) ([]byte, error) {
	p, err := w.safePath(rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s needs to be a regular file", rel)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	return data, nil
}

func (w *Wizard) writeFile(rel string, data []byte) error {
	p, err := w.safePath(rel)
	if err != nil {
		return err
	}
	info, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s needs to be a regular file", rel)
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".transplant-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(info.Mode().Perm()); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), p); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	return nil
}

func (w *Wizard) checkFiles(o Options) error {
	build, err := w.readFile("scripts/build.sh")
	if err != nil {
		return err
	}
	if _, err := editProject(build, o.projectValues()); err != nil {
		return err
	}
	for _, rel := range []string{"README.md", "LICENSE.md", "scripts/test.sh"} {
		data, err := w.readFile(rel)
		if err != nil {
			return err
		}
		if rel == "LICENSE.md" && !bytes.Contains(data, []byte("Copyright ")) {
			return fmt.Errorf("LICENSE.md has no recognizable copyright notice; please check this template")
		}
	}
	if o.Docs == "keep" {
		return nil
	}
	docs, err := w.safePath("docs")
	if err != nil {
		return err
	}
	if o.Docs == "markdown" {
		if _, err := w.readFile("docs/MAINTENANCE.md"); err != nil {
			return err
		}
		p, err := w.safePath("docs/content/docs")
		if err != nil {
			return err
		}
		info, err := os.Stat(p)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("docs/content/docs needs to be a directory")
		}
	}
	return filepath.WalkDir(docs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink; please sort that out before pruning docs", p)
		}
		return nil
	})
}

func (w *Wizard) finishFiles(o Options) error {
	data, err := w.readFile("scripts/build.sh")
	if err != nil {
		return err
	}
	data, err = editProject(data, o.projectValues())
	if err != nil {
		return err
	}
	if err := w.writeFile("scripts/build.sh", data); err != nil {
		return err
	}
	readme := fmt.Sprintf("# %s\n\nA Go application.\n\nBuilt from [Sprout](https://sproutcli.dev/).\n", o.AppName)
	if err := w.writeFile("README.md", []byte(readme)); err != nil {
		return err
	}
	license, err := w.readFile("LICENSE.md")
	if err != nil {
		return err
	}
	newline := "\n"
	if bytes.Contains(license, []byte("\r\n")) {
		newline = "\r\n"
	}
	notice := fmt.Sprintf("Copyright %s %s", o.Year, o.Author)
	// If the original author is making another app, don't repeat their notice.
	if !bytes.Contains(license, []byte(notice+newline)) {
		if err := w.writeFile("LICENSE.md", append([]byte(notice+newline+newline), license...)); err != nil {
			return err
		}
	}
	switch o.Docs {
	case "none":
		return os.RemoveAll(filepath.Join(w.Root, "docs"))
	case "markdown":
		return w.pruneDocs("docs")
	}
	return nil
}

func (w *Wizard) pruneDocs(rel string) error {
	entries, err := os.ReadDir(filepath.Join(w.Root, rel))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := rel + "/" + entry.Name()
		switch child {
		case "docs/MAINTENANCE.md", "docs/content/docs":
			continue
		case "docs/content":
			if err := w.pruneDocs(child); err != nil {
				return err
			}
		default:
			if err := os.RemoveAll(filepath.Join(w.Root, child)); err != nil {
				return fmt.Errorf("remove %s: %w", child, err)
			}
		}
	}
	return nil
}
