package main

import "interagent/internal/port"

type HotkeysBind struct {
	usecase interface {
		RegisterShortcut(sc port.Shortcut) error
		Unregister(id string) error
	}
}

func NewHotkeysBind(u interface {
	RegisterShortcut(sc port.Shortcut) error
	Unregister(id string) error
}) *HotkeysBind {
	return &HotkeysBind{usecase: u}
}

func (b *HotkeysBind) RegisterHotkey(id string, keys []string) error {
	return b.usecase.RegisterShortcut(port.Shortcut{ID: id, Keys: keys, Enabled: true})
}

func (b *HotkeysBind) UnregisterHotkey(id string) error {
	return b.usecase.Unregister(id)
}
