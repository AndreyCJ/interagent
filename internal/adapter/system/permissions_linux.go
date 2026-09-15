//go:build linux

package system

import (
	"errors"
	"fmt"
	"os/exec"

	"interagent/internal/port"
)

// Permissions on Linux are honest but consent-free: a native (non-sandboxed)
// app is not gated by the OS for microphone or system-audio capture (ADR-013
// amendment), so statuses reflect transport reachability — not a consent claim
// — and Request is a no-op for everything Linux can actually do.
type Permissions struct {
	micProbe func() error
}

func NewPermissions(micProbe func() error) *Permissions {
	return &Permissions{micProbe: micProbe}
}

func (p *Permissions) Status(perm port.Permission) (bool, error) {
	switch perm {
	case port.PermissionMicrophone:
		if p.micProbe == nil {
			return false, errors.New("mic probe unavailable")
		}
		return p.micProbe() == nil, nil
	case port.PermissionScreenCapture:
		// No OS gate for system audio on Linux; the capture layer reports
		// transport errors honestly when Start actually runs.
		return true, nil
	case port.PermissionAccessibility:
		return false, nil
	default:
		return false, fmt.Errorf("unknown permission %q", perm)
	}
}

func (p *Permissions) Request(perm port.Permission) error {
	switch perm {
	case port.PermissionMicrophone:
		if p.micProbe == nil {
			return errors.New("mic probe unavailable")
		}
		if err := p.micProbe(); err != nil {
			return errors.New("microphone unavailable: " + err.Error())
		}
		return nil // nothing to consent for a native app; pulse is not permission-gated
	case port.PermissionScreenCapture:
		return nil // no consent gate on Linux
	case port.PermissionAccessibility:
		return errors.New("accessibility permission is not supported on linux")
	}
	return fmt.Errorf("unknown permission %q", perm)
}

func (p *Permissions) OpenSettings(perm port.Permission) error {
	switch perm {
	case port.PermissionMicrophone, port.PermissionScreenCapture:
		if path, err := exec.LookPath("pavucontrol"); err == nil {
			return exec.Command(path).Start()
		}
		return errors.New("no audio settings UI found — install pavucontrol")
	case port.PermissionAccessibility:
		return errors.New("no accessibility settings UI on linux")
	}
	return fmt.Errorf("unknown permission %q", perm)
}
