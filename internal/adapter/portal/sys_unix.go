//go:build !windows

package portal

import "syscall"

func closeFD(fd int) error {
	if fd < 0 {
		return nil
	}
	return syscall.Close(fd)
}
