package cli

import "fmt"

const (
	ExitSuccess         = 0
	ExitGeneralError    = 1
	ExitValidationError = 2
	ExitMigrationError  = 3
	ExitRollbackBlocked = 4
)

// ExitError wraps an error with a specific exit code.
type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string {
	return e.Message
}

func NewExitError(code int, format string, args ...any) *ExitError {
	return &ExitError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}
