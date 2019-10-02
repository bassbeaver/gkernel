package event

import (
	"github.com/bassbeaver/gkernel/response"
	"net/http"
)

type ResponseBeforeSend struct {
	RequestHolder
	ResponseHolder
	Propagator
}

//--------------------

func NewResponseBeforeSend(responseWriterObj http.ResponseWriter, requestObj *http.Request, responseObj response.Response) *ResponseBeforeSend {
	return &ResponseBeforeSend{
		RequestHolder: RequestHolder{
			responseWriterObj: responseWriterObj,
			requestObj:        requestObj,
		},
		ResponseHolder: ResponseHolder{
			responseObj: responseObj,
		},
	}
}
