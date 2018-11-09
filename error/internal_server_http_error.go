package error

import "net/http"

type InternalServerHttpError struct {
	basicHttpError
}

//--------------------

func NewInternalServerHttpError() *InternalServerHttpError {
	e := &InternalServerHttpError{
		basicHttpError{
			status:  http.StatusInternalServerError,
			message: http.StatusText(http.StatusInternalServerError),
		},
	}

	return e
}
