//go:build !linux

package audio

import "errors"

// PulseReachable is unused on non-linux platforms; it exists so the uniform
// system.NewPermissions call site in app.go compiles everywhere.
func PulseReachable() error { return errors.New("pulse probe unsupported on this platform") }
