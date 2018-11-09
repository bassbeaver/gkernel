package response

import (
	"bytes"
	"net/http"
)

type BytesResponse struct {
	basicResponse
	Body *bytes.Buffer
}

func (r *BytesResponse) ClearBody() {
	r.Body = new(bytes.Buffer)
}

func (r *BytesResponse) GetBodyBytes() *bytes.Buffer {
	result := new(bytes.Buffer)
	*result = *r.Body

	return result
}

//--------------------

func NewBytesResponse() *BytesResponse {
	return &BytesResponse{
		basicResponse: basicResponse{
			httpStatus: http.StatusOK,
			headers:    make(http.Header),
		},
		Body: new(bytes.Buffer),
	}
}

//--------------------

type BytesResponseWriter struct {
	*BytesResponse
}

func (r *BytesResponseWriter) Header() http.Header {
	return r.GetHeaders()
}

func (r *BytesResponseWriter) Write(content []byte) (int, error) {
	return r.Body.Write(content)
}

func (r *BytesResponseWriter) WriteHeader(statusCode int) {
	r.SetHttpStatus(statusCode)
}

//--------------------

func NewBytesResponseWriter() *BytesResponseWriter {
	return &BytesResponseWriter{
		BytesResponse: NewBytesResponse(),
	}
}
