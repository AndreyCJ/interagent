//go:build linux

package system

import "interagent/internal/port"

// settingsURL reports no deep link for Linux: there is no permission-settings
// pane to jump to. Audio is pulse-transport-gated and OpenSettings launches
// pavucontrol; screen capture has no OS gate (ADR-013).
func settingsURL(p port.Permission) (string, bool) { return "", false }
