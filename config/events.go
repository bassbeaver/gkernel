package config

type EventListenerConfig struct {
	EventName string `mapstructure:"Event"`
	Listener  string
	Priority  int
}

func (c *EventListenerConfig) ListenerAlias() string {
	return getStringPart(c.Listener, 0)
}

func (c *EventListenerConfig) ListenerMethod() string {
	return getStringPart(c.Listener, 1)
}
