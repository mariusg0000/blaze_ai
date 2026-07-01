// hotkey_other.go — non-Linux desktop hotkey validation stub.
// Rejects explicit desktop hotkey configuration on platforms that do not yet
// implement the native desktop transport extras.
// Layer: transport config/runtime.
//go:build !linux

package desktop

import "fmt"

type HotkeySpec struct{}

func validateHotkeyConfig(cfg HotkeyConfig) error {
	if !cfg.Enabled && cfg.Shortcut == "" {
		return nil
	}
	return fmt.Errorf("desktop toggle hotkey is supported only on Linux right now")
}

func ParseHotkeyConfig(cfg HotkeyConfig) (HotkeySpec, error) {
	if err := validateHotkeyConfig(cfg); err != nil {
		return HotkeySpec{}, err
	}
	return HotkeySpec{}, nil
}
