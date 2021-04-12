package cli

import (
	"fmt"
	cliKernelError "github.com/bassbeaver/gkernel/cli/error"
)

type Controller func(args []string) cliKernelError.CliError

type Command struct {
	Name       string
	Controller Controller
	Help       string
}

func (c *Command) FormatHelp() string {
	return fmt.Sprintf("  %s\t%s\n", c.Name, c.Help)
}

func NewCommand(name string, controller Controller, help string) *Command {
	return &Command{
		Name:       name,
		Controller: controller,
		Help:       help,
	}
}
