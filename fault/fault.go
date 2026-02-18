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

type fault struct {
	code     faultCode
	message  string
	metadata any
	original error
}

func New(code faultCode, message string) fault {
	return fault{
		code:    code,
		message: message,
	}
}

func (f fault) WithMetadata(metadata any) fault {
	e := f
	e.metadata = metadata
	return e
}

func (f fault) WithOriginal(original error) fault {
	e := f
	e.original = original
	return e
}

func (f fault) Code() faultCode {
	return f.code
}

func (f fault) Message() string {
	return f.message
}

func (f fault) Metadata() any {
	return f.metadata
}

func (f fault) Original() error {
	return f.original
}

func (f fault) Error() string {
	if f.original != nil {
		return fmt.Sprintf("%s: %v", f.message, f.original)
	}
	return f.message
}
