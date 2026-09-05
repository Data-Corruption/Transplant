package transplant

import (
	"bytes"
	"strings"
	"testing"
)

const buildFixture = `#!/usr/bin/env bash
SERVICE_DESC="" # fallback for after cut
SERVICE_DEFAULT_PORT="0" # fallback for after cut
# Project config --------------------------------------------------------------
APP_NAME="sprout"
RELEASE_URL="https://example.com/"
CONTACT_URL="https://example.com/"
DEFAULT_LOG_LEVEL="warn"
# --- BEGIN service ---
SERVICE_DESC="Sprout daemon"
# --- END service ---
# --- BEGIN service.https ---
SERVICE_DEFAULT_PORT="8484"
# --- END service.https ---
# Pinned build inputs ---------------------------------------------------------
APP_NAME="outside-the-block"
`

func TestProjectEditorPreservesSurroundingBytes(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		original := strings.ReplaceAll(buildFixture, "\n", newline)
		got, err := editProject([]byte(original), map[string]string{"APP_NAME": "fern", "SERVICE_DESC": "Fern worker"})
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Replace(original, `APP_NAME="sprout"`, `APP_NAME="fern"`, 1)
		want = strings.Replace(want, `SERVICE_DESC="Sprout daemon"`, `SERVICE_DESC="Fern worker"`, 1)
		if string(got) != want {
			t.Fatalf("unexpected edit:\n%s", got)
		}
	}
}

func TestProjectEditorRejectsMissingDuplicateAndUnsafeValues(t *testing.T) {
	for _, source := range []string{
		strings.Replace(buildFixture, `APP_NAME="sprout"`, `export APP_NAME="sprout"`, 1),
		strings.Replace(buildFixture, `APP_NAME="sprout"`, "APP_NAME=\"sprout\"\nAPP_NAME=\"twice\"", 1),
		strings.Replace(buildFixture, "# Project config", "# Settings", 1),
		strings.Replace(buildFixture, "# Pinned build inputs ---------------------------------------------------------", "# No rule", 1),
	} {
		if _, err := editProject([]byte(source), map[string]string{"APP_NAME": "fern"}); err == nil {
			t.Fatal("accepted changed anchors")
		}
	}
	for _, value := range []string{"$(id)", "`id`", "x\ny", `x"y`, `x\y`} {
		if _, err := editProject([]byte(buildFixture), map[string]string{"APP_NAME": value}); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestHeadlessDoesNotEditPortFallback(t *testing.T) {
	o := validOptions()
	o.Service = "headless"
	source := strings.Replace(buildFixture, "SERVICE_DEFAULT_PORT=\"8484\"\n", "", 1)
	got, err := editProject([]byte(source), o.projectValues())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(`SERVICE_DEFAULT_PORT="0" # fallback`)) {
		t.Fatal("changed fallback")
	}
}
