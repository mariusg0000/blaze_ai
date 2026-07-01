// hotkey.go — desktop backend hotkey config validation.
// Validates only the persisted JSON shape used by the desktop config. Native
// registration is owned by Electron, not by the Go backend.
// Layer: desktop backend config validation. Dependencies: standard library only.
package desktopbackend

import (
	"fmt"
	"strings"
)

// HotkeySpec is a normalized persisted desktop hotkey value.
//
// WHAT:  Carries the normalized display form of the optional desktop shortcut.
// WHY:   The backend validates config text now; Electron will own native registration later.
// PARAMS: Display — normalized non-empty shortcut text.
type HotkeySpec struct {
	Display string
}

// validateHotkeyConfig checks that enabled hotkeys always provide a shortcut string.
func validateHotkeyConfig(cfg HotkeyConfig) error {
	_, err := ParseHotkeyConfig(cfg)
	return err
}

// ParseHotkeyConfig validates and normalizes the persisted desktop hotkey config.
//
// WHAT:  Checks one optional desktop shortcut text block.
// WHY:   Desktop config should fail fast on incomplete enabled hotkey settings.
// PARAMS: cfg — desktop hotkey config block.
// RETURNS: HotkeySpec — normalized shortcut wrapper; error on invalid input.
func ParseHotkeyConfig(cfg HotkeyConfig) (HotkeySpec, error) {
	shortcut := strings.TrimSpace(cfg.Shortcut)
	if !cfg.Enabled && shortcut == "" {
		return HotkeySpec{}, nil
	}
	if cfg.Enabled && shortcut == "" {
		return HotkeySpec{}, fmt.Errorf("toggle_hotkey.shortcut is required when toggle_hotkey.enabled is true")
	}
	if shortcut == "" {
		return HotkeySpec{}, nil
	}
	return HotkeySpec{Display: shortcut}, nil
}
