package error

import "net/http"

type BadRequestHttpError struct {
	basicHttpError
}

//--------------------

func NewBadRequestHttpError() *BadRequestHttpError {
	e := &BadRequestHttpError{
		basicHttpError{
			status:  http.StatusBadRequest,
			message: http.StatusText(http.StatusBadRequest),
		},
	}

	return e
}
