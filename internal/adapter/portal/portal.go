// Package portal wraps the freedesktop XDG Desktop Portal D-Bus interfaces
// used on Linux for system-audio capture and the matching permission grants.
// Pure Go (godbus) — no cgo.
package portal

import (
	"context"
	"errors"
)

type ScreenCast struct{}

func NewScreenCast() *ScreenCast { return &ScreenCast{} }

// Required stubs so *ScreenCast satisfies the permissions adapter's
// screenCast interface. Task 3 replaces this file with the real flow.
func (s *ScreenCast) Available(ctx context.Context) bool { return false }
func (s *ScreenCast) Grant(ctx context.Context) error {
	return errors.New("screen cast flow is implemented in a later task")
}
func (s *ScreenCast) Granted() bool { return false }
