package main

import (
	"testing"

	"interagent/internal/port"
)

type fakePermissions struct {
	status     map[port.Permission]bool
	requested  []port.Permission
	grantOnReq map[port.Permission]bool
}

func (f *fakePermissions) Status(p port.Permission) (bool, error) {
	return f.status[p], nil
}

func (f *fakePermissions) Request(p port.Permission) error {
	f.requested = append(f.requested, p)
	if f.grantOnReq[p] {
		f.status[p] = true
	}
	return nil
}

func (f *fakePermissions) OpenSettings(p port.Permission) error { return nil }

func TestEnsureListeningPermissions(t *testing.T) {
	t.Run("proceeds without request when granted", func(t *testing.T) {
		p := &fakePermissions{status: map[port.Permission]bool{
			port.PermissionScreenCapture: true,
		}}
		if err := ensureListeningPermissions(p); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(p.requested) != 0 {
			t.Errorf("requested %v, want none", p.requested)
		}
	})

	t.Run("requests screen when not granted and proceeds on grant", func(t *testing.T) {
		p := &fakePermissions{
			status:     map[port.Permission]bool{},
			grantOnReq: map[port.Permission]bool{port.PermissionScreenCapture: true},
		}
		if err := ensureListeningPermissions(p); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(p.requested) != 1 || p.requested[0] != port.PermissionScreenCapture {
			t.Errorf("requested %v, want [screen-recording]", p.requested)
		}
	})

	t.Run("errors when the request does not grant", func(t *testing.T) {
		p := &fakePermissions{status: map[port.Permission]bool{}}
		err := ensureListeningPermissions(p)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "screen recording permission required for system sound" {
			t.Errorf("error = %q, want %q", err.Error(), "screen recording permission required for system sound")
		}
	})
}
