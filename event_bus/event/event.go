package event

import (
	"context"
	"github.com/bassbeaver/gioc"
	"github.com/bassbeaver/gkernel/response"
	"net/http"
)

type Event interface {
	StopPropagation()
	IsPropagationStopped() bool
}

//--------------------

type Propagator struct {
	propagationStopped bool
}

func (e *Propagator) StopPropagation() {
	e.propagationStopped = true
}

func (e *Propagator) IsPropagationStopped() bool {
	return e.propagationStopped
}

//--------------------

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

// Provides access to response but does not allow response changes
type ResponseHolder struct {
	responseObj response.Response
}

func (r *ResponseHolder) GetResponse() response.Response {
	return r.responseObj
}

//--------------------

// Provides access to response and allows response setting/replacement
type Respondent struct {
	ResponseHolder
}

func (r *Respondent) SetResponse(responseObj response.Response) {
	r.ResponseHolder.responseObj = responseObj
}

//--------------------

// Provides access to response, allows response setting/replacement, interrupts event propagation after response is set
type StoppingRespondent struct {
	Respondent
	Propagator
}

func (sr *StoppingRespondent) SetResponse(responseObj response.Response) {
	sr.responseObj = responseObj
	sr.StopPropagation()
}

//--------------------

type containerAccessor interface {
	GetContainer() *gioc.Container
}
