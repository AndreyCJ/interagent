//go:build linux

package window

import "interagent/internal/port"

// applyMode applies the click-through mode to the window. Linux pointer
// passthrough (X11/XShape) is deferred; mode state is tracked by Overlay.
func applyMode(port.OverlayMode) error {
	return nil
}
