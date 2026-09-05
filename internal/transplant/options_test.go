package transplant

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestFeatureChoices(t *testing.T) {
	for _, tc := range []struct {
		service, update string
		cuts            []string
	}{
		{"https", "none", []string{"update"}},
		{"https", "check", []string{"update.apply"}},
		{"https", "manual", []string{"update.apply.auto"}},
		{"https", "auto", nil},
		{"none", "manual", []string{"service", "update.apply.auto"}},
		{"headless", "auto", []string{"service.https"}},
	} {
		got, err := featureCuts(tc.service, tc.update)
		if err != nil || !slices.Equal(got, tc.cuts) {
			t.Errorf("%s/%s = %v, %v; want %v", tc.service, tc.update, got, err, tc.cuts)
		}
	}
	for _, tc := range [][2]string{{"none", "auto"}, {"worker", "manual"}, {"https", "self"}} {
		if _, err := featureCuts(tc[0], tc[1]); err == nil {
			t.Errorf("accepted %v", tc)
		}
	}
}

func TestModuleFromRemote(t *testing.T) {
	for _, remote := range []string{"git@github.com:Some-Owner/MyApp.git", "https://github.com/Some-Owner/MyApp.git", "ssh://git@github.com/Some-Owner/MyApp.git", "https://GITHUB.COM/Some-Owner/MyApp"} {
		if got := moduleFromRemote(remote); got != "github.com/Some-Owner/MyApp" {
			t.Errorf("%q = %q", remote, got)
		}
	}
	for _, remote := range []string{"/tmp/repo", "https://gitlab.com/me/app", "https://github.com/me/app/tree/main", "https://github.com/me/app?token=x", "https://github.com/me/../app", "ssh://git@github.com:22/me/app"} {
		if got := moduleFromRemote(remote); got != "" {
			t.Errorf("accepted %q as %q", remote, got)
		}
	}
	if got := appNameFromModule("github.com/Some-Owner/My_App.v2"); got != "myappv2" {
		t.Fatal(got)
	}
}

const contractFixture = `{"version":1,"features":[{"name":"update","prerequisites":[]},{"name":"update.apply","prerequisites":["update"]},{"name":"update.apply.auto","prerequisites":["update.apply","service"]},{"name":"service","prerequisites":[]},{"name":"service.https","prerequisites":["service"]}]}`

func TestContractFailsClosed(t *testing.T) {
	if err := validateContract([]byte(contractFixture)); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{
		strings.Replace(contractFixture, `"version":1`, `"version":2`, 1),
		strings.Replace(contractFixture, `"name":"update"`, `"name":"unknown"`, 1),
		strings.Replace(contractFixture, `["update.apply","service"]`, `["update.apply"]`, 1),
		strings.Replace(contractFixture, `["update.apply","service"]`, `["update.apply","service","service"]`, 1),
		strings.Replace(contractFixture, `"name":"service"`, `"name":"update"`, 1),
		strings.Replace(contractFixture, `"version":1`, `"version":1,"surprise":true`, 1),
		contractFixture + "hello", contractFixture + contractFixture, `{}`, `[]`,
	} {
		if err := validateContract([]byte(data)); err == nil {
			t.Errorf("accepted %s", data)
		}
	}
	var decoded any
	if err := json.Unmarshal([]byte(contractFixture), &decoded); err != nil {
		t.Fatal(err)
	}
}

func validOptions() Options {
	return Options{Module: "example.com/me/app", AppName: "app", Service: "https", Update: "manual", Docs: "markdown", ReleaseURL: "https://releases.example.com/", ContactURL: "https://example.com/me/app", LogLevel: "warn", ServiceDesc: "app worker", Port: "8484", Author: "A Person", Year: "2026", Yes: true, SkipVerify: true, Set: map[string]bool{}}
}

func TestOptionValidation(t *testing.T) {
	if err := validOptions().validate(); err != nil {
		t.Fatal(err)
	}
	withApostrophe := validOptions()
	withApostrophe.Author = "Avery O'Neill"
	if err := withApostrophe.validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Options){
		"linker quote":  func(o *Options) { o.ServiceDesc = "Avery's worker" },
		"shell":         func(o *Options) { o.ServiceDesc = "$(touch /tmp/nope)" },
		"newline":       func(o *Options) { o.Author = "Person\nCopyright 1999 Someone" },
		"port":          func(o *Options) { o.Port = "65536" },
		"name":          func(o *Options) { o.AppName = "../app" },
		"url":           func(o *Options) { o.ReleaseURL = "https://example.com" },
		"query":         func(o *Options) { o.ReleaseURL = "https://example.com/?a=/" },
		"log":           func(o *Options) { o.LogLevel = "verbose" },
		"year":          func(o *Options) { o.Year = "now" },
		"docs":          func(o *Options) { o.Docs = "site" },
		"module":        func(o *Options) { o.Module = "../oops" },
		"headless port": func(o *Options) { o.Service = "headless"; o.Set["service-default-port"] = true },
	} {
		t.Run(name, func(t *testing.T) {
			o := validOptions()
			mutate(&o)
			if err := o.validate(); err == nil {
				t.Fatal("accepted invalid options")
			}
		})
	}
}
