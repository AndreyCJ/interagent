//go:build !darwin && !linux

package system

import "interagent/internal/port"

// settingsURL reports no deep link on platforms without a permission
// settings pane.
func settingsURL(p port.Permission) (string, bool) { return "", false }
