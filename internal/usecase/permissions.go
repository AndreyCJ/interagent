package usecase

import "interagent/internal/port"

type Permissions struct {
	adapter port.Permissions
	events  port.Events
}

func NewPermissions(adapter port.Permissions, events port.Events) *Permissions {
	return &Permissions{adapter: adapter, events: events}
}

var allPermissions = []port.Permission{
	port.PermissionMicrophone,
	port.PermissionScreenCapture,
	port.PermissionAccessibility,
}

func (p *Permissions) CheckAll() error {
	for _, perm := range allPermissions {
		granted := false
		if g, err := p.adapter.Status(perm); err == nil {
			granted = g
		}
		if err := p.events.Emit("app:permission", map[string]any{
			"permission": string(perm),
			"granted":    granted,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (p *Permissions) Status(perm port.Permission) (bool, error) { return p.adapter.Status(perm) }
func (p *Permissions) Request(perm port.Permission) error        { return p.adapter.Request(perm) }
func (p *Permissions) OpenSettings(perm port.Permission) error   { return p.adapter.OpenSettings(perm) }
