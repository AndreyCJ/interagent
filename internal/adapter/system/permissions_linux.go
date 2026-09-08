//go:build linux

package system

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"interagent/internal/port"
)

// screenCast is the slice of the portal adapter the permissions need;
// satisfied by *portal.ScreenCast (Available/Grant/Granted).
type screenCast interface {
	Available(ctx context.Context) bool
	Grant(ctx context.Context) error
	Granted() bool
}

type Permissions struct {
	portal   screenCast
	micProbe func() error
}

func NewPermissions(portalAdapter screenCast, micProbe func() error) *Permissions {
	return &Permissions{portal: portalAdapter, micProbe: micProbe}
}

func (p *Permissions) Status(perm port.Permission) (bool, error) {
	switch perm {
	case port.PermissionMicrophone:
		if p.micProbe == nil {
			return false, errors.New("mic probe unavailable")
		}
		return p.micProbe() == nil, nil
	case port.PermissionScreenCapture:
		if p.portal == nil {
			return false, nil
		}
		return p.portal.Granted(), nil
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
		if p.portal == nil {
			return errors.New("screen cast portal unavailable")
		}
		return p.portal.Grant(context.Background()) // the picker IS the grant dialog
	case port.PermissionAccessibility:
		return errors.New("accessibility permission is not supported on linux")
	}
	return fmt.Errorf("unknown permission %q", perm)
}

func (p *Permissions) OpenSettings(perm port.Permission) error {
	switch perm {
	case port.PermissionMicrophone:
		if path, err := exec.LookPath("pavucontrol"); err == nil {
			return exec.Command(path).Start()
		}
		return errors.New("no microphone settings UI found — install pavucontrol")
	case port.PermissionScreenCapture:
		if p.portal == nil {
			return errors.New("screen cast portal unavailable")
		}
		return p.portal.Grant(context.Background()) // the picker is the settings UI
	case port.PermissionAccessibility:
		return errors.New("no accessibility settings UI on linux")
	}
	return fmt.Errorf("unknown permission %q", perm)
}
