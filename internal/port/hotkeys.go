package port

type Hotkeys interface {
	Register(id string, keys []string) error
	Unregister(id string) error
}
