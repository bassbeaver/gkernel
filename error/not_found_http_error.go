package error

import "net/http"

type NotFoundHttpError struct {
	basicHttpError
}

//--------------------

func NewNotFoundHttpError() *NotFoundHttpError {
	e := &NotFoundHttpError{
		basicHttpError{
			status:  http.StatusNotFound,
			message: http.StatusText(http.StatusNotFound),
		},
	}

	return e
}
