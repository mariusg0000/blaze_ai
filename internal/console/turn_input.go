// turn_input.go — platform-neutral turn cancellation input contract.
// Purpose: Defines the watcher used to detect ESC during an active console turn.
// Layer: console transport input boundary. Dependencies: os.
package console

// turnAbortWatcher monitors terminal input for the ESC key during one agent turn.
//
// WHAT: Exposes an abort event and a cleanup operation.
// HOW: Platform-specific implementations put the terminal into a temporary input mode.
type turnAbortWatcher struct {
	aborted <-chan struct{}
	stop    func()
}
