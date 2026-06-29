package helpers

import (
	"errors"
	"fmt"
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

// TestAvailableCoreHelpers verifies available core helpers appear when detected.
func TestAvailableCoreHelpers(t *testing.T) {
	statuses := Detect(fakeLookup([]string{"rg", "fd", "jq", "git", "curl", "pandoc", "sqlite3"}))
	available := Available(statuses, "/tmp")
	if len(available) != 7 {
		t.Fatalf("Available() returned %d helpers, want 7", len(available))
	}
	if available[0].Name != "curl" || available[6].Name != "sqlite3" {
		t.Fatalf("Available() not sorted by name: first=%q last=%q", available[0].Name, available[6].Name)
	}
}

// TestMissingCoreHelpers verifies missing core helpers are returned when unavailable.
func TestMissingCoreHelpers(t *testing.T) {
	statuses := Detect(fakeLookup([]string{"git", "curl"}))
	missing := MissingCore(statuses, config.HelperSetup{})
	if len(missing) == 0 {
		t.Fatal("MissingCore() returned no helpers, want missing core helpers")
	}
	if missing[0].Name != "fd" || missing[len(missing)-1].Name != "sqlite3" {
		t.Fatalf("MissingCore() not sorted or incomplete: first=%q last=%q", missing[0].Name, missing[len(missing)-1].Name)
	}
}

// TestMissingCoreHelpersDeclined verifies declined helpers are excluded.
func TestMissingCoreHelpersDeclined(t *testing.T) {
	statuses := Detect(fakeLookup([]string{"curl"}))
	missing := MissingCore(statuses, config.HelperSetup{Declined: []string{"rg", "fd"}})
	for _, s := range missing {
		if s.Name == "rg" || s.Name == "fd" {
			t.Fatalf("MissingCore() included declined helper %q", s.Name)
		}
	}
}

// TestMissingCoreHelpersNoMissing verifies empty result when nothing is missing.
func TestMissingCoreHelpersNoMissing(t *testing.T) {
	statuses := Detect(fakeLookup([]string{"rg", "fd", "jq", "git", "curl", "pandoc", "sqlite3"}))
	missing := MissingCore(statuses, config.HelperSetup{})
	if len(missing) != 0 {
		t.Fatalf("MissingCore() returned %d helpers, want 0", len(missing))
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
