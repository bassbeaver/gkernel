package event_bus

import (
	"errors"
	"fmt"
	"github.com/bassbeaver/gkernel/event_bus/event"
	"github.com/bassbeaver/gkernel/event_bus/listener"
	"reflect"
	"sort"
)

type EventBus map[reflect.Type]listenersChain

// eventObj - should be a pointer to the Event object
// listenerFunc - should be a function with one argument, that argument should be a pointer to the Event object
// priority - priority of this listener in listeners chain
func (b EventBus) AppendListener(eventObj event.Event, listenerFunc interface{}, priority int) error {
	eventType := reflect.TypeOf(eventObj)

	if isListener := listener.IsListenerForEvent(listenerFunc, eventObj); !isListener {
		return errors.New(
			fmt.Sprintf(
				"%s is not event listener for %s",
				reflect.TypeOf(listenerFunc).String(),
				eventType.String(),
			),
		)
	}

	if _, chainExists := b[eventType]; !chainExists {
		b[eventType] = make(listenersChain, 0)
	}

	b[eventType] = append(
		b[eventType],
		&listenersChainElement{
			listenerReflectValue: reflect.ValueOf(listenerFunc),
			priority:             priority,
		},
	)
	sort.Sort(b[eventType])

	return nil
}

func (b EventBus) Dispatch(eventObj event.Event) {
	eventType := reflect.TypeOf(eventObj)

	listenersChain, listenersChainExists := b[eventType]
	if !listenersChainExists {
		return
	}

	for _, chainElement := range listenersChain {
		chainElement.listenerReflectValue.Call([]reflect.Value{reflect.ValueOf(eventObj)})

		if eventObj.IsPropagationStopped() {
			return
		}
	}
}

//--------------------

func NewEventBus() EventBus {
	return make(EventBus)
}
