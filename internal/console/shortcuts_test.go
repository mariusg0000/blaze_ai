// shortcuts_test.go — tests for console readline shortcut encoding.
// Verifies inputrc notation is converted to the raw control bytes used by the dispatcher.
// Layer: console transport tests. Dependencies: reeflective/readline inputrc package.
package console

import (
	"testing"

	"github.com/reeflective/readline/inputrc"
)

// TestShortcutSequencesDecodeToControlBytes prevents literal inputrc notation from being bound.
func TestShortcutSequencesDecodeToControlBytes(t *testing.T) {
	tests := map[string]byte{
		`\C-i`: 0x09,
		`\C-\`: 0x1c,
		`\C-f`: 0x06,
		`\C-r`: 0x12,
		`\C-t`: 0x14,
		`\C-]`: 0x1d,
	}
	for notation, want := range tests {
		got := inputrc.Unescape(notation)
		if len(got) != 1 || got[0] != want {
			t.Errorf("inputrc.Unescape(%q) = % x, want %02x", notation, got, want)
		}
	}
}
