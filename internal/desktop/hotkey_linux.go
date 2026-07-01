// hotkey_linux.go — Linux desktop hotkey parsing and validation.
// Parses one explicit global show/hide shortcut from desktop config and maps
// it to the Linux X11 hotkey package.
// Layer: transport config/runtime. Dependencies: golang.design/x/hotkey.
//go:build linux

package desktop

import (
	"fmt"
	"strconv"
	"strings"

	hotkey "golang.design/x/hotkey"
)

// HotkeySpec is the parsed form of one configured desktop shortcut.
//
// WHAT:  Holds Linux hotkey modifiers and one trigger key.
// WHY:   Config parsing and runtime registration should share one normalized form.
// PARAMS: Modifiers — X11 modifier mask list; Key — trigger key; Display — normalized human-readable shortcut.
type HotkeySpec struct {
	Modifiers []hotkey.Modifier
	Key       hotkey.Key
	Display   string
}

func validateHotkeyConfig(cfg HotkeyConfig) error {
	_, err := ParseHotkeyConfig(cfg)
	return err
}

// ParseHotkeyConfig parses one configured desktop shortcut.
//
// WHAT:  Validates and normalizes the optional desktop toggle hotkey config.
// WHY:   Startup should fail fast on invalid explicit shortcuts.
// PARAMS: cfg — desktop hotkey config block.
// RETURNS: HotkeySpec — parsed hotkey; error on invalid or incomplete input.
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
	return parseHotkeyShortcut(shortcut)
}

func parseHotkeyShortcut(shortcut string) (HotkeySpec, error) {
	parts := strings.Split(shortcut, "+")
	if len(parts) < 2 {
		return HotkeySpec{}, fmt.Errorf("toggle_hotkey.shortcut must include at least one modifier and one key: %s", shortcut)
	}
	modifiers := make([]hotkey.Modifier, 0, len(parts)-1)
	seenMods := map[hotkey.Modifier]struct{}{}
	keySet := false
	var key hotkey.Key
	keyName := ""
	for _, part := range parts {
		token := normalizeHotkeyToken(part)
		if token == "" {
			return HotkeySpec{}, fmt.Errorf("toggle_hotkey.shortcut contains an empty token: %s", shortcut)
		}
		if modifier, ok := parseHotkeyModifier(token); ok {
			if _, seen := seenMods[modifier]; !seen {
				seenMods[modifier] = struct{}{}
				modifiers = append(modifiers, modifier)
			}
			continue
		}
		if keySet {
			return HotkeySpec{}, fmt.Errorf("toggle_hotkey.shortcut must contain exactly one non-modifier key: %s", shortcut)
		}
		parsedKey, parsedName, ok := parseHotkeyKey(token)
		if !ok {
			return HotkeySpec{}, fmt.Errorf("toggle_hotkey.shortcut uses an unsupported key token %q", part)
		}
		key = parsedKey
		keyName = parsedName
		keySet = true
	}
	if len(modifiers) == 0 {
		return HotkeySpec{}, fmt.Errorf("toggle_hotkey.shortcut must include at least one modifier: %s", shortcut)
	}
	if !keySet {
		return HotkeySpec{}, fmt.Errorf("toggle_hotkey.shortcut must include one trigger key: %s", shortcut)
	}
	return HotkeySpec{Modifiers: modifiers, Key: key, Display: formatHotkeyDisplay(modifiers, keyName)}, nil
}

func normalizeHotkeyToken(token string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(token), " ", ""))
}

func parseHotkeyModifier(token string) (hotkey.Modifier, bool) {
	switch token {
	case "CTRL", "CONTROL":
		return hotkey.ModCtrl, true
	case "SHIFT":
		return hotkey.ModShift, true
	case "ALT", "OPTION":
		return hotkey.Mod1, true
	case "SUPER", "WIN", "WINDOWS", "META", "CMD", "COMMAND":
		return hotkey.Mod4, true
	default:
		return 0, false
	}
}

func parseHotkeyKey(token string) (hotkey.Key, string, bool) {
	if len(token) == 1 {
		if token[0] >= 'A' && token[0] <= 'Z' {
			return hotkey.Key(int(hotkey.KeyA) + int(token[0]-'A')), token, true
		}
		if token[0] >= '0' && token[0] <= '9' {
			offset := (int(token[0]-'1') + 10) % 10
			return hotkey.Key(int(hotkey.Key1) + offset), token, true
		}
	}
	if strings.HasPrefix(token, "F") {
		n, err := strconv.Atoi(strings.TrimPrefix(token, "F"))
		if err == nil && n >= 1 && n <= 12 {
			return hotkey.Key(int(hotkey.KeyF1) + (n - 1)), fmt.Sprintf("F%d", n), true
		}
	}
	switch token {
	case "SPACE":
		return hotkey.KeySpace, "Space", true
	case "ENTER", "RETURN":
		return hotkey.KeyReturn, "Enter", true
	case "ESC", "ESCAPE":
		return hotkey.KeyEscape, "Escape", true
	case "TAB":
		return hotkey.KeyTab, "Tab", true
	case "DELETE", "DEL":
		return hotkey.KeyDelete, "Delete", true
	default:
		return 0, "", false
	}
}

func formatHotkeyDisplay(modifiers []hotkey.Modifier, keyName string) string {
	parts := make([]string, 0, len(modifiers)+1)
	for _, modifier := range modifiers {
		switch modifier {
		case hotkey.ModCtrl:
			parts = append(parts, "Ctrl")
		case hotkey.ModShift:
			parts = append(parts, "Shift")
		case hotkey.Mod1:
			parts = append(parts, "Alt")
		case hotkey.Mod4:
			parts = append(parts, "Super")
		}
	}
	parts = append(parts, keyName)
	return strings.Join(parts, "+")
}
