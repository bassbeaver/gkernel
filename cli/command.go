package cli

import (
	cliKernelError "github.com/bassbeaver/gkernel/cli/error"
)

type Controller func(args []string) cliKernelError.CliError

type Command struct {
	Name       string
	Controller Controller
	Help       string
}

func NewCommand(name string, controller Controller, help string) *Command {
	return &Command{
		Name:       name,
		Controller: controller,
		Help:       help,
	}
}
