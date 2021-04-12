package error

type RuntimeError struct {
	basicCliError
}

//--------------------

func NewRuntimeError(message string) *RuntimeError {
	return &RuntimeError{
		basicCliError{
			status:  3,
			message: message,
		},
	}
}
