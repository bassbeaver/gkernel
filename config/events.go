package config

import "github.com/bassbeaver/gkernel/helper"

type EventListenerConfig struct {
	EventName string `mapstructure:"Event"`
	Listener  string
	Priority  int
}

func (c *EventListenerConfig) ListenerAlias() string {
	return helper.GetStringPart(c.Listener, ":", 0)
}

func (c *EventListenerConfig) ListenerMethod() string {
	return helper.GetStringPart(c.Listener, ":", 1)
}
