package messaging

import "fmt"

// ErrorKind classifica o tipo de erro, o que decide o tratamento:
//
//	BusinessError  -> ACK + publica evento *.rejected. NAO faz retry.
//	TransientError -> Republica em blockchain.retry para tentar de novo.
//	PermanentError -> Manda direto pra DLQ.
//
// A distincao e crucial: retry de erro de negocio causa loop infinito;
// erro permanente em retry queima slots e atrasa a DLQ.
type ErrorKind int

const (
	KindBusiness ErrorKind = iota
	KindTransient
	KindPermanent
)

type Error struct {
	Kind   ErrorKind
	Reason string
	Cause  error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Reason, e.Cause)
	}
	return e.Reason
}

func (e *Error) Unwrap() error { return e.Cause }

func Business(reason string) *Error  { return &Error{Kind: KindBusiness, Reason: reason} }
func Transient(reason string, cause error) *Error {
	return &Error{Kind: KindTransient, Reason: reason, Cause: cause}
}
func Permanent(reason string, cause error) *Error {
	return &Error{Kind: KindPermanent, Reason: reason, Cause: cause}
}
