package response

import (
	"bytes"
	"net/http"
)

type Response interface {
	GetHttpStatus() int
	GetHeaders() http.Header
	GetBodyBytes() *bytes.Buffer
}

//--------------------

type basicResponse struct {
	httpStatus int
	headers    http.Header
}

func (r *basicResponse) HeaderSet(key, value string) {
	r.headers.Set(key, value)
}

func (r *basicResponse) GetHeaders() http.Header {
	return r.headers
}

func (r *basicResponse) GetHttpStatus() int {
	return r.httpStatus
}

func (r *basicResponse) SetHttpStatus(status int) {
	r.httpStatus = status
}
