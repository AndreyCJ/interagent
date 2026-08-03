package main

import "interagent/internal/port"

type PermissionsBind struct {
	usecase interface {
		Status(p port.Permission) (bool, error)
		Request(p port.Permission) error
		OpenSettings(p port.Permission) error
	}
}

func NewPermissionsBind(u interface {
	Status(p port.Permission) (bool, error)
	Request(p port.Permission) error
	OpenSettings(p port.Permission) error
}) *PermissionsBind {
	return &PermissionsBind{usecase: u}
}

func (b *PermissionsBind) GetPermissionStatus(p port.Permission) (bool, error) {
	return b.usecase.Status(p)
}

func (b *PermissionsBind) RequestPermission(p port.Permission) (bool, error) {
	if err := b.usecase.Request(p); err != nil {
		return false, err
	}
	return b.usecase.Status(p)
}

func (b *PermissionsBind) OpenPermissionSettings(p port.Permission) error {
	return b.usecase.OpenSettings(p)
}
