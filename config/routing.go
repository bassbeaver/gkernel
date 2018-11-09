package config

import (
	"strings"
)

type RouteConfig struct {
	Url            string
	Methods        []string
	Controller     string
	EventListeners []EventListenerConfig `mapstructure:"event_listeners"`
}

func (c *RouteConfig) ControllerAlias() string {
	return getStringPart(c.Controller, 0)
}

func (c *RouteConfig) ControllerMethod() string {
	return getStringPart(c.Controller, 1)
}

//--------------------

func getStringPart(source string, part int) string {
	parts := strings.Split(source, ":")

	return parts[part]
}
