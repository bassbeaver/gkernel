package cli

import (
	"errors"
	"flag"
	"fmt"
	"github.com/bassbeaver/gioc"
	cliConfig "github.com/bassbeaver/gkernel/cli/config"
	cliKernelError "github.com/bassbeaver/gkernel/cli/error"
	cliEventBus "github.com/bassbeaver/gkernel/cli/event_bus"
	commonConfig "github.com/bassbeaver/gkernel/config"
	commonEventBus "github.com/bassbeaver/gkernel/event_bus"
	commonEvent "github.com/bassbeaver/gkernel/event_bus/event"
	"github.com/bassbeaver/gkernel/helper"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"reflect"
)

type Kernel struct {
	config              *viper.Viper
	configIsRead        bool
	container           *gioc.Container
	commands            map[string]*Command
	eventsRegistry      *commonEventBus.EventsRegistry
	applicationEventBus commonEventBus.EventBus // Event bus for application level events
}

func (k *Kernel) GetContainer() *gioc.Container {
	return k.container
}

func (k *Kernel) GetEventsRegistry() *commonEventBus.EventsRegistry {
	return k.eventsRegistry
}

func (k *Kernel) RegisterListener(eventObj commonEvent.Event, listenerFunc interface{}, priority int) error {
	return k.applicationEventBus.AppendListener(eventObj, listenerFunc, priority)
}

func (k *Kernel) RegisterCommand(command *Command) {
	k.commands[command.Name] = command
}

func (k *Kernel) RegisterService(alias string, factoryMethod interface{}, enableCaching bool) error {
	return helper.RegisterService(
		k.config,
		k.container,
		alias,
		factoryMethod,
		enableCaching,
	)
}

func (k *Kernel) Run(args []string) cliKernelError.CliError {
	if noCycles, cycledService := k.container.CheckCycles(); !noCycles {
		return cliKernelError.NewRuntimeError("Failed to start Kernel, errors in DI container: service " + cycledService + " has circular dependencies")
	}

	// Config files reading
	configError := k.readConfig()
	if nil != configError {
		return configError
	}

	// Check if common help flag requested
	helpFlagsSet := flag.NewFlagSet("", flag.ContinueOnError)
	helpFlagsSet.SetOutput(ioutil.Discard) // Disabling native output of flag package

	helpFlag := helpFlagsSet.Bool("help", false, "Output help")
	helpFlagsSet.BoolVar(helpFlag, "h", false, "Output help (shorthand)")

	flagsErr := helpFlagsSet.Parse(args)
	if nil != flagsErr {
		return cliKernelError.NewRuntimeError("Failed to parse Kernel CLI arguments, error: " + flagsErr.Error())
	}
	if *helpFlag {
		fmt.Printf(k.FormatHelp())

		return nil
	}

	// Determine command
	commonArgsWithoutHelp := helpFlagsSet.Args()

	if 0 == len(commonArgsWithoutHelp) {
		return cliKernelError.NewCommandNotSpecifiedError()
	}

	commandName := commonArgsWithoutHelp[0]
	commandArgs := commonArgsWithoutHelp[1:]

	command, commandExists := k.commands[commandName]
	if !commandExists {
		return cliKernelError.NewCommandNotFoundError()
	}

	// Check if command level help flag requested. Ignoring parsing errors because we only need  to determine if help flag was set or not.
	_ = helpFlagsSet.Parse(commandArgs)
	if *helpFlag {
		var helpString string = "Usage: \n"
		helpString += command.FormatHelp()
		fmt.Printf(helpString)

		return nil
	}

	// Processing of application-level events
	k.applicationEventBus.Dispatch(commonEvent.NewApplicationLaunched(k))

	terminationErrors := make([]error, 0)
	defer k.applicationEventBus.Dispatch(commonEvent.NewApplicationTermination(k, &terminationErrors))

	// Run command processing

	return command.Controller(commandArgs)
}

func (k *Kernel) GetHelp() map[string]string {
	if !k.configIsRead {
		configError := k.readConfig()
		if nil != configError {
			panic(configError)
		}
	}

	result := make(map[string]string)

	for commandName, command := range k.commands {
		result[commandName] = command.Help
	}

	return result
}

func (k *Kernel) FormatHelp() string {
	if !k.configIsRead {
		configError := k.readConfig()
		if nil != configError {
			panic(configError)
		}
	}

	var result string = "Usage: \n"

	for _, command := range k.commands {
		result += command.FormatHelp()
	}

	return result
}

func (k *Kernel) readConfig() cliKernelError.CliError {
	// Parsing routing config
	if k.config.IsSet("cli") {
		for commandLabel := range k.config.GetStringMap("cli.commands") {
			commandConfig := &cliConfig.CommandConfig{}
			commandConfigErr := k.config.UnmarshalKey("cli.commands."+commandLabel, commandConfig)
			if nil != commandConfigErr {
				return cliKernelError.NewRuntimeError("failed to read cli commands config: " + commandConfigErr.Error())
			}

			// Registering command
			controllerObj := k.GetContainer().GetByAlias(commandConfig.ControllerAlias())
			controllerMethodValue := reflect.ValueOf(controllerObj).MethodByName(commandConfig.ControllerMethod())
			if (reflect.Value{}) == controllerMethodValue {
				return cliKernelError.NewRuntimeError(fmt.Sprintf("method %s not found in controller object %s", commandConfig.ControllerMethod(), commandConfig.ControllerAlias()))
			}
			controller := controllerMethodValue.Interface().(func(args []string) cliKernelError.CliError)

			k.RegisterCommand(&Command{
				Name:       commandConfig.Name,
				Controller: controller,
				Help:       commandConfig.Help,
			})
		}
	}

	// Parsing config for application level event listeners
	if k.config.IsSet("event_listeners") {
		applicationLevelListenersConfig := make([]commonConfig.EventListenerConfig, 0)
		commonListenersConfigErr := k.config.UnmarshalKey("event_listeners", &applicationLevelListenersConfig)
		if nil != commonListenersConfigErr {
			return cliKernelError.NewRuntimeError("failed to read application level event listeners config, error: " + commonListenersConfigErr.Error())
		}

		for _, listenerConfig := range applicationLevelListenersConfig {
			listenerObj := k.GetContainer().GetByAlias(listenerConfig.ListenerAlias())
			eventObj, eventRegistryError := k.eventsRegistry.GetEventByName(listenerConfig.EventName)
			if nil != eventRegistryError {
				return cliKernelError.NewRuntimeError(
					fmt.Sprintf(
						"failed to register application level event listener %s, event: %s, error: %s",
						listenerConfig.Listener,
						listenerConfig.EventName,
						eventRegistryError.Error(),
					),
				)
			}

			listenerError := k.RegisterListener(
				eventObj,
				reflect.ValueOf(listenerObj).MethodByName(listenerConfig.ListenerMethod()).Interface(),
				listenerConfig.Priority,
			)
			if nil != listenerError {
				return cliKernelError.NewRuntimeError(
					fmt.Sprintf(
						"failed to register application level event listener %s, event: %s, error: %s",
						listenerConfig.Listener,
						listenerConfig.EventName,
						listenerError.Error(),
					),
				)
			}
		}
	}

	k.configIsRead = true

	return nil
}

//--------------------

func NewKernel(configPath string) (*Kernel, error) {
	// Read config files to temporary viper object
	configObj, configBuildError := helper.BuildConfigFromDir(configPath)
	if nil != configBuildError {
		return nil, configBuildError
	}

	// Creating kernel obj
	kernel := &Kernel{
		config:              viper.New(),
		container:           gioc.NewContainer(),
		commands:            make(map[string]*Command, 0),
		eventsRegistry:      cliEventBus.NewDefaultRegistry(),
		applicationEventBus: commonEventBus.NewEventBus(),
	}

	// Copy known config parts to kernel's viper object
	func(params []string, source, target *viper.Viper) {
		for _, param := range params {
			if source.IsSet(param) {
				target.Set(param, source.Get(param))
			}
		}
	}(
		[]string{"services", "cli", "event_listeners"},
		configObj,
		kernel.config,
	)

	// Set working directory config
	workdir, workdirError := os.Getwd()
	if nil != workdirError {
		return nil, errors.New("failed to determine working directory, error: " + workdirError.Error())
	}
	kernel.config.Set("workdir", workdir)

	// Setting parameters to container
	if configObj.IsSet("parameters") {
		parametersStringMap := configObj.GetStringMapString("parameters")
		kernel.container.SetParameters(parametersStringMap)
	}

	return kernel, nil
}
