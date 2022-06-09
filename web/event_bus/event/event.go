package event

import (
	"context"
	commonEvent "github.com/bassbeaver/gkernel/event_bus/event"
	"github.com/bassbeaver/gkernel/web/response"
	"net/http"
)

type RequestHolder struct {
	requestObj        *http.Request
	responseWriterObj http.ResponseWriter
}

func (h *RequestHolder) GetRequest() *http.Request {
	return h.requestObj
}

func (h *RequestHolder) RequestContextAppend(key, val interface{}) {
	newContext := context.WithValue(h.requestObj.Context(), key, val)
	*h.requestObj = *h.requestObj.WithContext(newContext)
}

func (h *RequestHolder) GetResponseWriter() http.ResponseWriter {
	return h.responseWriterObj
}

//--------------------

// ResponseHolder Provides access to response but does not allow response changes
type ResponseHolder struct {
	responseObj response.Response
}

func (r *ResponseHolder) GetResponse() response.Response {
	return r.responseObj
}

//--------------------

// Respondent Provides access to response and allows response setting/replacement
type Respondent struct {
	ResponseHolder
}

func (r *Respondent) SetResponse(responseObj response.Response) {
	r.ResponseHolder.responseObj = responseObj
}

//--------------------

// StoppingRespondent Provides access to response, allows response setting/replacement, interrupts event propagation after response is set
type StoppingRespondent struct {
	Respondent
	commonEvent.Propagator
}

func (sr *StoppingRespondent) SetResponse(responseObj response.Response) {
	sr.responseObj = responseObj
	sr.StopPropagation()
}
