package objsender

type SessionCommon interface {
	Send(obj Model) error
	Done() error
	SetTotal(v int) error
}
