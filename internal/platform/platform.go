// platform.go — core platform detection and bootstrap logic.
// Provides OS identification, shell chain selection, app home resolution,
// and standard subfolder creation. No external dependencies.
// Layer: platform abstraction. Dependencies: none (standard library only).
package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// OS represents a supported operating system identifier.
// Valid values: "linux", "darwin", "windows".
type OS string

const (
	Linux   OS = "linux"
	Darwin  OS = "darwin"
	Windows OS = "windows"
)

// ErrUnsupportedOS is returned when runtime.GOOS is not linux, darwin, or windows.
var ErrUnsupportedOS = errors.New("unsupported operating system")

// ErrNoShell is returned when no shell binary from the chain is found on PATH.
var ErrNoShell = errors.New("no shell binary found on PATH")

// appHomeName is the folder name under the OS home directory.
const appHomeName = "blazeai"

// subfolders are the standard directories created under app home.
// scripts/venv is intentionally excluded: it is created lazily when Python is first needed.
var subfolders = []string{
	"skills",
	"memories",
	"scripts",
	"backups",
	"projects",
	"memory",
	"config",
}

// Detect returns the current operating system based on runtime.GOOS.
// Returns ErrUnsupportedOS if the OS is not linux, darwin, or windows.
//
// WHAT:  Identifies the current OS as one of the supported constants.
// WHY:   Shell selection and path handling depend on the operating system.
// HOW:   Maps runtime.GOOS to the OS type. Stops on unsupported values.
// RETURNS: OS — one of Linux, Darwin, Windows; error if unsupported.
func Detect() (OS, error) {
	switch runtime.GOOS {
	case "linux":
		return Linux, nil
	case "darwin":
		return Darwin, nil
	case "windows":
		return Windows, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedOS, runtime.GOOS)
	}
}

// ShellChain returns the ordered shell preference list for the given OS.
// Linux and Darwin: bash, then sh.
// Windows: pwsh, then powershell.exe, then cmd.exe.
//
// WHAT:  Returns the shell binary name chain for a given OS.
// WHY:   The runtime prefers OS-native shells and needs a fallback order.
// HOW:   Static lookup per OS constant. No filesystem checks here.
// PARAMS: os — the target operating system.
// RETURNS: []string — ordered list of shell binary names.
func ShellChain(os OS) []string {
	switch os {
	case Linux, Darwin:
		return []string{"bash", "sh"}
	case Windows:
		return []string{"pwsh", "powershell.exe", "cmd.exe"}
	default:
		return nil
	}
}

// SelectShell returns the first available shell binary from the chain for the given OS.
// Searches PATH using exec.LookPath. Returns the resolved binary path.
//
// WHAT:  Finds the first shell binary that exists on PATH for the given OS.
// WHY:   Tool execution needs a concrete shell to run commands.
// HOW:   Iterates ShellChain, calls exec.LookPath for each, returns first hit.
// PARAMS: os — the target operating system.
// RETURNS: string — resolved shell binary path; error if none found or OS unsupported.
func SelectShell(os OS) (string, error) {
	chain := ShellChain(os)
	if chain == nil {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedOS, os)
	}
	for _, name := range chain {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: tried %v", ErrNoShell, chain)
}

// AppHome resolves the blazeai application home directory.
// The app home is the "blazeai" folder inside the OS user home directory.
//
// WHAT:  Returns the absolute path to the blazeai app home directory.
// WHY:   All runtime data (config, sessions, skills, memory, memories) lives under app home.
// HOW:   Calls os.UserHomeDir and appends the appHomeName folder.
// RETURNS: string — absolute path to app home; error if home dir cannot be resolved.
func AppHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve user home directory: %w", err)
	}
	return filepath.Join(home, appHomeName), nil
}

// ProjectFolderName sanitizes a working directory path into a safe folder name.
// Slashes and colons become underscores, the result is lowercase, and leading/trailing underscores are stripped.
//
// WHAT:  Converts an OS path to a portable folder name for project-based session storage.
// WHY:   Each project (working directory) gets its own folder under app_home/projects/.
// HOW:   Replaces path separators and colons with underscores, lowercases, trims edge underscores.
// PARAMS: workDir — absolute working directory path.
// RETURNS: string — sanitized folder name (e.g., "mnt_data_work_ai_projects_blazeai").
func ProjectFolderName(workDir string) string {
	name := filepath.ToSlash(workDir)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ToLower(name)
	name = strings.Trim(name, "_")
	return name
}

// ProjectDir returns the absolute path to the project folder for a given working directory.
// The project folder is app_home/projects/<sanitized_workdir>/
//
// WHAT:  Resolves the project-specific directory under app home.
// WHY:   Sessions are grouped by project (working directory) per spec.
// PARAMS: workDir — absolute working directory path.
// RETURNS: string — absolute path to the project folder; error if app home cannot be resolved.
func ProjectDir(workDir string) (string, error) {
	home, err := AppHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "projects", ProjectFolderName(workDir)), nil
}

// EnsureProjectDir creates the project folder and its sessions subfolder if missing.
//
// WHAT:  Ensures the project-specific directory structure exists on disk.
// WHY:   New sessions need a project folder with a sessions/ subfolder.
// PARAMS: workDir — absolute working directory path.
// RETURNS: string — absolute path to the project sessions directory; error if creation fails.
func EnsureProjectDir(workDir string) (string, error) {
	projectDir, err := ProjectDir(workDir)
	if err != nil {
		return "", err
	}
	sessionsDir := filepath.Join(projectDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create project sessions dir %s: %w", sessionsDir, err)
	}
	return sessionsDir, nil
}

// Bootstrap creates the app home directory and all standard subfolders if missing.
// Also removes the legacy sessions/ directory if present (migrated to projects/).
// Does NOT create scripts/venv — that is created lazily when Python is first needed.
//
// WHAT:  Ensures app home and standard subfolders exist on disk.
// WHY:   The runtime needs these directories to store config, skills, memory, etc.
// HOW:   Calls os.MkdirAll on app home and each subfolder with 0755 permissions.
// RETURNS: error if any directory creation fails.
func Bootstrap() error {
	home, err := AppHome()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0755); err != nil {
		return fmt.Errorf("cannot create app home %s: %w", home, err)
	}

	// Remove legacy sessions/ directory (migrated to projects/).
	legacySessions := filepath.Join(home, "sessions")
	if _, statErr := os.Stat(legacySessions); statErr == nil {
		_ = os.RemoveAll(legacySessions)
	}

	for _, sub := range subfolders {
		dir := filepath.Join(home, sub)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create %s: %w", dir, err)
		}
	}
	return nil
}

// OSInfo returns a human-readable OS description (name and version when available).
//
// WHAT:  Produces a concise OS identifier for prompt injection.
// WHY:   The LLM needs to know the exact OS environment for correct shell commands.
// HOW:   Linux: reads /etc/os-release for PRETTY_NAME; macOS: uses sw_vers; Windows: uses ver.
//
//	Falls back to runtime.GOOS if detection fails.
//
// RETURNS: string — human-readable OS description.
func OSInfo() string {
	switch runtime.GOOS {
	case "linux":
		return linuxOSInfo()
	case "darwin":
		return darwinOSInfo()
	case "windows":
		return windowsOSInfo()
	default:
		return runtime.GOOS
	}
}

// linuxOSInfo returns the OS name from /etc/os-release,
// falling back to bare "Linux" on any error.
func linuxOSInfo() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			val := strings.TrimPrefix(line, "PRETTY_NAME=")
			val = strings.Trim(val, `"`)
			if val != "" {
				return val
			}
		}
	}
	return "Linux"
}

// darwinOSInfo returns the macOS name and version via sw_vers,
// falling back to "macOS" on any error.
func darwinOSInfo() string {
	cmd := exec.Command("sw_vers", "-productName")
	out, err := cmd.Output()
	name := ""
	if err == nil {
		name = strings.TrimSpace(string(out))
	}
	cmd = exec.Command("sw_vers", "-productVersion")
	out, err = cmd.Output()
	ver := ""
	if err == nil {
		ver = " " + strings.TrimSpace(string(out))
	}
	if name != "" {
		return name + ver
	}
	return "macOS"
}

// windowsOSInfo returns the Windows version via cmd /c ver,
// falling back to "Windows" on any error.
func windowsOSInfo() string {
	cmd := exec.Command("cmd", "/c", "ver")
	out, err := cmd.Output()
	if err != nil {
		return "Windows"
	}
	ver := strings.TrimSpace(string(out))
	if ver != "" {
		return ver
	}
	return "Windows"
}
