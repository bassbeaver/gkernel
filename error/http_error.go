package error

type HttpError interface {
	Status() int
	Message() string
}

//--------------------

type basicHttpError struct {
	status  int
	message string
}

func (e *basicHttpError) Status() int {
	return e.status
}

func (e *basicHttpError) Message() string {
	return e.message
}
