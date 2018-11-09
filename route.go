package gkernel

import (
	"github.com/bassbeaver/gkernel/event_bus"
	"github.com/bassbeaver/gkernel/response"
	"net/http"
)

type Controller func(*http.Request) response.Response

//--------------------

type Route struct {
	Name       string
	Methods    []string
	Url        string
	Controller Controller
	eventBus   event_bus.EventBus
}
