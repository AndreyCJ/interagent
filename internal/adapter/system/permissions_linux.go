//go:build linux

package system

import (
	"context"
	"errors"

	"interagent/internal/port"
)

// screenCast is the slice of the portal adapter the permissions need.
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
	case port.PermissionAccessibility:
		return false, nil // honest: no Linux equivalent
	case port.PermissionScreenCapture:
		if p.portal != nil {
			return p.portal.Granted(), nil
		}
	case port.PermissionMicrophone:
		if p.micProbe == nil {
			return false, nil
		}
		return p.micProbe() == nil, nil
	}
	return false, nil
}

func (p *Permissions) Request(perm port.Permission) error {
	if perm == port.PermissionAccessibility {
		return errors.New("accessibility permission is not supported on linux")
	}
	return errors.New("permission request flow lands in Task 5")
}

func (p *Permissions) OpenSettings(perm port.Permission) error {
	return errors.New("settings flow lands in Task 5")
}
