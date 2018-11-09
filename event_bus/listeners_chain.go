package event_bus

import "reflect"

type listenersChainElement struct {
	priority             int
	listenerReflectValue reflect.Value
}

//--------------------

type listenersChain []*listenersChainElement

func (c listenersChain) Len() int {
	return len(c)
}

func (c listenersChain) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c listenersChain) Less(i, j int) bool {
	return c[i].priority < c[j].priority
}
