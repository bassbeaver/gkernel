package event_bus

import commonEventBus "github.com/bassbeaver/gkernel/event_bus"
import commonEvent "github.com/bassbeaver/gkernel/event_bus/event"
import webEvent "github.com/bassbeaver/gkernel/web/event_bus/event"

const (
	KernelEventRequestReceived    = "kernelEvent.RequestReceived"
	KernelEventRequestProcessed   = "kernelEvent.RequestProcessed"
	KernelEventResponseBeforeSend = "kernelEvent.ResponseBeforeSend"
	KernelEventRequestTermination = "kernelEvent.RequestTermination"
	KernelEventRuntimeError       = "kernelEvent.RuntimeError"
)

func NewDefaultRegistry() *commonEventBus.EventsRegistry {
	r := commonEventBus.NewRegistry()

	r.Register(commonEventBus.KernelEventApplicationLaunched, (*commonEvent.ApplicationLaunched)(nil))
	r.Register(commonEventBus.KernelEventApplicationTermination, (*commonEvent.ApplicationTermination)(nil))
	r.Register(KernelEventRequestReceived, (*webEvent.RequestReceived)(nil))
	r.Register(KernelEventRequestProcessed, (*webEvent.RequestProcessed)(nil))
	r.Register(KernelEventResponseBeforeSend, (*webEvent.ResponseBeforeSend)(nil))
	r.Register(KernelEventRequestTermination, (*webEvent.RequestTermination)(nil))
	r.Register(KernelEventRuntimeError, (*webEvent.RuntimeError)(nil))

	return r
}
