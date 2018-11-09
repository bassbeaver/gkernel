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

func NewResponseBeforeSend(requestObj *http.Request, responseObj response.Response) *ResponseBeforeSend {
	return &ResponseBeforeSend{
		RequestHolder: RequestHolder{
			requestObj: requestObj,
		},
		ResponseHolder: ResponseHolder{
			responseObj: responseObj,
		},
	}
}
