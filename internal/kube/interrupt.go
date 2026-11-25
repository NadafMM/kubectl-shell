package kube

import (
	"os"
	"os/signal"
	"sync/atomic"
)

// replIgnoresInterrupt is set while the interactive shell owns SIGINT so the
// line editor can handle Ctrl+C as a key. Child kubectl must temporarily restore
// the default disposition or it inherits Ignore and cannot be interrupted.
var replIgnoresInterrupt atomic.Bool

// SetREPLInterruptIgnore toggles process-wide SIGINT ignore for the REPL.
func SetREPLInterruptIgnore(ignore bool) {
	if ignore {
		signal.Ignore(os.Interrupt)
		replIgnoresInterrupt.Store(true)
		return
	}
	signal.Reset(os.Interrupt)
	replIgnoresInterrupt.Store(false)
}

// withChildInterrupt restores default SIGINT for the duration of fn so a child
// kubectl can receive Ctrl+C, then re-applies Ignore when the REPL wants it.
func withChildInterrupt(fn func()) {
	signal.Reset(os.Interrupt)
	defer func() {
		if replIgnoresInterrupt.Load() {
			signal.Ignore(os.Interrupt)
		}
	}()
	fn()
}
