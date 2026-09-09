//go:build darwin

package system

import (
	"testing"

	"interagent/internal/port"
)

func TestSettingsURL_Microphone(t *testing.T) {
	url, ok := settingsURL(port.PermissionMicrophone)
	if !ok {
		t.Fatal("settingsURL(microphone) = not ok")
	}
	if want := "x-apple.systempreferences:com.apple.preference.security?Privacy_Microphone"; url != want {
		t.Errorf("settingsURL(microphone) = %q, want %q", url, want)
	}
}

func TestSettingsURL_ScreenRecording(t *testing.T) {
	url, ok := settingsURL(port.PermissionScreenCapture)
	if !ok {
		t.Fatal("settingsURL(screen-recording) = not ok")
	}
	if want := "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"; url != want {
		t.Errorf("settingsURL(screen-recording) = %q, want %q", url, want)
	}
}

func TestSettingsURL_Accessibility(t *testing.T) {
	url, ok := settingsURL(port.PermissionAccessibility)
	if !ok {
		t.Fatal("settingsURL(accessibility) = not ok")
	}
	if want := "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"; url != want {
		t.Errorf("settingsURL(accessibility) = %q, want %q", url, want)
	}
}

func TestSettingsURL_UnknownPermission_NotOk(t *testing.T) {
	if _, ok := settingsURL(port.Permission("unknown")); ok {
		t.Error("settingsURL(unknown) = ok, want not ok")
	}
}
