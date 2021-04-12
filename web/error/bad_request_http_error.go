package error

import "net/http"

type BadRequestHttpError struct {
	basicHttpError
}

//--------------------

func NewBadRequestHttpError() *BadRequestHttpError {
	return &BadRequestHttpError{
		basicHttpError{
			status:  http.StatusBadRequest,
			message: http.StatusText(http.StatusBadRequest),
		},
	}
}
