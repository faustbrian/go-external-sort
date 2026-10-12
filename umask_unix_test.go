//go:build unix

package externalsort

import "syscall"

// Used only in the isolated helper process: umask is process-global and must
// not race parallel tests in the parent. The returned function restores it.
func setRestrictiveUmask() func() {
	previous := syscall.Umask(0o777)

	return func() {
		syscall.Umask(previous)
	}
}
