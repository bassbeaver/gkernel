package listener

import webEvent "github.com/bassbeaver/gkernel/web/event_bus/event"

type RequestReceived func(received *webEvent.RequestReceived)

type RequestProcessed func(*webEvent.RequestProcessed)

type ResponseBeforeSend func(*webEvent.ResponseBeforeSend)

type RequestTermination func(*webEvent.RequestTermination)

type RuntimeError func(*webEvent.RequestTermination)
