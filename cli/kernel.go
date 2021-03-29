package cli

import (
	"github.com/bassbeaver/gioc"
	cliKernelError "github.com/bassbeaver/gkernel/cli/error"
	commonEventBus "github.com/bassbeaver/gkernel/event_bus"
	"github.com/spf13/viper"
)

type Kernel struct {
	config         *viper.Viper
	container      *gioc.Container
	commands       map[string]*Command
	eventsRegistry *commonEventBus.EventsRegistry
}

func (k *Kernel) GetContainer() *gioc.Container {
	return k.container
}

func (k *Kernel) GetEventsRegistry() *commonEventBus.EventsRegistry {
	return k.eventsRegistry
}

func (k *Kernel) RegisterCommand(command *Command) {
	k.commands[command.Name] = command
}

func (k *Kernel) Run(args []string) cliKernelError.CliError {
	if 0 == len(args) {
		return cliKernelError.NewCommandNotSpecifiedError()
	}

	commandName := args[0]
	args = args[1:]

	command, commandExists := k.commands[commandName]
	if !commandExists {
		return cliKernelError.NewCommandNotFoundError()
	}

	return command.Controller(args)
}
