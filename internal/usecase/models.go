package usecase

import (
	"interagent/internal/port"
)

type Models struct {
	events port.Events
	store  port.ModelStore
}

func NewModels(events port.Events, store port.ModelStore) *Models {
	return &Models{events: events, store: store}
}

func (m *Models) Status(model string) (port.STTModelStatus, error) {
	installed, path, err := m.store.Status(model)
	if err != nil {
		return port.STTModelStatus{}, err
	}
	return port.STTModelStatus{Installed: installed, Path: path}, nil
}

// Download fetches the model asynchronously. Progress events are throttled to
// at most one per 5% of the total; the final event is always emitted.
func (m *Models) Download(model string) error {
	go func() {
		if err := m.ensure(model); err != nil {
			_ = m.events.Emit("app:error", map[string]string{"stage": "models", "error": err.Error()})
		}
	}()
	return nil
}

// Ensure installs the model synchronously, streaming the same progress/done
// events as Download. Used by StartListening so an absent model is fetched
// before capture begins. Errors are returned to the caller — unlike the async
// Download path, no app:error is emitted here (the caller surfaces it).
func (m *Models) Ensure(model string) error {
	return m.ensure(model)
}

func (m *Models) ensure(model string) error {
	installed, _, err := m.store.Status(model)
	if err != nil {
		return err
	}
	if installed {
		_ = m.events.Emit("model:downloaded", map[string]string{"model": model})
		return nil
	}
	var last int64
	if err := m.store.Download(model, func(received, total int64) {
		if total > 0 && (received-last >= total/20 || received >= total) {
			last = received
			_ = m.events.Emit("model:download-progress", map[string]any{
				"model": model, "received": received, "total": total,
			})
		}
	}); err != nil {
		return err
	}
	_ = m.events.Emit("model:downloaded", map[string]string{"model": model})
	return nil
}
