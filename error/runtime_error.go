package error

type RuntimeError struct {
	Error interface{}
	Trace []byte
}

//--------------------

func NewRuntimeError(errorObj interface{}, trace []byte) *RuntimeError {
	if nil == trace {
		trace = make([]byte, 0)
	}

	return &RuntimeError{
		Error: errorObj,
		Trace: trace,
	}
}
