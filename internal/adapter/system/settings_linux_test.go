//go:build linux

package system

import (
	"testing"

	"interagent/internal/port"
)

func TestSettingsURL_Linux_HasNoDeepLinks(t *testing.T) {
	for _, perm := range []port.Permission{
		port.PermissionMicrophone,
		port.PermissionScreenCapture,
		port.PermissionAccessibility,
	} {
		if url, ok := settingsURL(perm); ok {
			t.Errorf("settingsURL(%s) = %q, ok — linux has no deep-link pane", perm, url)
		}
	}
}
