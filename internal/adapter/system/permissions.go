// Package system implements port.Permissions against the host platform's
// permission model. Each platform has its own adapter: darwin (TCC:
// ScreenCaptureKit preflight, AVFoundation authorization, Accessibility
// trust), linux (XDG portal ScreenCast + pulse reachability), and other
// (no-op stub).
package system
