package event

import (
	commonEvents "github.com/bassbeaver/gkernel/event_bus/event"
	"github.com/bassbeaver/gkernel/web/response"
	"net/http"
)

type RequestTermination struct {
	RequestHolder
	ResponseHolder
	commonEvents.Propagator
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
