// testmain_test.go — process-wide test isolation for Telegram tests.
// Purpose: keep Telegram-driven runtime persistence away from the user's app home.
// Layer: test harness. Dependencies: Go testing and operating-system environment.
package telegram

import (
	"os"
	"testing"
)

// TestMain runs Telegram tests with an isolated user home.
//
// WHAT: Prevents Telegram tests from writing runtime configuration to the real app home.
// HOW: Creates a temporary directory, assigns it to HOME for the whole package
// process, runs the tests, and removes the directory afterward.
func TestMain(m *testing.M) {
	testHome, err := os.MkdirTemp("", "blazeai-telegram-tests-")
	if err != nil {
		panic("cannot create isolated Telegram test home: " + err.Error())
	}
	if err := os.Setenv("HOME", testHome); err != nil {
		_ = os.RemoveAll(testHome)
		panic("cannot set isolated Telegram test home: " + err.Error())
	}
	exitCode := m.Run()
	_ = os.RemoveAll(testHome)
	os.Exit(exitCode)
}
