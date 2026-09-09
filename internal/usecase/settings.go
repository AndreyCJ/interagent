package usecase

import (
	"fmt"

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

var sttModels = map[string]bool{"tiny": true, "base": true, "small": true}
var sttLanguages = map[string]bool{"auto": true, "ru": true, "en": true}

func (s *Settings) Save(cfg port.AppSettings) error {
	if !sttModels[cfg.SttModel] {
		return fmt.Errorf("invalid stt model %q (want tiny, base or small)", cfg.SttModel)
	}
	if !sttLanguages[cfg.SttLanguage] {
		return fmt.Errorf("invalid stt language %q (want auto, ru or en)", cfg.SttLanguage)
	}
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
