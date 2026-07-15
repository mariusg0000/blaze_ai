// testmain_test.go — process-wide test isolation for runtime tests.
// Purpose: keep runtime persistence tests away from the user's app home.
// Layer: test harness. Dependencies: Go testing and operating-system environment.
package runtime

import (
	"os"
	"testing"
)

// TestMain runs runtime tests with an isolated user home.
//
// WHAT: Prevents runtime tests from writing modes.json to the real app home.
// HOW: Creates a temporary directory, assigns it to HOME for the whole package
// process, runs the tests, and removes the directory afterward.
func TestMain(m *testing.M) {
	testHome, err := os.MkdirTemp("", "blazeai-runtime-tests-")
	if err != nil {
		panic("cannot create isolated runtime test home: " + err.Error())
	}
	if err := os.Setenv("HOME", testHome); err != nil {
		_ = os.RemoveAll(testHome)
		panic("cannot set isolated runtime test home: " + err.Error())
	}
	exitCode := m.Run()
	_ = os.RemoveAll(testHome)
	os.Exit(exitCode)
}
