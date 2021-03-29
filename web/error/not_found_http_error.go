package error

import "net/http"

type NotFoundHttpError struct {
	basicHttpError
}

//--------------------

func NewNotFoundHttpError() *NotFoundHttpError {
	return &NotFoundHttpError{
		basicHttpError{
			status:  http.StatusNotFound,
			message: http.StatusText(http.StatusNotFound),
		},
	}
}
