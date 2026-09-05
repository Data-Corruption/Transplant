// Package transplant drives a checkout's own Sprout cutter. It does not own
// feature expansion, source rewriting, or any installed application state.
package transplant

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/urfave/cli/v3"
	"golang.org/x/mod/module"
)

type Options struct {
	Module, AppName, Service, Update, Docs              string
	ReleaseURL, ContactURL, LogLevel, ServiceDesc, Port string
	Author, Year                                        string
	Branch, Commit, Yes, SkipVerify, Preview            bool
	Set                                                 map[string]bool
}

func Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "module", Usage: "Go module path (defaults to your origin remote)"},
		&cli.StringFlag{Name: "app-name", Usage: "binary and storage name"},
		&cli.StringFlag{Name: "service", Value: "https", Usage: "none, headless, or https"},
		&cli.StringFlag{Name: "update", Value: "manual", Usage: "none, check, manual, or auto (needs a service)"},
		&cli.StringFlag{Name: "docs", Value: "markdown", Usage: "keep, markdown, or none"},
		&cli.StringFlag{Name: "release-url", Value: "https://releases.example.com/", Usage: "default release host, ending in / (a placeholder is fine)"},
		&cli.StringFlag{Name: "contact-url", Usage: "project contact URL (defaults to the repo URL)"},
		&cli.StringFlag{Name: "default-log-level", Value: "warn", Usage: "debug, info, warn, error, or none"},
		&cli.StringFlag{Name: "service-desc", Usage: "short service description"},
		&cli.StringFlag{Name: "service-default-port", Value: "8484", Usage: "HTTPS port (1–65535)"},
		&cli.StringFlag{Name: "author", Usage: "your copyright name (defaults to git user.name)"},
		&cli.StringFlag{Name: "year", Value: strconv.Itoa(time.Now().Year()), Usage: "copyright year"},
		&cli.BoolFlag{Name: "branch", Value: true, Usage: "create a setup branch (--branch=false to stay here)"},
		&cli.BoolFlag{Name: "commit", Value: true, Usage: "offer a setup commit (--commit=false to leave the changes for review)"},
		&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "use defaults and accept the preview, branch, and commit without prompts"},
		&cli.BoolFlag{Name: "skip-verify", Usage: "skip the generated project's tests and dev build"},
		&cli.BoolFlag{Name: "preview", Usage: "show the plan without changing the checkout"},
	}
}

func FromCommand(cmd *cli.Command) Options {
	o := Options{
		Module: cmd.String("module"), AppName: cmd.String("app-name"),
		Service: cmd.String("service"), Update: cmd.String("update"), Docs: cmd.String("docs"),
		ReleaseURL: cmd.String("release-url"), ContactURL: cmd.String("contact-url"),
		LogLevel: cmd.String("default-log-level"), ServiceDesc: cmd.String("service-desc"), Port: cmd.String("service-default-port"),
		Author: cmd.String("author"), Year: cmd.String("year"),
		Branch: cmd.Bool("branch"), Commit: cmd.Bool("commit"), Yes: cmd.Bool("yes"),
		SkipVerify: cmd.Bool("skip-verify"), Preview: cmd.Bool("preview"), Set: make(map[string]bool),
	}
	for _, flag := range Flags() {
		name := flag.Names()[0]
		o.Set[name] = cmd.IsSet(name)
	}
	return o
}

// Only choices live here. The cutter expands prerequisite removals itself.
func featureCuts(service, update string) ([]string, error) {
	var cuts []string
	switch service {
	case "none":
		cuts = append(cuts, "service")
	case "headless":
		cuts = append(cuts, "service.https")
	case "https":
	default:
		return nil, fmt.Errorf("--service needs none, headless, or https")
	}
	switch update {
	case "none":
		cuts = append(cuts, "update")
	case "check":
		cuts = append(cuts, "update.apply")
	case "manual":
		cuts = append(cuts, "update.apply.auto")
	case "auto":
		if service == "none" {
			return nil, fmt.Errorf("unattended updates need a service; pick --update=manual or keep a service")
		}
	default:
		return nil, fmt.Errorf("--update needs none, check, manual, or auto")
	}
	return cuts, nil
}

func moduleFromRemote(remote string) string {
	remote = strings.TrimSpace(remote)
	if strings.HasPrefix(remote, "git@github.com:") {
		remote = "https://github.com/" + strings.TrimPrefix(remote, "git@github.com:")
	}
	u, err := url.Parse(remote)
	if err != nil || !strings.EqualFold(u.Hostname(), "github.com") || u.Port() != "" ||
		(u.Scheme != "https" && u.Scheme != "ssh") || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	p := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
	parts := strings.Split(p, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	m := "github.com/" + p
	if module.CheckPath(m) != nil {
		return ""
	}
	return m
}

func appNameFromModule(m string) string {
	var name strings.Builder
	for _, r := range strings.ToLower(path.Base(m)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			name.WriteRune(r)
		}
	}
	return strings.Trim(name.String(), "-")
}

var appNamePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)

func textLine(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("can't be empty")
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("keep this to one line without control characters")
	}
	return nil
}

func plainValue(value string) error {
	if err := textLine(value); err != nil {
		return err
	}
	// build.sh also places these values in single-quoted Go linker arguments.
	if strings.ContainsAny(value, "'\"$`\\") {
		return fmt.Errorf("keep this to one line, without quotes, backslashes, $ or backticks")
	}
	return nil
}

func webURL(value string) error {
	if err := plainValue(value); err != nil {
		return err
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || strings.ContainsAny(value, " \t") {
		return fmt.Errorf("use a full http:// or https:// URL without login details")
	}
	return nil
}

func (o Options) validate() error {
	if err := module.CheckPath(o.Module); err != nil {
		return fmt.Errorf("--module: %w", err)
	}
	if o.Module == "sprout" {
		return fmt.Errorf("pick your own --module path")
	}
	if !appNamePattern.MatchString(o.AppName) {
		return fmt.Errorf("--app-name must match [A-Za-z0-9_][A-Za-z0-9._-]*")
	}
	if _, err := featureCuts(o.Service, o.Update); err != nil {
		return err
	}
	if o.Docs != "keep" && o.Docs != "markdown" && o.Docs != "none" {
		return fmt.Errorf("--docs needs keep, markdown, or none")
	}
	if err := webURL(o.ReleaseURL); err != nil {
		return fmt.Errorf("--release-url: %w", err)
	}
	u, _ := url.Parse(o.ReleaseURL)
	if !strings.HasSuffix(o.ReleaseURL, "/") || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("--release-url needs a trailing / and no query or fragment")
	}
	if err := webURL(o.ContactURL); err != nil {
		return fmt.Errorf("--contact-url: %w", err)
	}
	switch o.LogLevel {
	case "debug", "info", "warn", "error", "none":
	default:
		return fmt.Errorf("--default-log-level needs debug, info, warn, error, or none")
	}
	if o.Service != "none" {
		if err := plainValue(o.ServiceDesc); err != nil {
			return fmt.Errorf("--service-desc: %w", err)
		}
	} else if o.Set["service-desc"] || o.Set["service-default-port"] {
		return fmt.Errorf("service settings need a service; drop --service-desc and --service-default-port")
	}
	if o.Service == "https" {
		port, err := strconv.Atoi(o.Port)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("--service-default-port needs a number from 1 to 65535")
		}
	} else if o.Set["service-default-port"] {
		return fmt.Errorf("--service-default-port needs --service=https")
	}
	if err := textLine(o.Author); err != nil {
		return fmt.Errorf("--author: %w", err)
	}
	year, err := strconv.Atoi(o.Year)
	if err != nil || year < 1000 || year > 9999 {
		return fmt.Errorf("--year needs a four-digit year")
	}
	return nil
}
