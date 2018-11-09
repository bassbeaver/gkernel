package event

import (
	"github.com/bassbeaver/gkernel/response"
	"net/http"
)

type RequestProcessed struct {
	RequestHolder
	Respondent
	Propagator
}

func (e *RequestProcessed) SetResponse(responseObj response.Response) {
	e.responseObj = responseObj
}

//--------------------

func NewRequestProcessed(requestObj *http.Request, responseObj response.Response) *RequestProcessed {
	return &RequestProcessed{
		RequestHolder: RequestHolder{
			requestObj: requestObj,
		},
		Respondent: Respondent{
			ResponseHolder: ResponseHolder{
				responseObj: responseObj,
			},
		},
	}
}
