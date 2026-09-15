// Package system implements port.Permissions against the host platform's
// permission model. Each platform has its own adapter: darwin (TCC:
// ScreenCaptureKit preflight, AVFoundation authorization, Accessibility
// trust), linux (no OS gates — transcript- and audio-transport probes, ADR-013),
// and other (no-op stub).
package system
