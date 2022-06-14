package config

import "github.com/bassbeaver/gkernel/helper"

type TimeoutConfig struct {
	Duration string
	Handler  string
}

func (c *TimeoutConfig) HandlerAlias() string {
	return helper.GetStringPart(c.Handler, ":", 0)
}

func (c *TimeoutConfig) HandlerMethod() string {
	return helper.GetStringPart(c.Handler, ":", 1)
}
