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
	go m.downloadModel(model)
	return nil
}

func (m *Models) downloadModel(model string) {
	installed, _, err := m.store.Status(model)
	if err != nil {
		_ = m.events.Emit("app:error", map[string]string{"stage": "models", "error": err.Error()})
		return
	}
	if installed {
		_ = m.events.Emit("model:downloaded", map[string]string{"model": model})
		return
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
		_ = m.events.Emit("app:error", map[string]string{"stage": "models", "error": err.Error()})
		return
	}
	_ = m.events.Emit("model:downloaded", map[string]string{"model": model})
}
