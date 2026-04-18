// Package singleinstance ensures only one instance of the application runs at a time.
package singleinstance

import (
	"fmt"
	"net"
)

// Lock holds the resource that enforces single-instance execution.
type Lock struct {
	listener net.Listener
}

// Acquire tries to bind a loopback TCP port as a mutex. If another instance is
// already running, it returns ErrAlreadyRunning. The OS releases the port when
// the process exits, so crashes don't leave stale locks behind.
func Acquire(port int) (*Lock, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, ErrAlreadyRunning
	}
	return &Lock{listener: l}, nil
}

// Release frees the lock. Safe to call multiple times.
func (l *Lock) Release() {
	if l == nil || l.listener == nil {
		return
	}
	_ = l.listener.Close()
	l.listener = nil
}

// ErrAlreadyRunning is returned when another instance already holds the lock.
var ErrAlreadyRunning = fmt.Errorf("another instance is already running")
