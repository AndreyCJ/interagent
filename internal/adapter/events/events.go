package events

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Events struct {
	mu  sync.RWMutex
	ctx context.Context
}

func New(ctx context.Context) *Events {
	return &Events{ctx: ctx}
}

func (e *Events) SetContext(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ctx = ctx
}

func (e *Events) Emit(name string, payload any) error {
	e.mu.RLock()
	ctx := e.ctx
	e.mu.RUnlock()
	if ctx == nil {
		return nil
	}
	runtime.EventsEmit(ctx, name, payload)
	return nil
}
