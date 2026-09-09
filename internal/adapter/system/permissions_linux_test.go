//go:build linux

package system

import (
	"context"
	"errors"
	"testing"

	"interagent/internal/port"
)

type fakeSC struct {
	available bool
	granted   bool
	grantErr  error
	requested bool
}

func (f *fakeSC) Available(ctx context.Context) bool { return f.available }
func (f *fakeSC) Granted() bool                      { return f.granted }
func (f *fakeSC) Grant(ctx context.Context) error {
	f.requested = true
	return f.grantErr
}

func TestPermissions_Status_Accessibility_NeverGranted(t *testing.T) {
	p := NewPermissions(&fakeSC{granted: true}, nil)
	ok, err := p.Status(port.PermissionAccessibility)
	if err != nil || ok {
		t.Fatalf("Status(accessibility) = %v, %v; want false, nil", ok, err)
	}
}

func TestPermissions_Status_Microphone_ProbeBased(t *testing.T) {
	p := NewPermissions(nil, func() error { return nil })
	if ok, err := p.Status(port.PermissionMicrophone); err != nil || !ok {
		t.Fatalf("Status(microphone) = %v, %v; want true, nil", ok, err)
	}
	p = NewPermissions(nil, func() error { return errors.New("no pulse") })
	if ok, _ := p.Status(port.PermissionMicrophone); ok {
		t.Error("Status(microphone) = true, want false when probe fails")
	}
}

func TestPermissions_Status_ScreenCapture_ReflectsGrant(t *testing.T) {
	p := NewPermissions(&fakeSC{granted: true}, nil)
	if ok, err := p.Status(port.PermissionScreenCapture); err != nil || !ok {
		t.Fatalf("Status(screen-recording) = %v, %v; want true, nil", ok, err)
	}
	p = NewPermissions(&fakeSC{granted: false}, nil)
	if ok, err := p.Status(port.PermissionScreenCapture); err != nil || ok {
		t.Fatalf("Status(screen-recording) = %v, %v; want false, nil", ok, err)
	}
}

func TestPermissions_Request_ScreenCapture_RunsPickerFlow(t *testing.T) {
	f := &fakeSC{granted: true, available: true}
	p := NewPermissions(f, nil)
	if err := p.Request(port.PermissionScreenCapture); err != nil {
		t.Fatalf("Request(screen-recording): %v", err)
	}
	if !f.requested {
		t.Error("portal Grant not invoked")
	}
}

func TestPermissions_Request_Accessibility_ErrorsHonestly(t *testing.T) {
	p := NewPermissions(nil, nil)
	if err := p.Request(port.PermissionAccessibility); err == nil {
		t.Fatal("Request(accessibility) = nil, want error (not supported)")
	}
}

func TestPermissions_OpenSettings_Accessibility_ErrorsHonestly(t *testing.T) {
	p := NewPermissions(nil, nil)
	if err := p.OpenSettings(port.PermissionAccessibility); err == nil {
		t.Fatal("OpenSettings(accessibility) = nil, want error")
	}
}
