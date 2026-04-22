//go:build linux

package main

import (
	"fmt"
	"os"
)

// notifyAlreadyRunning prints to stderr. On Linux, the app is typically
// launched from a terminal or .desktop file; both cases surface stderr.
func notifyAlreadyRunning() {
	fmt.Fprintln(os.Stderr, "CS Stats Tracker is already running.")
}
