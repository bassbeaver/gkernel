package web

import (
	"github.com/bassbeaver/gkernel/event_bus"
	"github.com/bassbeaver/gkernel/web/response"
	"net/http"
	"time"
)

type Controller func(*http.Request) response.Response
type TimeoutHandler func() (int, string)

//--------------------

type Route struct {
	Name           string
	Methods        []string
	Url            string
	Controller     Controller
	Timeout        time.Duration
	TimeoutHandler TimeoutHandler
	eventBus       event_bus.EventBus
}
