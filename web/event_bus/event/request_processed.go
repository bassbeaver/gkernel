package event

import (
	commonEvents "github.com/bassbeaver/gkernel/event_bus/event"
	"github.com/bassbeaver/gkernel/web/response"
	"net/http"
)

type RequestProcessed struct {
	RequestHolder
	Respondent
	commonEvents.Propagator
}

func (e *RequestProcessed) SetResponse(responseObj response.Response) {
	e.responseObj = responseObj
}

//--------------------

func NewRequestProcessed(responseWriterObj http.ResponseWriter, requestObj *http.Request, responseObj response.Response) *RequestProcessed {
	return &RequestProcessed{
		RequestHolder: RequestHolder{
			responseWriterObj: responseWriterObj,
			requestObj:        requestObj,
		},
		Respondent: Respondent{
			ResponseHolder: ResponseHolder{
				responseObj: responseObj,
			},
		},
	}
}
