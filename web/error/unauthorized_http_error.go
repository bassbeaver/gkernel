package error

import "net/http"

type UnauthorizedHttpError struct {
	basicHttpError
}

//--------------------

func NewUnauthorizedHttpError() *UnauthorizedHttpError {
	return &UnauthorizedHttpError{
		basicHttpError{
			status:  http.StatusUnauthorized,
			message: http.StatusText(http.StatusUnauthorized),
		},
	}
}
