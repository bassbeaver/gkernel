package error

type CliError interface {
	Status() int
	Message() string
}

//--------------------

type basicCliError struct {
	status  int
	message string
}

func (e *basicCliError) Status() int {
	return e.status
}

func (e *basicCliError) Message() string {
	return e.message
}
