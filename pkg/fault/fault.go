package fault

import "fmt"

type faultCode string

const (
	UnknownCode          faultCode = "unknown"
	NotFoundCode         faultCode = "not_found"
	BadInputCode         faultCode = "bad_input"
	PermissionDeniedCode faultCode = "permission_denied"
)

type FieldErrorsMetadata map[string][]string

type Fault struct {
	code     faultCode
	message  string
	metadata any
	original error
}

func New(code faultCode, message string) Fault {
	return Fault{
		code:    code,
		message: message,
	}
}

func (f Fault) WithMetadata(metadata any) Fault {
	e := f
	e.metadata = metadata
	return e
}

func (f Fault) WithOriginal(original error) Fault {
	e := f
	e.original = original
	return e
}

func (f Fault) Code() faultCode {
	return f.code
}

func (f Fault) Message() string {
	return f.message
}

func (f Fault) Metadata() any {
	return f.metadata
}

func (f Fault) Original() error {
	return f.original
}

func (f Fault) Error() string {
	if f.original != nil {
		return fmt.Sprintf("%s: %v", f.message, f.original)
	}
	return f.message
}
