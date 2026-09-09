//go:build darwin

package system

import "interagent/internal/port"

// settingsURL returns the System Settings deep link for a permission and
// whether the permission maps to a macOS TCC pane.
func settingsURL(p port.Permission) (string, bool) {
	switch p {
	case port.PermissionMicrophone:
		return "x-apple.systempreferences:com.apple.preference.security?Privacy_Microphone", true
	case port.PermissionScreenCapture:
		return "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture", true
	case port.PermissionAccessibility:
		return "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility", true
	default:
		return "", false
	}
}
