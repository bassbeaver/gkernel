package event

import (
	commonEvents "github.com/bassbeaver/gkernel/event_bus/event"
	"github.com/bassbeaver/gkernel/web/response"
	"net/http"
)

type ResponseBeforeSend struct {
	RequestHolder
	ResponseHolder
	commonEvents.Propagator
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
