package line

type Service interface {
	Notify(meessage string) error
}
