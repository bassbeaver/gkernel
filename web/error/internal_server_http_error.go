package error

import "net/http"

type InternalServerHttpError struct {
	basicHttpError
}

//--------------------

func NewInternalServerHttpError() *InternalServerHttpError {
	return &InternalServerHttpError{
		basicHttpError{
			status:  http.StatusInternalServerError,
			message: http.StatusText(http.StatusInternalServerError),
		},
	}
}
