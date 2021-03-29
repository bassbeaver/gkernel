package event

import "net/http"

type RequestReceived struct {
	RequestHolder
	StoppingRespondent
}

//--------------------

func NewRequestReceived(responseWriterObj http.ResponseWriter, requestObj *http.Request) *RequestReceived {
	return &RequestReceived{
		RequestHolder: RequestHolder{
			responseWriterObj: responseWriterObj,
			requestObj:        requestObj,
		},
	}
}
