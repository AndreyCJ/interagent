//go:build windows

package window

import "interagent/internal/port"

// applyMode applies the click-through mode to the window. Windows
// WS_EX_TRANSPARENT is deferred; the mode state is tracked by Overlay.
func applyMode(port.OverlayMode) error { return nil }
