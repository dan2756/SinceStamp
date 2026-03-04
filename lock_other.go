//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package main

// lockFile is a no-op where flock(2) isn't available. Writes are still
// atomic, but two marks racing each other can lose one of them.
func lockFile(path string) (unlock func(), err error) {
	return func() {}, nil
}
