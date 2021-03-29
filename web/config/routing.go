package config

import (
	commonConfig "github.com/bassbeaver/gkernel/config"
	"github.com/bassbeaver/gkernel/helper"
)

type RouteConfig struct {
	Url            string
	Methods        []string
	Controller     string
	EventListeners []commonConfig.EventListenerConfig `mapstructure:"event_listeners"`
}

func (c *RouteConfig) ControllerAlias() string {
	return helper.GetStringPart(c.Controller, ":", 0)
}

func (c *RouteConfig) ControllerMethod() string {
	return helper.GetStringPart(c.Controller, ":", 1)
}
