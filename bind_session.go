package main

import "interagent/internal/port"

type SessionBind struct {
	usecase interface {
		Create() (port.Session, error)
		Get(id string) (port.Session, error)
		Clear(id string) error
	}
}

func NewSessionBind(u interface {
	Create() (port.Session, error)
	Get(id string) (port.Session, error)
	Clear(id string) error
}) *SessionBind {
	return &SessionBind{usecase: u}
}

func (b *SessionBind) NewSession() (port.Session, error) {
	return b.usecase.Create()
}

func (b *SessionBind) GetSession() (port.Session, error) {
	return b.usecase.Get("current")
}

func (b *SessionBind) ClearSession() error {
	return b.usecase.Clear("current")
}
