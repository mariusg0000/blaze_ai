// testmain_test.go — process-wide test isolation for configuration tests.
// Purpose: keep configuration persistence tests away from the user's app home.
// Layer: test harness. Dependencies: Go testing and operating-system environment.
package config

import (
	"os"
	"testing"
)

// TestMain runs configuration tests with an isolated user home.
//
// WHAT: Prevents tests from writing config.json or modes.json to the real app home.
// HOW: Creates a temporary directory, assigns it to HOME for the whole package
// process, runs the tests, and removes the directory afterward.
func TestMain(m *testing.M) {
	testHome, err := os.MkdirTemp("", "blazeai-config-tests-")
	if err != nil {
		panic("cannot create isolated config test home: " + err.Error())
	}
	if err := os.Setenv("HOME", testHome); err != nil {
		_ = os.RemoveAll(testHome)
		panic("cannot set isolated config test home: " + err.Error())
	}
	exitCode := m.Run()
	_ = os.RemoveAll(testHome)
	os.Exit(exitCode)
}
