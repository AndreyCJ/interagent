package usecase

import (
	"interagent/internal/port"
)

type Settings struct {
	store port.SettingsStorage
}

func NewSettings(store port.SettingsStorage) *Settings {
	return &Settings{store: store}
}

func (s *Settings) Get() (port.AppSettings, error) {
	return s.store.GetSettings()
}

func (s *Settings) Save(cfg port.AppSettings) error {
	return s.store.SaveSettings(cfg)
}

func (s *Settings) UpdateShortcut(id string, keys []string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	for i := range settings.Shortcuts {
		if settings.Shortcuts[i].ID == id {
			settings.Shortcuts[i].Keys = keys
			return s.store.SaveSettings(settings)
		}
	}
	return nil
}
