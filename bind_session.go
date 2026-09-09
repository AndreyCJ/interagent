package main

import "interagent/internal/port"

type SessionBind struct {
	usecase interface {
		Create() (port.Session, error)
		Get(id string) (port.Session, error)
		GetCurrent() (port.Session, error)
		Clear(id string) error
	}
}

func NewSessionBind(u interface {
	Create() (port.Session, error)
	Get(id string) (port.Session, error)
	GetCurrent() (port.Session, error)
	Clear(id string) error
}) *SessionBind {
	return &SessionBind{usecase: u}
}

func (b *SessionBind) NewSession() (port.Session, error) {
	return b.usecase.Create()
}

func (b *SessionBind) GetSession() (port.Session, error) {
	return b.usecase.GetCurrent()
}

func (b *SessionBind) ClearSession() error {
	cur, err := b.usecase.GetCurrent()
	if err != nil {
		return err
	}
	return b.usecase.Clear(cur.ID)
}
