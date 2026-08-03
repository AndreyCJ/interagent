package port

type Events interface {
	Emit(name string, payload any) error
}
