package event_bus

import (
	"errors"
	"github.com/bassbeaver/gkernel/event_bus/event"
	"sync"
)

const (
	KernelEventApplicationLaunched    = "kernelEvent.ApplicationLaunched"
	KernelEventApplicationTermination = "kernelEvent.ApplicationTermination"
	KernelEventRequestReceived        = "kernelEvent.RequestReceived"
	KernelEventRequestProcessed       = "kernelEvent.RequestProcessed"
	KernelEventResponseBeforeSend     = "kernelEvent.ResponseBeforeSend"
	KernelEventRequestTermination     = "kernelEvent.RequestTermination"
	KernelEventRuntimeError           = "kernelEvent.RuntimeError"
)

type EventsRegistry struct {
	registry      map[string]event.Event
	registryMutex sync.RWMutex
}

func (r *EventsRegistry) Register(name string, eventObj event.Event) {
	r.registryMutex.Lock()
	defer r.registryMutex.Unlock()

	r.registry[name] = eventObj
}

func (r *EventsRegistry) GetEventByName(name string) (event.Event, error) {
	r.registryMutex.RLock()
	defer r.registryMutex.RUnlock()

	if eventObj, eventMapped := r.registry[name]; eventMapped {
		return eventObj, nil
	}

	return nil, errors.New("unknown event " + name)
}

//--------------------

func NewDefaultRegistry() *EventsRegistry {
	r := &EventsRegistry{
		registry: make(map[string]event.Event),
	}

	r.Register(KernelEventApplicationLaunched, (*event.ApplicationLaunched)(nil))
	r.Register(KernelEventApplicationTermination, (*event.ApplicationTermination)(nil))
	r.Register(KernelEventRequestReceived, (*event.RequestReceived)(nil))
	r.Register(KernelEventRequestProcessed, (*event.RequestProcessed)(nil))
	r.Register(KernelEventResponseBeforeSend, (*event.ResponseBeforeSend)(nil))
	r.Register(KernelEventRequestTermination, (*event.RequestTermination)(nil))
	r.Register(KernelEventRuntimeError, (*event.RuntimeError)(nil))

	return r
}
