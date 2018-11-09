package error

import "net/http"

type UnauthorizedHttpError struct {
	basicHttpError
}

//--------------------

func NewUnauthorizedHttpError() *UnauthorizedHttpError {
	e := &UnauthorizedHttpError{
		basicHttpError{
			status:  http.StatusUnauthorized,
			message: http.StatusText(http.StatusUnauthorized),
		},
	}

	return e
}
