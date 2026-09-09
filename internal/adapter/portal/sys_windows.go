//go:build windows

package portal

import "errors"

func closeFD(fd int) error {
	return errors.New("pipewire fd close unsupported on windows")
}
