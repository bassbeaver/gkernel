package event

import (
	kernelError "github.com/bassbeaver/gkernel/error"
	"net/http"
)

type RuntimeError struct {
	errorObj *kernelError.RuntimeError
	RequestHolder
	StoppingRespondent
}

func (e *RuntimeError) GetError() *kernelError.RuntimeError {
	return e.errorObj
}

//--------------------

func NewRuntimeError(requestObj *http.Request, errorObj *kernelError.RuntimeError) *RuntimeError {
	return &RuntimeError{
		errorObj: errorObj,
		RequestHolder: RequestHolder{
			requestObj: requestObj,
		},
	}
}
