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
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
)

const (
	configServicesPrefix = "services"
)

type Kernel struct {
	config         *viper.Viper
	container      *gioc.Container
	routes         map[string]*Route
	eventsRegistry *event_bus.EventsRegistry
	eventBus       event_bus.EventBus
	templates      *template.Template
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

func (k *Kernel) RegisterListener(eventObj event.Event, listenerFunc interface{}, priority int) error {
	return k.eventBus.AppendListener(eventObj, listenerFunc, priority)
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
	if noCycles, cycledService := k.container.CheckCycles(); !noCycles {
		panic("Failed to start application, errors in DI container: service " + cycledService + " has circular dependencies")
	}

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

	if !k.config.IsSet("http_port") {
		panic("Failed to start application, http port to serve not configured")
	}
	portNum := k.config.GetInt("http_port")

	// Processing of application-level events
	k.eventBus.Dispatch(event.NewApplicationLaunched(k))
	defer k.eventBus.Dispatch(event.NewApplicationTermination(k))

	e := http.ListenAndServe(fmt.Sprintf(":%d", portNum), router)
	panic(e)
}

func (k *Kernel) readConfig() {
	// Parsing templates if templates are configured
	if k.config.IsSet("templates_path") {
		var templateError error
		k.templates, templateError = k.parseTemplatesPath(k.config.GetString("templates_path"))
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

		for routeName := range k.config.GetStringMap("routing.routes") {
			routeConfig := &config.RouteConfig{}
			routeConfigErr := k.config.UnmarshalKey("routing.routes."+routeName, routeConfig)
			if nil != routeConfigErr {
				panic("failed to read routing config: " + routeConfigErr.Error())
			}

			// Registering route
			controllerObj := k.GetContainer().GetByAlias(routeConfig.ControllerAlias())
			controller := reflect.ValueOf(controllerObj).MethodByName(routeConfig.ControllerMethod()).Interface().(func(*http.Request) response.Response)

			k.RegisterRoute(&Route{
				Name:       routeName,
				Url:        routeConfig.Url,
				Methods:    routeConfig.Methods,
				Controller: controller,
			})

			// Registering route's event listeners
			fullPackOfEventListenersConfig := append(routeConfig.EventListeners, commonListenersConfig...)
			for _, listenerConfig := range fullPackOfEventListenersConfig {
				listenerObj := k.GetContainer().GetByAlias(listenerConfig.ListenerAlias())
				eventObj, eventRegistryError := k.eventsRegistry.GetEventByName(listenerConfig.EventName)
				if nil != eventRegistryError {
					panic("failed to register event listener to route " + routeName + ", error: " + eventRegistryError.Error())
				}

				listenerError := k.RegisterListenerForRoute(
					routeName,
					eventObj,
					reflect.ValueOf(listenerObj).MethodByName(listenerConfig.ListenerMethod()).Interface(),
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

		for _, listenerConfig := range applicationLevelListenersConfig {
			listenerObj := k.GetContainer().GetByAlias(listenerConfig.ListenerAlias())
			eventObj, eventRegistryError := k.eventsRegistry.GetEventByName(listenerConfig.EventName)
			if nil != eventRegistryError {
				panic("failed to register application level event listener, error: " + eventRegistryError.Error())
			}

			listenerError := k.RegisterListener(
				eventObj,
				reflect.ValueOf(listenerObj).MethodByName(listenerConfig.ListenerMethod()).Interface(),
				listenerConfig.Priority,
			)
			if nil != listenerError {
				panic("failed to register application level event listener, error: " + listenerError.Error())
			}
		}
	}
}

func (k *Kernel) createRouteHandler(route *Route) http.HandlerFunc {
	var eventBus event_bus.EventBus
	// If route has listeners - use that listeners, if no - use global listeners from Kernel
	if nil != route.eventBus {
		eventBus = route.eventBus
	} else {
		eventBus = k.eventBus
	}

	return http.HandlerFunc(func(responseWriter http.ResponseWriter, requestObj *http.Request) {
		RequestContextAppend(requestObj, "event_bus", eventBus)
		responseObj := k.runRequestProcessingFlow(requestObj, route.Controller)
		k.runSendResponse(requestObj, responseObj, responseWriter)
	})
}

func (k *Kernel) createNotFoundHandler() http.HandlerFunc {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, requestObj *http.Request) {
		RequestContextAppend(requestObj, "event_bus", k.eventBus)
		responseObj := runNotFoundFlow(requestObj)
		k.runSendResponse(requestObj, responseObj, responseWriter)
	})
}

func (k *Kernel) parseTemplatesPath(templatesPath string) (*template.Template, error) {
	result := template.New("root")

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

			result.New(tplName).Parse(tplFileContent)

			return nil
		},
	)
	if nil != pathWalkError {
		return nil, pathWalkError
	}

	return result, nil
}

func (k *Kernel) runRequestProcessingFlow(requestObj *http.Request, controller Controller) (responseObj response.Response) {
	defer func() {
		// Recover should be called directly by a deferred function. https://golang.org/ref/spec#Handling_panics
		recoveredError := recover()
		if nil == recoveredError {
			return
		}

		responseObj = performRecover(recoveredError, debug.Stack(), requestObj)
	}()

	eventBus := GetRequestEventBus(requestObj)

	// Running RequestReceived event processing
	requestReceivedEvent := event.NewRequestReceived(requestObj)
	eventBus.Dispatch(requestReceivedEvent)
	responseObj = requestReceivedEvent.GetResponse()

	// Running controller if request pre-processing has not returned response
	if nil == responseObj {
		responseObj = controller(requestObj)

		// Running RequestProcessed event processing
		requestProcessedEvent := event.NewRequestProcessed(requestObj, responseObj)
		eventBus.Dispatch(requestProcessedEvent)
		responseObj = requestProcessedEvent.GetResponse()
	}

	return
}

func (k *Kernel) runSendResponse(requestObj *http.Request, responseObj response.Response, responseWriter http.ResponseWriter) {
	k.performResponseSend(requestObj, responseObj, responseWriter)
	go performRequestTermination(requestObj, responseObj)
}

func (k *Kernel) performResponseSend(requestObj *http.Request, responseObj response.Response, responseWriter http.ResponseWriter) {
	defer func() {
		// Recover should be called directly by a deferred function. https://golang.org/ref/spec#Handling_panics
		recoveredError := recover()
		if nil == recoveredError {
			return
		}

		responseWriter.WriteHeader(http.StatusInternalServerError)
		responseWriter.Write([]byte(fmt.Sprintf("Failed to send response. Error: %+v", recoveredError)))
	}()

	if nil == responseObj {
		errorResponse := response.NewBytesResponse()
		errorResponse.SetHttpStatus(http.StatusInternalServerError)
		errorResponse.Body.WriteString("failed to send response, no response provided")
		responseObj = errorResponse
	}

	// Running ResponseBeforeSend event processing
	eventBus := GetRequestEventBus(requestObj)
	responseBeforeSendEvent := event.NewResponseBeforeSend(requestObj, responseObj)
	eventBus.Dispatch(responseBeforeSendEvent)

	// If response is view - execute template and fill response body
	switch typedResponse := responseObj.(type) {
	case *response.ViewResponse:
		if nil == typedResponse.Template {
			typedResponse.SetTemplate(k.templates)
		}
	}

	// Get response body before headers where sent to prevent case "Headers sent -> Panic in responseObj.GetBodyBytes()"
	responseBody := responseObj.GetBodyBytes().Bytes()

	// Sending headers
	for headerName, headerValues := range responseObj.GetHeaders() {
		for _, value := range headerValues {
			responseWriter.Header().Add(headerName, value)
		}
	}
	responseWriter.WriteHeader(responseObj.GetHttpStatus())

	// Sending body
	responseWriter.Write(responseBody)
}

//--------------------

func NewKernel(configFile string) (*Kernel, error) {
	copyParam := func(params []string, source, target *viper.Viper) {
		for _, param := range params {
			if source.IsSet(param) {
				target.Set(param, source.Get(param))
			}
		}
	}

	kernel := &Kernel{
		config:         viper.New(),
		container:      gioc.NewContainer(),
		routes:         make(map[string]*Route, 0),
		eventsRegistry: event_bus.NewDefaultRegistry(),
		eventBus:       event_bus.NewEventBus(),
	}

	configObj := viper.New()
	configObj.SetConfigFile(configFile)
	if configError := configObj.ReadInConfig(); nil != configError {
		return nil, configError
	}

	// Setting config to kernel
	kernel.config.Set("workdir", filepath.Dir(configFile))
	copyParam(
		[]string{"http_port", "templates_path", "services", "routing", "event_listeners"},
		configObj,
		kernel.config,
	)

	// Setting parameters to container
	if configObj.IsSet("parameters") {
		parametersStringMap := configObj.GetStringMapString("parameters")
		kernel.container.SetParameters(parametersStringMap)
	}

	return kernel, nil
}

//--------------------

func runNotFoundFlow(requestObj *http.Request) (responseObj response.Response) {
	defer func() {
		// Recover should be called directly by a deferred function. https://golang.org/ref/spec#Handling_panics
		recoveredError := recover()
		if nil == recoveredError {
			return
		}

		responseObj = performRecover(recoveredError, debug.Stack(), requestObj)
	}()

	eventBus := GetRequestEventBus(requestObj)

	errorObj := kernelError.NewNotFoundHttpError()
	runtimeErrorEvent := event.NewRuntimeError(requestObj, kernelError.NewRuntimeError(errorObj, nil))
	eventBus.Dispatch(runtimeErrorEvent)
	responseObj = runtimeErrorEvent.GetResponse()

	if nil == responseObj {
		defaultResponseObj := response.NewBytesResponse()
		defaultResponseObj.SetHttpStatus(errorObj.Status())
		defaultResponseObj.Body.WriteString(errorObj.Message())

		responseObj = defaultResponseObj
	}

	return
}

func performRequestTermination(requestObj *http.Request, responseObj response.Response) {
	requestTerminationEvent := event.NewRequestTermination(requestObj, responseObj)
	GetRequestEventBus(requestObj).Dispatch(requestTerminationEvent)
}

func performRecover(recoveredError interface{}, trace []byte, requestObj *http.Request) response.Response {
	var responseObj response.Response

	runtimeError := kernelError.NewRuntimeError(recoveredError, trace)

	eventBus := GetRequestEventBus(requestObj)

	runtimeErrorEvent := event.NewRuntimeError(requestObj, runtimeError)
	eventBus.Dispatch(runtimeErrorEvent)
	responseObj = runtimeErrorEvent.GetResponse()

	if nil == responseObj {
		defaultResponseObj := response.NewBytesResponse()
		defaultResponseObj.SetHttpStatus(http.StatusInternalServerError)
		defaultResponseObj.Body.WriteString(fmt.Sprintf("%+v\n%s", runtimeError.Error, runtimeError.Trace))

		responseObj = defaultResponseObj
	}

	return responseObj
}

func GetRequestEventBus(requestObj *http.Request) event_bus.EventBus {
	eventBus := requestObj.Context().Value("event_bus")
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
