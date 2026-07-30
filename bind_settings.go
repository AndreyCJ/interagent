package main

import "interagent/internal/port"

type SettingsBind struct {
	usecase interface {
		Get() (port.AppSettings, error)
		Save(cfg port.AppSettings) error
		UpdateShortcut(id string, keys []string) error
	}
}

func NewSettingsBind(u interface {
	Get() (port.AppSettings, error)
	Save(cfg port.AppSettings) error
	UpdateShortcut(id string, keys []string) error
}) *SettingsBind {
	return &SettingsBind{usecase: u}
}

func (b *SettingsBind) GetSettings() (port.AppSettings, error) {
	return b.usecase.Get()
}

func (b *SettingsBind) SaveSettings(s port.AppSettings) error {
	return b.usecase.Save(s)
}

func (b *SettingsBind) GetShortcuts() ([]port.Shortcut, error) {
	settings, err := b.usecase.Get()
	if err != nil {
		return nil, err
	}
	return settings.Shortcuts, nil
}

func (b *SettingsBind) UpdateShortcut(id string, keys []string) error {
	return b.usecase.UpdateShortcut(id, keys)
}
