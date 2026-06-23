package helpers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/config"
)

// fakeLookup returns a function that reports helpers by name as available or not.
func fakeLookup(availableNames []string) LookupFunc {
	avail := make(map[string]bool)
	for _, n := range availableNames {
		avail[n] = true
	}
	return func(name string) (string, error) {
		if avail[name] {
			return fmt.Sprintf("/usr/bin/%s", name), nil
		}
		return "", errors.New("not found")
	}
}

// TestDetectAllAvailable verifies all helpers report Available when present.
func TestDetectAllAvailable(t *testing.T) {
	names := make([]string, len(Known))
	for i, h := range Known {
		names[i] = h.Name
	}
	statuses := Detect(fakeLookup(names))
	if len(statuses) != len(Known) {
		t.Fatalf("Detect() returned %d statuses, want %d", len(statuses), len(Known))
	}
	for _, s := range statuses {
		if !s.Available {
			t.Errorf("Detect() helper %q not Available when it should be", s.Name)
		}
	}
}

// TestDetectNoneAvailable verifies all helpers report unavailable when missing.
func TestDetectNoneAvailable(t *testing.T) {
	statuses := Detect(fakeLookup(nil))
	for _, s := range statuses {
		if s.Available {
			t.Errorf("Detect() helper %q is Available when it should not be", s.Name)
		}
	}
}

// TestBuildPromptSectionAvailableCore verifies available core helpers appear.
func TestBuildPromptSectionAvailableCore(t *testing.T) {
	statuses := Detect(fakeLookup([]string{"rg", "fd", "jq", "git", "curl"}))
	section := BuildPromptSection(statuses, "/tmp", config.HelperSetup{})
	if !strings.Contains(section, "## Host Environment Helpers") {
		t.Error("expected Host Environment Helpers section")
	}
	if !strings.Contains(section, "rg") || !strings.Contains(section, "jq") {
		t.Error("expected rg and jq in available section")
	}
	if strings.Contains(section, "Optional Host Environment Helpers") {
		t.Error("unexpected Optional Host Environment Helpers when core helpers are available")
	}
}

// TestBuildPromptSectionMissingCoreNotDismissed verifies missing core helpers appear when not dismissed.
func TestBuildPromptSectionMissingCoreNotDismissed(t *testing.T) {
	statuses := Detect(fakeLookup([]string{"git", "curl"}))
	section := BuildPromptSection(statuses, "/tmp", config.HelperSetup{})
	if !strings.Contains(section, "Optional Host Environment Helpers") {
		t.Error("expected Optional Host Environment Helpers section when core helpers are missing")
	}
	if !strings.Contains(section, "rg") {
		t.Error("expected rg in optional section")
	}
}

// TestBuildPromptSectionMissingCoreDismissed verifies missing core helpers suppressed when dismissed.
func TestBuildPromptSectionMissingCoreDismissed(t *testing.T) {
	statuses := Detect(fakeLookup([]string{"git", "curl"}))
	section := BuildPromptSection(statuses, "/tmp", config.HelperSetup{Dismissed: true})
	if strings.Contains(section, "Optional Host Environment Helpers") {
		t.Error("unexpected Optional Host Environment Helpers when dismissed=true")
	}
	if !strings.Contains(section, "Host Environment Helpers") {
		t.Error("expected Host Environment Helpers for git and curl")
	}
}

// TestBuildPromptSectionDeclinedHelper verifies declined helpers do not appear in optional.
func TestBuildPromptSectionDeclinedHelper(t *testing.T) {
	statuses := Detect(fakeLookup([]string{"curl"}))
	section := BuildPromptSection(statuses, "/tmp", config.HelperSetup{
		Declined: []string{"rg", "fd"},
	})
	if strings.Contains(section, "rg") || strings.Contains(section, "fd") {
		t.Error("expected declined helpers rg and fd to be excluded from optional section")
	}
	if strings.Contains(section, "jq") {
		// jq is not declined, should appear.
	} else {
		t.Error("expected non-declined jq to appear in optional section")
	}
}

// TestBuildPromptSectionNoHelpersDismissed verifies empty section when nothing to show.
func TestBuildPromptSectionNoHelpersDismissed(t *testing.T) {
	statuses := Detect(fakeLookup(nil))
	section := BuildPromptSection(statuses, "/tmp", config.HelperSetup{Dismissed: true})
	if section != "" {
		t.Errorf("expected empty section, got: %s", section)
	}
}

// TestBuildPromptSectionContextualGo verifies contextual go appears only with go.mod.
func TestBuildPromptSectionContextualGo(t *testing.T) {
	dir := t.TempDir()
	statuses := Detect(fakeLookup([]string{"rg", "go"}))

	section := BuildPromptSection(statuses, dir, config.HelperSetup{})
	if strings.Contains(section, "go") {
		t.Error("expected go NOT to appear without go.mod in workdir")
	}

	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	section = BuildPromptSection(statuses, dir, config.HelperSetup{})
	if !strings.Contains(section, "go") {
		t.Error("expected go to appear with go.mod in workdir")
	}
}

// TestBuildPromptSectionContextualNode verifies contextual node appears only with package.json.
func TestBuildPromptSectionContextualNode(t *testing.T) {
	dir := t.TempDir()
	statuses := Detect(fakeLookup([]string{"rg", "node", "npm"}))

	section := BuildPromptSection(statuses, dir, config.HelperSetup{})
	if strings.Contains(section, "node") {
		t.Error("expected node NOT to appear without package.json")
	}

	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
	section = BuildPromptSection(statuses, dir, config.HelperSetup{})
	if !strings.Contains(section, "node") {
		t.Error("expected node to appear with package.json")
	}
	if !strings.Contains(section, "npm") {
		t.Error("expected npm to appear with package.json")
	}
}

// TestProjectRelevant verifies core is always relevant and contextual depends on files.
func TestProjectRelevant(t *testing.T) {
	dir := t.TempDir()

	for _, h := range Known {
		if h.Kind == KindCore {
			if !ProjectRelevant(dir, h) {
				t.Errorf("expected core helper %q always relevant", h.Name)
			}
		} else {
			if ProjectRelevant(dir, h) {
				t.Errorf("expected contextual helper %q not relevant without project files", h.Name)
			}
		}
	}

	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x"), 0644)
	goHelper := Known[5] // go
	if !ProjectRelevant(dir, goHelper) {
		t.Error("expected go relevant with go.mod present")
	}
}

// TestIsDeclined verifies declined name lookup.
func TestIsDeclined(t *testing.T) {
	if !IsDeclined("rg", []string{"rg", "fd"}) {
		t.Error("expected rg to be declined")
	}
	if IsDeclined("jq", []string{"rg", "fd"}) {
		t.Error("expected jq not to be declined")
	}
	if IsDeclined("rg", nil) {
		t.Error("expected no-op on nil declined list")
	}
}
