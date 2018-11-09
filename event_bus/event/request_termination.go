package event

import (
	"github.com/bassbeaver/gkernel/response"
	"net/http"
)

type RequestTermination struct {
	RequestHolder
	ResponseHolder
	Propagator
}

//--------------------

func NewRequestTermination(requestObj *http.Request, responseObj response.Response) *RequestTermination {
	return &RequestTermination{
		RequestHolder: RequestHolder{
			requestObj: requestObj,
		},
		ResponseHolder: ResponseHolder{
			responseObj: responseObj,
		},
	}
}
