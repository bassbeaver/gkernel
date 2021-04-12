package response

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type JsonResponse struct {
	basicResponse
	Body interface{}
}

func (r *JsonResponse) GetBodyBytes() *bytes.Buffer {
	resultBytes, err := json.Marshal(r.Body)
	if err != nil {
		panic(err)
	}

	result := bytes.NewBuffer(resultBytes)

	return result
}

//--------------------

func NewJsonResponse() *JsonResponse {
	r := &JsonResponse{
		basicResponse: basicResponse{
			httpStatus: http.StatusOK,
			headers:    make(http.Header),
		},
		Body: nil,
	}
	r.HeaderSet("Content-Type", "application/json")

	return r
}

func NewJsonApiResponse() *JsonResponse {
	r := &JsonResponse{
		basicResponse: basicResponse{
			httpStatus: http.StatusOK,
			headers:    make(http.Header),
		},
		Body: nil,
	}
	r.HeaderSet("Content-Type", "application/vnd.api+json")

	return r
}
