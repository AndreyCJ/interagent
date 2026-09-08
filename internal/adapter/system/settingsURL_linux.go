//go:build linux

package system

import "interagent/internal/port"

// settingsURL reports no deep link for Linux: there is no permission-settings
// pane to jump to. Microphone audio is pulse-gated (pavucontrol is launched by
// OpenSettings), screen capture by the portal picker itself.
func settingsURL(p port.Permission) (string, bool) { return "", false }
