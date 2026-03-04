//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockFile takes an exclusive flock(2) on path, creating it if needed, and
// blocks until the lock is free. The lock is released by calling unlock, or
// by the kernel if the process dies, so a crash never leaves a stale lock.
func lockFile(path string) (unlock func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("locking %s: %w", path, err)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
