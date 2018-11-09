package listener

import (
	"github.com/bassbeaver/gkernel/event_bus/event"
	"reflect"
)

type ApplicationLaunched func(*event.ApplicationLaunched)

type ApplicationTermination func(*event.ApplicationTermination)

type RequestReceived func(*event.RequestReceived)

type RequestProcessed func(*event.RequestProcessed)

type ResponseBeforeSend func(*event.ResponseBeforeSend)

type RequestTermination func(*event.RequestTermination)

type RuntimeError func(*event.RequestTermination)

//--------------------

// listener - should be a function with one argument, that argument should be a pointer to the Event object
// event - should be a pointer to the Event object
func IsListenerForEvent(listener interface{}, eventPtr event.Event) bool {
	listenerType := reflect.TypeOf(listener)
	if reflect.Func != listenerType.Kind() || listenerType.NumIn() != 1 {
		return false
	}

	listenerArgumentType := listenerType.In(0)
	if reflect.Ptr != listenerArgumentType.Kind() || listenerArgumentType != reflect.TypeOf(eventPtr) {
		return false
	}

	return true
}
