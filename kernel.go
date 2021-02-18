package gkernel

import (
	"context"
	"errors"
	"fmt"
	"github.com/bassbeaver/gioc"
	"github.com/bassbeaver/gkernel/config"
	kernelError "github.com/bassbeaver/gkernel/error"
	"github.com/bassbeaver/gkernel/event_bus"
	"github.com/bassbeaver/gkernel/event_bus/event"
	"github.com/bassbeaver/gkernel/response"
	"github.com/husobee/vestigo"
	"github.com/spf13/viper"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

const (
	configServicesPrefix           = "services"
	configDefaultShutdownTimeoutMs = 500
	requestCtxEventBusKey          = "event_bus"
)

type Kernel struct {
	config                  *viper.Viper
	container               *gioc.Container
	routes                  map[string]*Route
	notFoundHandlerEventBus event_bus.EventBus // Event bus for request level events for 404 not found case. Filled with event listeners common for all routes.
	eventsRegistry          *event_bus.EventsRegistry
	applicationEventBus     event_bus.EventBus // Event bus for application level events
	templates               *template.Template
	httpServer              *http.Server
}

func (k *Kernel) GetContainer() *gioc.Container {
	return k.container
}

func (k *Kernel) GetEventsRegistry() *event_bus.EventsRegistry {
	return k.eventsRegistry
}

func (k *Kernel) RegisterRoute(route *Route) *Kernel {
	k.routes[route.Name] = route

	return k
}

func (k *Kernel) GetTemplates() *template.Template {
	return k.templates
}

func (k *Kernel) GetHttpServer() *http.Server {
	return k.httpServer
}

func (k *Kernel) RegisterListener(eventObj event.Event, listenerFunc interface{}, priority int) error {
	return k.applicationEventBus.AppendListener(eventObj, listenerFunc, priority)
}

func (k *Kernel) RegisterListenerForRoute(routeName string, eventObj event.Event, listenerFunc interface{}, priority int) error {
	route, routeExists := k.routes[routeName]
	if !routeExists {
		return errors.New("route " + routeName + " not exists")
	}

	if nil == route.eventBus {
		route.eventBus = event_bus.NewEventBus()
	}

	return route.eventBus.AppendListener(eventObj, listenerFunc, priority)
}

func (k *Kernel) RegisterService(alias string, factoryMethod interface{}, enableCaching bool) error {
	configServicePath := configServicesPrefix + "." + alias
	configServiceArgumentsPath := configServicesPrefix + "." + alias + ".arguments"
	if !k.config.IsSet(configServicePath) {
		return errors.New(alias + " service configuration not found")
	}

	var arguments []string
	if k.config.IsSet(configServiceArgumentsPath) {
		arguments = k.config.GetStringSlice(configServiceArgumentsPath)
	} else {
		arguments = make([]string, 0)
	}

	k.container.RegisterServiceFactoryByAlias(
		alias,
		gioc.Factory{
			Create:    factoryMethod,
			Arguments: arguments,
		},
		enableCaching,
	)

	return nil
}

func (k *Kernel) Run() {
	if !k.config.IsSet("http_port") {
		panic("Failed to start application, http port to serve not configured")
	}
	portNum := k.config.GetInt("http_port")

	if noCycles, cycledService := k.container.CheckCycles(); !noCycles {
		panic("Failed to start application, errors in DI container: service " + cycledService + " has circular dependencies")
	}

	// Config files reading
	k.readConfig()

	// Routes handlers setup
	router := vestigo.NewRouter()
	for _, route := range k.routes {
		for _, method := range route.Methods {
			router.Add(method, route.Url, k.createRouteHandler(route))
		}
	}

	// 404 handler setup
	vestigo.CustomNotFoundHandlerFunc(k.createNotFoundHandler())

	// Processing of application-level events
	k.applicationEventBus.Dispatch(event.NewApplicationLaunched(k))

	terminationErrors := make([]error, 0)
	defer k.applicationEventBus.Dispatch(event.NewApplicationTermination(k, &terminationErrors))

	// HTTP Server setup
	k.httpServer.Handler = router
	k.httpServer.Addr = fmt.Sprintf(":%d", portNum)

	// Graceful HTTP Server shutdown on signals setup
	httpShutdownChannel := k.setupGraceShutdown(&terminationErrors)

	// Run HTTP Server and wait for graceful shutdown
	listenError := k.httpServer.ListenAndServe()
	if nil != listenError && http.ErrServerClosed != listenError {
		terminationErrors = append(terminationErrors, listenError)
	}
	<-httpShutdownChannel
}

func (k *Kernel) readConfig() {
	// Parsing templates if templates are configured
	if k.config.IsSet("templates_path") {
		templateError := k.parseTemplatesPath(k.config.GetString("templates_path"))
		if nil != templateError {
			panic(templateError)
		}
	}

	// Parsing routing config
	if k.config.IsSet("routing") {
		// Creating list of common event listeners
		commonListenersConfig := make([]config.EventListenerConfig, 0)
		commonListenersConfigErr := k.config.UnmarshalKey("routing.event_listeners", &commonListenersConfig)
		if nil != commonListenersConfigErr {
			panic("failed to read routing common listeners config: " + commonListenersConfigErr.Error())
		}

		// Register events listeners for 404 Not Found handler
		for _, listenerConfig := range commonListenersConfig {
			eventObj, listenerFunc := k.extractHandlerFromListenerConfig(listenerConfig)
			listenerError := k.notFoundHandlerEventBus.AppendListener(eventObj, listenerFunc, listenerConfig.Priority)
			if nil != listenerError {
				panic("failed to register routes common event listener, error: " + listenerError.Error())
			}
		}

		for routeName := range k.config.GetStringMap("routing.routes") {
			routeConfig := &config.RouteConfig{}
			routeConfigErr := k.config.UnmarshalKey("routing.routes."+routeName, routeConfig)
			if nil != routeConfigErr {
				panic("failed to read routing config: " + routeConfigErr.Error())
			}

			// Registering route
			controllerObj := k.GetContainer().GetByAlias(routeConfig.ControllerAlias())
			controllerMethodValue := reflect.ValueOf(controllerObj).MethodByName(routeConfig.ControllerMethod())
			if (reflect.Value{}) == controllerMethodValue {
				panic(fmt.Sprintf("method %s not found in controller object %s", routeConfig.ControllerMethod(), routeConfig.ControllerAlias()))
			}
			controller := controllerMethodValue.Interface().(func(*http.Request) response.Response)

			k.RegisterRoute(&Route{
				Name:       routeName,
				Url:        routeConfig.Url,
				Methods:    routeConfig.Methods,
				Controller: controller,
				eventBus:   event_bus.NewEventBus(),
			})

			// Registering route's event listeners
			fullPackOfEventListenersConfig := append(routeConfig.EventListeners, commonListenersConfig...)
			for _, listenerConfig := range fullPackOfEventListenersConfig {
				eventObj, listenerFunc := k.extractHandlerFromListenerConfig(listenerConfig)

				listenerError := k.RegisterListenerForRoute(
					routeName,
					eventObj,
					listenerFunc,
					listenerConfig.Priority,
				)
				if nil != listenerError {
					panic("failed to register event listener to route " + routeName + ", error: " + listenerError.Error())
				}
			}
		}
	}

	// Parsing config for application level event listeners
	if k.config.IsSet("event_listeners") {
		applicationLevelListenersConfig := make([]config.EventListenerConfig, 0)
		commonListenersConfigErr := k.config.UnmarshalKey("event_listeners", &applicationLevelListenersConfig)
		if nil != commonListenersConfigErr {
			panic("failed to read application level event listeners config, error: " + commonListenersConfigErr.Error())
		}

		errorHandler := func(lc config.EventListenerConfig, err error) {
			if nil == err {
				return
			}

			panic(
				fmt.Sprintf(
					"failed to register application level event listener %s, event: %s, error: %s",
					lc.Listener,
					lc.EventName,
					err.Error(),
				),
			)
		}

		for _, listenerConfig := range applicationLevelListenersConfig {
			listenerObj := k.GetContainer().GetByAlias(listenerConfig.ListenerAlias())
			eventObj, eventRegistryError := k.eventsRegistry.GetEventByName(listenerConfig.EventName)
			errorHandler(listenerConfig, eventRegistryError)

			listenerError := k.RegisterListener(
				eventObj,
				reflect.ValueOf(listenerObj).MethodByName(listenerConfig.ListenerMethod()).Interface(),
				listenerConfig.Priority,
			)
			errorHandler(listenerConfig, listenerError)
		}
	}
}

func (k *Kernel) extractHandlerFromListenerConfig(listenerConfig config.EventListenerConfig) (eventObj event.Event, listenerFunc interface{}) {
	var eventRegistryError error

	listenerObj := k.GetContainer().GetByAlias(listenerConfig.ListenerAlias())
	eventObj, eventRegistryError = k.eventsRegistry.GetEventByName(listenerConfig.EventName)
	if nil != eventRegistryError {
		panic("failed to extract event handler from event listener config, error: " + eventRegistryError.Error())
	}

	listenerFunc = reflect.ValueOf(listenerObj).MethodByName(listenerConfig.ListenerMethod()).Interface()

	return
}

func (k *Kernel) createRouteHandler(route *Route) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, requestObj *http.Request) {
		RequestContextAppend(requestObj, requestCtxEventBusKey, route.eventBus)
		responseObj := k.runRequestProcessingFlow(responseWriter, requestObj, route.Controller)
		k.runSendResponse(responseWriter, requestObj, responseObj)
	}
}

func (k *Kernel) createNotFoundHandler() http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, requestObj *http.Request) {
		RequestContextAppend(requestObj, requestCtxEventBusKey, k.notFoundHandlerEventBus)
		responseObj := k.runNotFoundFlow(responseWriter, requestObj)
		k.runSendResponse(responseWriter, requestObj, responseObj)
	}
}

func (k *Kernel) parseTemplatesPath(templatesPath string) error {
	fullTemplatesPath := k.config.GetString("workdir") + "/" + templatesPath

	pathWalkError := filepath.Walk(
		fullTemplatesPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				panic(err)
			}

			if info.IsDir() {
				return nil
			}

			tplFileBytes, tplFileReadError := ioutil.ReadFile(path)
			if tplFileReadError != nil {
				return tplFileReadError
			}
			tplFileContent := string(tplFileBytes)

			tplName := strings.Replace(path, fullTemplatesPath+"/", "", -1)

			_, parseError := k.templates.New(tplName).Parse(tplFileContent)

			return parseError
		},
	)

	return pathWalkError
}

func (k *Kernel) runRequestProcessingFlow(
	responseWriterObj http.ResponseWriter,
	requestObj *http.Request,
	controller Controller,
) (responseObj response.Response) {
	defer func() {
		// Recover should be called directly by a deferred function. https://golang.org/ref/spec#Handling_panics
		recoveredError := recover()
		if nil != recoveredError {
			responseObj = k.performRecover(recoveredError, debug.Stack(), responseWriterObj, requestObj)
		}
	}()

	eventBus := GetRequestEventBus(requestObj)

	// Running RequestReceived event processing
	requestReceivedEvent := event.NewRequestReceived(responseWriterObj, requestObj)
	eventBus.Dispatch(requestReceivedEvent)
	responseObj = requestReceivedEvent.GetResponse()

	// Running controller if request pre-processing has not returned response
	if nil == responseObj {
		responseObj = controller(requestObj)

		switch typedResponse := responseObj.(type) {
		// If response is websocket upgrade - perform it
		case *response.WebsocketUpgradeResponse:
			typedResponse.UpgradeToWebsocket(requestObj, responseWriterObj)
		// If response is view - execute template and fill response body
		case *response.ViewResponse:
			if nil == typedResponse.Template {
				typedResponse.SetTemplate(k.templates)
			}
		}

		// Running RequestProcessed event processing
		requestProcessedEvent := event.NewRequestProcessed(responseWriterObj, requestObj, responseObj)
		eventBus.Dispatch(requestProcessedEvent)
		responseObj = requestProcessedEvent.GetResponse()
	}

	return
}

func (k *Kernel) runSendResponse(responseWriter http.ResponseWriter, requestObj *http.Request, responseObj response.Response) {
	k.performResponseSend(responseWriter, requestObj, responseObj)
	go performRequestTermination(requestObj, responseObj)
}

func (k *Kernel) performResponseSend(responseWriterObj http.ResponseWriter, requestObj *http.Request, responseObj response.Response) {
	var responseBody []byte
	var responseStatus int
	var responseHeader http.Header

	defer func() {
		// Recover should be called directly by a deferred function. https://golang.org/ref/spec#Handling_panics
		recoveredError := recover()
		if nil != recoveredError {
			func() {
				// Recover panic inside of panic recovery
				defer func() {
					recoveryRecoveredError := recover()
					if nil != recoveryRecoveredError {
						responseBody = []byte(fmt.Sprintf("Failed to send response. Error: %+v", recoveryRecoveredError))
						responseStatus = http.StatusInternalServerError
						responseHeader = make(http.Header)
					}
				}()

				responseObj = k.performRecover(recoveredError, debug.Stack(), responseWriterObj, requestObj)

				responseBody = responseObj.GetBodyBytes().Bytes()
				responseStatus = responseObj.GetHttpStatus()
				responseHeader = responseObj.GetHeaders()
			}()
		}

		// Sending headers
		for headerName, headerValues := range responseObj.GetHeaders() {
			for _, value := range headerValues {
				responseWriterObj.Header().Add(headerName, value)
			}
		}
		responseWriterObj.WriteHeader(responseStatus)

		// Sending body
		responseWriterObj.Write(responseBody)
	}()

	if nil == responseObj {
		errorResponse := response.NewBytesResponse()
		errorResponse.SetHttpStatus(http.StatusInternalServerError)
		errorResponse.Body.WriteString("failed to send response, no response provided")
		responseObj = errorResponse
	}

	// Running ResponseBeforeSend event processing
	eventBus := GetRequestEventBus(requestObj)
	responseBeforeSendEvent := event.NewResponseBeforeSend(responseWriterObj, requestObj, responseObj)
	eventBus.Dispatch(responseBeforeSendEvent)

	// Get response body bytes before headers where sent to prevent case "Headers sent -> Panic in responseObj.GetBodyBytes()"
	responseBody = responseObj.GetBodyBytes().Bytes()
	responseStatus = responseObj.GetHttpStatus()
	responseHeader = responseObj.GetHeaders()
}

func (k *Kernel) setupGraceShutdown(terminationErrors *[]error) chan bool {
	shutdownTimeout := k.config.GetDuration("shutdown_timeout")
	if 0 >= shutdownTimeout {
		shutdownTimeout = configDefaultShutdownTimeoutMs
	}
	shutdownTimeout = shutdownTimeout * time.Millisecond

	signalsChannel := make(chan os.Signal)
	httpShutdownChannel := make(chan bool)

	signal.Notify(signalsChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer close(signalsChannel)

		<-signalsChannel

		shutdownContext, shutdownContextCancelFunc := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownContextCancelFunc()

		shutdownError := k.httpServer.Shutdown(shutdownContext)
		if nil != shutdownError {
			if context.DeadlineExceeded == shutdownError {
				*terminationErrors = append(
					*terminationErrors,
					errors.New(fmt.Sprintf("Gkernel: graceful shutdown timeout of %s expired", shutdownTimeout)),
				)
			} else {
				*terminationErrors = append(*terminationErrors, shutdownError)
			}
		}

		httpShutdownChannel <- true
	}()

	return httpShutdownChannel
}

func (k *Kernel) runNotFoundFlow(responseWriter http.ResponseWriter, requestObj *http.Request) (responseObj response.Response) {
	defer func() {
		// Recover should be called directly by a deferred function. https://golang.org/ref/spec#Handling_panics
		recoveredError := recover()
		if nil != recoveredError {
			responseObj = k.performRecover(recoveredError, debug.Stack(), responseWriter, requestObj)
		}
	}()

	eventBus := GetRequestEventBus(requestObj)

	errorObj := kernelError.NewNotFoundHttpError()
	runtimeErrorEvent := event.NewRuntimeError(responseWriter, requestObj, kernelError.NewRuntimeError(errorObj, nil))
	eventBus.Dispatch(runtimeErrorEvent)
	responseObj = runtimeErrorEvent.GetResponse()

	if nil == responseObj {
		defaultResponseObj := response.NewBytesResponse()
		defaultResponseObj.SetHttpStatus(errorObj.Status())
		defaultResponseObj.Body.WriteString(errorObj.Message())

		responseObj = defaultResponseObj
	}

	// If returned response is view - set templates to it
	switch typedResponse := responseObj.(type) {
	case *response.ViewResponse:
		if nil == typedResponse.Template {
			typedResponse.SetTemplate(k.templates)
		}
	}

	return
}

func (k *Kernel) performRecover(recoveredError interface{}, trace []byte, responseWriterObj http.ResponseWriter, requestObj *http.Request) response.Response {
	var responseObj response.Response

	runtimeError := kernelError.NewRuntimeError(recoveredError, trace)

	eventBus := GetRequestEventBus(requestObj)

	runtimeErrorEvent := event.NewRuntimeError(responseWriterObj, requestObj, runtimeError)
	eventBus.Dispatch(runtimeErrorEvent)
	responseObj = runtimeErrorEvent.GetResponse()

	if nil == responseObj {
		defaultResponseObj := response.NewBytesResponse()
		defaultResponseObj.SetHttpStatus(http.StatusInternalServerError)
		defaultResponseObj.Body.WriteString(fmt.Sprintf("%+v\n%s", runtimeError.Error, runtimeError.Trace))

		responseObj = defaultResponseObj
	}

	// If recovered response is view - set templates to it
	switch typedResponse := responseObj.(type) {
	case *response.ViewResponse:
		if nil == typedResponse.Template {
			typedResponse.SetTemplate(k.templates)
		}
	}

	return responseObj
}

//--------------------

func NewKernel(configPath string) (*Kernel, error) {
	// Read config files to temporary viper object
	configObj := viper.New()

	var configDir string
	configPathStat, configPathStatError := os.Stat(configPath)
	if nil != configPathStatError {
		return nil, errors.New("failed to read configs: " + configPathStatError.Error())
	}
	if configPathStat.IsDir() {
		configDir = configPath
	} else {
		configDir = filepath.Dir(configPath)
	}

	firstConfigFile := true
	pathWalkError := filepath.Walk(
		configDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return errors.New("failed to read config file " + path + ", error: " + err.Error())
			}

			if info.IsDir() {
				return nil
			}

			configFilePath := filepath.Dir(path)
			configFileExt := filepath.Ext(info.Name())
			// if extension is not allowed - take next file
			if !stringInSlice(configFileExt[1:], viper.SupportedExts) {
				return nil
			}

			configFileName := info.Name()[0 : len(info.Name())-len(configFileExt)]

			configObj.AddConfigPath(configFilePath)
			configObj.SetConfigName(configFileName)

			if firstConfigFile {
				if configError := configObj.ReadInConfig(); nil != configError {
					return configError
				}

				firstConfigFile = false
			} else {
				if configError := configObj.MergeInConfig(); nil != configError {
					return configError
				}
			}

			return nil
		},
	)
	if nil != pathWalkError {
		return nil, errors.New("failed to read configs: " + pathWalkError.Error())
	}

	// Creating kernel obj
	kernel := &Kernel{
		config:                  viper.New(),
		container:               gioc.NewContainer(),
		routes:                  make(map[string]*Route, 0),
		notFoundHandlerEventBus: event_bus.NewEventBus(),
		eventsRegistry:          event_bus.NewDefaultRegistry(),
		applicationEventBus:     event_bus.NewEventBus(),
		templates:               template.New("root"),
		httpServer:              &http.Server{},
	}

	// Copy known config parts to kernel's viper object
	func(params []string, source, target *viper.Viper) {
		for _, param := range params {
			if source.IsSet(param) {
				target.Set(param, source.Get(param))
			}
		}
	}(
		[]string{"http_port", "templates_path", "shutdown_timeout", "services", "routing", "event_listeners"},
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

//--------------------

func performRequestTermination(requestObj *http.Request, responseObj response.Response) {
	requestTerminationEvent := event.NewRequestTermination(requestObj, responseObj)
	GetRequestEventBus(requestObj).Dispatch(requestTerminationEvent)
}

func GetRequestEventBus(requestObj *http.Request) event_bus.EventBus {
	eventBus := requestObj.Context().Value(requestCtxEventBusKey)
	if nil != eventBus {
		if eventBusTyped, isEventBus := eventBus.(event_bus.EventBus); isEventBus {
			return eventBusTyped
		}
	}

	panic("EventBus not set to request object")
}

func RequestContextAppend(requestObj *http.Request, key, val interface{}) {
	newContext := context.WithValue(requestObj.Context(), key, val)
	*requestObj = *requestObj.WithContext(newContext)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}

	return false
}
