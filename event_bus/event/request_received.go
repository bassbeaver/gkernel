package event

import "net/http"

type RequestReceived struct {
	RequestHolder
	StoppingRespondent
}

//--------------------

func NewRequestReceived(requestObj *http.Request) *RequestReceived {
	return &RequestReceived{
		RequestHolder: RequestHolder{
			requestObj: requestObj,
		},
	}
}
