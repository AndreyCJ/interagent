package usecase

import (
	"errors"
	"testing"

	"interagent/internal/port"
)

type mockPermissions struct {
	status    map[port.Permission]bool
	statusErr error
	requested []port.Permission
	opened    []port.Permission
}

func newMockPermissions() *mockPermissions {
	return &mockPermissions{status: make(map[port.Permission]bool)}
}

func (m *mockPermissions) Status(p port.Permission) (bool, error) {
	if m.statusErr != nil {
		return false, m.statusErr
	}
	return m.status[p], nil
}

func (m *mockPermissions) Request(p port.Permission) error {
	m.requested = append(m.requested, p)
	return nil
}

func (m *mockPermissions) OpenSettings(p port.Permission) error {
	m.opened = append(m.opened, p)
	return nil
}

func permissionPayload(payload any) (string, bool) {
	m := payload.(map[string]any)
	return m["permission"].(string), m["granted"].(bool)
}

func TestPermissions_CheckAll_EmitsStatusForEachPermission(t *testing.T) {
	adapter := newMockPermissions()
	adapter.status[port.PermissionAccessibility] = true
	events := newMockEvents()
	u := NewPermissions(adapter, events)

	if err := u.CheckAll(); err != nil {
		t.Fatalf("CheckAll() returned error: %v", err)
	}
	if events.count("app:permission") != 3 {
		t.Fatalf("expected 3 app:permission events, got %d", events.count("app:permission"))
	}

	want := []port.Permission{
		port.PermissionMicrophone,
		port.PermissionScreenCapture,
		port.PermissionAccessibility,
	}
	for i, perm := range want {
		name, granted := permissionPayload(events.payload("app:permission", i))
		if name != string(perm) {
			t.Errorf("event %d permission = %q, want %q", i, name, perm)
		}
		if granted != adapter.status[perm] {
			t.Errorf("event %d granted = %v, want %v", i, granted, adapter.status[perm])
		}
	}
}

func TestPermissions_CheckAll_StatusError_EmitsDenied_NoCrash(t *testing.T) {
	adapter := newMockPermissions()
	adapter.status[port.PermissionAccessibility] = true
	adapter.statusErr = errors.New("query failed")
	events := newMockEvents()
	u := NewPermissions(adapter, events)

	if err := u.CheckAll(); err != nil {
		t.Fatalf("CheckAll() should not fail on status error: %v", err)
	}
	if events.count("app:permission") != 3 {
		t.Fatalf("expected 3 app:permission events, got %d", events.count("app:permission"))
	}
	for i := 0; i < events.count("app:permission"); i++ {
		if _, granted := permissionPayload(events.payload("app:permission", i)); granted {
			t.Errorf("event %d should be denied when status errors", i)
		}
	}
}

func TestPermissions_Status_Delegates(t *testing.T) {
	adapter := newMockPermissions()
	adapter.status[port.PermissionMicrophone] = false
	u := NewPermissions(adapter, newMockEvents())

	got, err := u.Status(port.PermissionMicrophone)
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}
	if got {
		t.Error("Status(microphone) = true, want false")
	}
}

func TestPermissions_Request_Delegates(t *testing.T) {
	adapter := newMockPermissions()
	u := NewPermissions(adapter, newMockEvents())

	if err := u.Request(port.PermissionAccessibility); err != nil {
		t.Fatalf("Request() returned error: %v", err)
	}
	if len(adapter.requested) != 1 || adapter.requested[0] != port.PermissionAccessibility {
		t.Errorf("requested = %v, want [accessibility]", adapter.requested)
	}
}

func TestPermissions_OpenSettings_Delegates(t *testing.T) {
	adapter := newMockPermissions()
	u := NewPermissions(adapter, newMockEvents())

	if err := u.OpenSettings(port.PermissionScreenCapture); err != nil {
		t.Fatalf("OpenSettings() returned error: %v", err)
	}
	if len(adapter.opened) != 1 || adapter.opened[0] != port.PermissionScreenCapture {
		t.Errorf("opened = %v, want [screen-recording]", adapter.opened)
	}
}
