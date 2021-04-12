package event_bus

import commonEventBus "github.com/bassbeaver/gkernel/event_bus"
import commonEvent "github.com/bassbeaver/gkernel/event_bus/event"

func NewDefaultRegistry() *commonEventBus.EventsRegistry {
	r := commonEventBus.NewRegistry()

	r.Register(commonEventBus.KernelEventApplicationLaunched, (*commonEvent.ApplicationLaunched)(nil))
	r.Register(commonEventBus.KernelEventApplicationTermination, (*commonEvent.ApplicationTermination)(nil))

	return r
}
