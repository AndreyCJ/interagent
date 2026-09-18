//go:build linux

package system

import (
	"errors"
	"testing"

	"interagent/internal/port"
)

func TestPermissions_Status_Accessibility_NeverGranted(t *testing.T) {
	p := NewPermissions(nil)
	ok, err := p.Status(port.PermissionAccessibility)
	if err != nil || ok {
		t.Fatalf("Status(accessibility) = %v, %v; want false, nil", ok, err)
	}
}

func TestPermissions_Status_Microphone_ProbeBased(t *testing.T) {
	p := NewPermissions(func() error { return nil })
	if ok, err := p.Status(port.PermissionMicrophone); err != nil || !ok {
		t.Fatalf("Status(microphone) = %v, %v; want true, nil", ok, err)
	}
	p = NewPermissions(func() error { return errors.New("no pulse") })
	if ok, _ := p.Status(port.PermissionMicrophone); ok {
		t.Error("Status(microphone) = true, want false when probe fails")
	}
}

func TestPermissions_Status_ScreenRecording_NoOSGate(t *testing.T) {
	// A native Linux app needs no screen-recording consent; even with a dead
	// mic probe the status must not claim a missing grant (the capture layer
	// reports transport errors honestly when actually starting).
	p := NewPermissions(func() error { return errors.New("no pulse") })
	if ok, err := p.Status(port.PermissionScreenCapture); err != nil || !ok {
		t.Fatalf("Status(screen-recording) = %v, %v; want true, nil (no gate)", ok, err)
	}
}

func TestPermissions_Request_ScreenRecording_Noop(t *testing.T) {
	p := NewPermissions(nil)
	if err := p.Request(port.PermissionScreenCapture); err != nil {
		t.Fatalf("Request(screen-recording) = %v, want nil (no consent gate)", err)
	}
}

func TestPermissions_Request_Microphone_ProbeChecked(t *testing.T) {
	p := NewPermissions(func() error { return errors.New("no pulse") })
	if err := p.Request(port.PermissionMicrophone); err == nil {
		t.Fatal("Request(microphone) = nil, want error when probe fails")
	}
}

func TestPermissions_Request_Accessibility_ErrorsHonestly(t *testing.T) {
	p := NewPermissions(nil)
	if err := p.Request(port.PermissionAccessibility); err == nil {
		t.Fatal("Request(accessibility) = nil, want error (not supported)")
	}
}

func TestPermissions_OpenSettings_Accessibility_ErrorsHonestly(t *testing.T) {
	p := NewPermissions(nil)
	if err := p.OpenSettings(port.PermissionAccessibility); err == nil {
		t.Fatal("OpenSettings(accessibility) = nil, want error")
	}
}
