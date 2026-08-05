package main

import "interagent/internal/port"

type ModelsBind struct {
	app interface {
		DownloadSTTModel() error
		GetSTTModelStatus() (port.STTModelStatus, error)
	}
}

func NewModelsBind(app interface {
	DownloadSTTModel() error
	GetSTTModelStatus() (port.STTModelStatus, error)
}) *ModelsBind {
	return &ModelsBind{app: app}
}

func (b *ModelsBind) DownloadSTTModel() error { return b.app.DownloadSTTModel() }

func (b *ModelsBind) GetSTTModelStatus() (port.STTModelStatus, error) {
	return b.app.GetSTTModelStatus()
}
