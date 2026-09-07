//go:build !darwin && !linux

package system

import (
	"errors"

	"interagent/internal/adapter/portal"
	"interagent/internal/port"
)

// Permissions on non-darwin platforms reports every permission as granted
// (no TCC) and OpenSettings as a no-op, matching the previous stub behaviour.
type Permissions struct{}

func NewPermissions(_ *portal.ScreenCast, _ func() error) *Permissions {
	return &Permissions{}
}

func (p *Permissions) Status(perm port.Permission) (bool, error) { return true, nil }
func (p *Permissions) Request(perm port.Permission) error {
	return errors.New("permission requests are not supported on this platform")
}
func (p *Permissions) OpenSettings(perm port.Permission) error { return nil }
