// apphome_readmes.go — embedded README templates for app_home bootstrap.
// Keeps concise folder guidance inside the binary so startup can materialize
// missing README.md files without relying on repository files at runtime.
// Layer: platform bootstrap. Dependencies: embed, io/fs, os, filepath.
package platform

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// embeddedAppHomeReadmes stores the bootstrap README.md files for app_home.
//
// WHAT:  Embeds the README templates shipped with the application.
// WHY:   app_home bootstrap must be able to copy them even from a standalone binary.
// HOW:   go:embed packages the readme tree under apphome_readmes/ into the binary.
// RETURNS: N/A.

//go:embed apphome_readmes/**
var embeddedAppHomeReadmes embed.FS

// copyMissingAppHomeReadmes writes embedded README.md files into app_home only when absent.
//
// WHAT:  Materializes shipped app_home README.md files on disk.
// WHY:   Users and the LLM need short folder guidance, but existing files must be preserved.
// HOW:   Walks the embedded tree, maps each README to its app_home destination, and writes only missing files.
// PARAMS: home — absolute path to app_home.
// RETURNS: error if embedded assets cannot be read or a missing README cannot be created.
func copyMissingAppHomeReadmes(home string) error {
	entries := []struct {
		embedded string
		target   string
	}{
		{embedded: "apphome_readmes/README.md", target: filepath.Join(home, "README.md")},
		{embedded: "apphome_readmes/backups/README.md", target: filepath.Join(home, "backups", "README.md")},
		{embedded: "apphome_readmes/config/README.md", target: filepath.Join(home, "config", "README.md")},
		{embedded: "apphome_readmes/projects/README.md", target: filepath.Join(home, "projects", "README.md")},
		{embedded: "apphome_readmes/scripts/README.md", target: filepath.Join(home, "scripts", "README.md")},
		{embedded: "apphome_readmes/skills/README.md", target: filepath.Join(home, "skills", "README.md")},
	}

	for _, entry := range entries {
		if _, err := os.Stat(entry.target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("cannot inspect %s: %w", entry.target, err)
		}

		data, err := fs.ReadFile(embeddedAppHomeReadmes, entry.embedded)
		if err != nil {
			return fmt.Errorf("cannot read embedded app home README %s: %w", entry.embedded, err)
		}
		content := strings.TrimSpace(string(data)) + "\n"
		if err := os.WriteFile(entry.target, []byte(content), 0644); err != nil {
			return fmt.Errorf("cannot create %s: %w", entry.target, err)
		}
	}

	return nil
}
