package config

import "github.com/bassbeaver/gkernel/helper"

type CommandConfig struct {
	Name       string
	Controller string
	Help       string
}

func (c *CommandConfig) ControllerAlias() string {
	return helper.GetStringPart(c.Controller, ":", 0)
}

func (c *CommandConfig) ControllerMethod() string {
	return helper.GetStringPart(c.Controller, ":", 1)
}
