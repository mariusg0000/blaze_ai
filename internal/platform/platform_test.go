// platform_test.go — tests for OS detection, shell selection, and app home resolution.
// Tests run on the host OS and verify both deterministic and filesystem-dependent behavior.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDetect returns the current OS and verifies it matches runtime.GOOS.
func TestDetect(t *testing.T) {
	got, err := Detect()
	if err != nil {
		t.Fatalf("Detect() unexpected error: %v", err)
	}
	var want OS
	switch runtime.GOOS {
	case "linux":
		want = Linux
	case "darwin":
		want = Darwin
	case "windows":
		want = Windows
	default:
		t.Fatalf("test host has unsupported OS: %s", runtime.GOOS)
	}
	if got != want {
		t.Errorf("Detect() = %q, want %q", got, want)
	}
}

// TestShellChain verifies the correct chain for each supported OS.
func TestShellChain(t *testing.T) {
	tests := []struct {
		name string
		os   OS
		want []string
	}{
		{"linux", Linux, []string{"bash", "sh"}},
		{"darwin", Darwin, []string{"bash", "sh"}},
		{"windows", Windows, []string{"pwsh", "powershell.exe", "cmd.exe"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShellChain(tt.os)
			if len(got) != len(tt.want) {
				t.Errorf("ShellChain(%s) = %v, want %v", tt.os, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ShellChain(%s)[%d] = %q, want %q", tt.os, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestShellChainUnsupported verifies that an unknown OS returns nil.
func TestShellChainUnsupported(t *testing.T) {
	got := ShellChain("solaris")
	if got != nil {
		t.Errorf("ShellChain(unknown) = %v, want nil", got)
	}
}

// TestSelectShell verifies a shell is found on the host system.
func TestSelectShell(t *testing.T) {
	currentOS, _ := Detect()
	got, err := SelectShell(currentOS)
	if err != nil {
		t.Fatalf("SelectShell(%s) unexpected error: %v", currentOS, err)
	}
	if got == "" {
		t.Error("SelectShell() returned empty path")
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("SelectShell() returned path that does not exist: %s", got)
	}
}

// TestSelectShellUnsupported verifies error for unknown OS.
func TestSelectShellUnsupported(t *testing.T) {
	_, err := SelectShell("solaris")
	if err == nil {
		t.Fatal("SelectShell(unknown) expected error, got nil")
	}
}

// TestAppHome verifies the resolved path ends with the app home folder name.
func TestAppHome(t *testing.T) {
	got, err := AppHome()
	if err != nil {
		t.Fatalf("AppHome() unexpected error: %v", err)
	}
	if got == "" {
		t.Fatal("AppHome() returned empty path")
	}
	if !strings.HasSuffix(got, filepath.Join("blazeai")) {
		t.Errorf("AppHome() = %q, want path ending with 'blazeai'", got)
	}
}

// TestBootstrap verifies that app home and subfolders are created.
// This test is idempotent: it should succeed whether or not dirs already exist.
func TestBootstrap(t *testing.T) {
	if err := Bootstrap(); err != nil {
		t.Fatalf("Bootstrap() unexpected error: %v", err)
	}
	home, err := AppHome()
	if err != nil {
		t.Fatalf("AppHome() unexpected error: %v", err)
	}
	for _, sub := range subfolders {
		dir := filepath.Join(home, sub)
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("subfolder %s was not created: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("subfolder %s exists but is not a directory", sub)
		}
	}
}

// TestBootstrapIdempotent verifies that calling Bootstrap twice does not error.
func TestBootstrapIdempotent(t *testing.T) {
	if err := Bootstrap(); err != nil {
		t.Fatalf("first Bootstrap() failed: %v", err)
	}
	if err := Bootstrap(); err != nil {
		t.Fatalf("second Bootstrap() failed: %v", err)
	}
}

// TestOSInfo verifies that OSInfo returns a non-empty string matching the current OS.
func TestOSInfo(t *testing.T) {
	info := OSInfo()
	if info == "" {
		t.Fatal("OSInfo() returned empty string")
	}
	lower := strings.ToLower(info)
	switch runtime.GOOS {
	case "linux":
		if !strings.Contains(lower, "linux") && !strings.Contains(lower, "ubuntu") {
			t.Errorf("OSInfo() = %q, expected to contain 'linux' or 'ubuntu'", info)
		}
	case "darwin":
		if !strings.Contains(lower, "mac") && !strings.Contains(lower, "darwin") {
			t.Errorf("OSInfo() = %q, expected to contain 'mac' or 'darwin'", info)
		}
	case "windows":
		if !strings.Contains(lower, "windows") {
			t.Errorf("OSInfo() = %q, expected to contain 'windows'", info)
		}
	}
}
