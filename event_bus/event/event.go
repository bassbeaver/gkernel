package event

import (
	"github.com/bassbeaver/gioc"
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

type containerAccessor interface {
	GetContainer() *gioc.Container
}
