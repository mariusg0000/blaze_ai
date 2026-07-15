// testmain_test.go — process-wide test isolation for console tests.
// Purpose: keep console-driven runtime persistence away from the user's app home.
// Layer: test harness. Dependencies: Go testing and operating-system environment.
package console

import (
	"os"
	"testing"
)

// TestMain runs console tests with an isolated user home.
//
// WHAT: Prevents console tests from writing runtime configuration to the real app home.
// HOW: Creates a temporary directory, assigns it to HOME for the whole package
// process, runs the tests, and removes the directory afterward.
func TestMain(m *testing.M) {
	testHome, err := os.MkdirTemp("", "blazeai-console-tests-")
	if err != nil {
		panic("cannot create isolated console test home: " + err.Error())
	}
	if err := os.Setenv("HOME", testHome); err != nil {
		_ = os.RemoveAll(testHome)
		panic("cannot set isolated console test home: " + err.Error())
	}
	exitCode := m.Run()
	_ = os.RemoveAll(testHome)
	os.Exit(exitCode)
}
