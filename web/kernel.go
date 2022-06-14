package web

import (
	"context"
	"errors"
	"fmt"
	"github.com/bassbeaver/gioc"
	commonConfig "github.com/bassbeaver/gkernel/config"
	commonKernelError "github.com/bassbeaver/gkernel/error"
	commonEventBus "github.com/bassbeaver/gkernel/event_bus"
	commonEvent "github.com/bassbeaver/gkernel/event_bus/event"
	"github.com/bassbeaver/gkernel/helper"
	webConfig "github.com/bassbeaver/gkernel/web/config"
	webKernelError "github.com/bassbeaver/gkernel/web/error"
	webEventBus "github.com/bassbeaver/gkernel/web/event_bus"
	webEvent "github.com/bassbeaver/gkernel/web/event_bus/event"
	"github.com/bassbeaver/gkernel/web/response"
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
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	configDefaultShutdownTimeoutMs = 500
	requestCtxEventBusKey          = "event_bus"
	defaultReadHeaderTimeoutMs     = 5000
	defaultReadTimeoutMs           = 15000
	defaultWriteTimeoutMs          = 30000
	defaultIdleTimeoutMs           = 15000
	defaultRouteTimeoutMs          = 20000
)

type Kernel struct {
	config                  *viper.Viper
	container               *gioc.Container
	routes                  map[string]*Route
	notFoundHandlerEventBus commonEventBus.EventBus // Event bus for request level events for 404 not found case. Filled with event listeners common for all routes.
	eventsRegistry          *commonEventBus.EventsRegistry
	applicationEventBus     commonEventBus.EventBus // Event bus for application level events
	templates               *template.Template
	httpServer              *http.Server
}

func (k *Kernel) GetContainer() *gioc.Container {
	return k.container
}

func (k *Kernel) GetEventsRegistry() *commonEventBus.EventsRegistry {
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

func (k *Kernel) RegisterListener(eventObj commonEvent.Event, listenerFunc interface{}, priority int) error {
	return k.applicationEventBus.AppendListener(eventObj, listenerFunc, priority)
}

func (k *Kernel) RegisterListenerForRoute(routeName string, eventObj commonEvent.Event, listenerFunc interface{}, priority int) error {
	route, routeExists := k.routes[routeName]
	if !routeExists {
		return errors.New("route " + routeName + " not exists")
	}

	if nil == route.eventBus {
		route.eventBus = commonEventBus.NewEventBus()
	}

	return route.eventBus.AppendListener(eventObj, listenerFunc, priority)
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

func (k *Kernel) Run() {
	if !k.config.IsSet("web.http_port") {
		panic("Failed to start application, http port to serve not configured")
	}
	portNum := k.config.GetInt("web.http_port")

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
	k.applicationEventBus.Dispatch(commonEvent.NewApplicationLaunched(k))

	terminationErrors := make([]error, 0)
	defer k.applicationEventBus.Dispatch(commonEvent.NewApplicationTermination(k, &terminationErrors))

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
	if k.config.IsSet("web.templates_path") {
		templateError := k.parseTemplatesPath(k.config.GetString("web.templates_path"))
		if nil != templateError {
			panic(templateError)
		}
	}

	// Parsing routing config
	if k.config.IsSet("web.routing") {
		// Creating list of common event listeners
		commonListenersConfig := make([]commonConfig.EventListenerConfig, 0)
		commonListenersConfigErr := k.config.UnmarshalKey("web.routing.event_listeners", &commonListenersConfig)
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

		// Create common timeout handler
		commonTimeoutConfig := webConfig.TimeoutConfig{}
		commonTimeoutConfigErr := k.config.UnmarshalKey("web.routing.timeout", &commonTimeoutConfig)
		if nil != commonTimeoutConfigErr {
			panic("failed to read routing common listeners config: " + commonListenersConfigErr.Error())
		}
		commonTimeout, commonTimeoutHandler := k.extractTimeoutHandlerFromConfig(
			commonTimeoutConfig,
			"common timeout config",
			defaultRouteTimeoutMs,
			func() response.Response {
				tr := response.NewBytesResponse()
				tr.SetHttpStatus(http.StatusServiceUnavailable)
				tr.Body.WriteString("timeout")

				return tr
			},
		)

		for routeName := range k.config.GetStringMap("web.routing.routes") {
			routeConfig := &webConfig.RouteConfig{}
			routeConfigErr := k.config.UnmarshalKey("web.routing.routes."+routeName, routeConfig)
			if nil != routeConfigErr {
				panic("failed to read routing config: " + routeConfigErr.Error())
			}

			// Determining controller
			controllerObj := k.GetContainer().GetByAlias(routeConfig.ControllerAlias())
			controllerMethodValue := reflect.ValueOf(controllerObj).MethodByName(routeConfig.ControllerMethod())
			if (reflect.Value{}) == controllerMethodValue {
				panic(fmt.Sprintf("method %s not found in Controller object %s", routeConfig.ControllerMethod(), routeConfig.ControllerAlias()))
			}
			controller := controllerMethodValue.Interface().(func(*http.Request) response.Response)

			// Determining route timeout and timeout handler
			routeTimeout, routeTimeoutHandler := k.extractTimeoutHandlerFromConfig(
				routeConfig.Timeout,
				fmt.Sprintf(
					"route '%s %s' has non-integer value of timeout (%s)", strings.Join(routeConfig.Methods, " "),
					routeConfig.Url,
					routeConfig.Timeout,
				),
				commonTimeout,
				commonTimeoutHandler,
			)

			// Registering route to Kernel
			k.RegisterRoute(&Route{
				Name:           routeName,
				Url:            routeConfig.Url,
				Methods:        routeConfig.Methods,
				Controller:     controller,
				Timeout:        time.Millisecond * time.Duration(routeTimeout),
				TimeoutHandler: routeTimeoutHandler,
				eventBus:       commonEventBus.NewEventBus(),
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
		applicationLevelListenersConfig := make([]commonConfig.EventListenerConfig, 0)
		commonListenersConfigErr := k.config.UnmarshalKey("event_listeners", &applicationLevelListenersConfig)
		if nil != commonListenersConfigErr {
			panic("failed to read application level event listeners config, error: " + commonListenersConfigErr.Error())
		}

		errorHandler := func(lc commonConfig.EventListenerConfig, err error) {
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

func (k *Kernel) extractHandlerFromListenerConfig(listenerConfig commonConfig.EventListenerConfig) (eventObj commonEvent.Event, listenerFunc interface{}) {
	var eventRegistryError error

	listenerObj := k.GetContainer().GetByAlias(listenerConfig.ListenerAlias())
	eventObj, eventRegistryError = k.eventsRegistry.GetEventByName(listenerConfig.EventName)
	if nil != eventRegistryError {
		panic("failed to extract event handler from event listener config, error: " + eventRegistryError.Error())
	}

	listenerFunc = reflect.ValueOf(listenerObj).MethodByName(listenerConfig.ListenerMethod()).Interface()

	return
}

func (k *Kernel) extractTimeoutHandlerFromConfig(
	timeoutConfig webConfig.TimeoutConfig,
	description string,
	defaultTimeout int,
	defaultHandler TimeoutHandler,
) (int, TimeoutHandler) {
	var timeout int
	var handler TimeoutHandler

	if "" == timeoutConfig.Duration {
		timeout = defaultTimeout
	} else {
		var timeoutError error
		timeout, timeoutError = strconv.Atoi(timeoutConfig.Duration)
		if nil != timeoutError {
			panic(fmt.Sprintf("%s has non-integer value of timeout (%s)", description, timeoutConfig.Duration))
		}
	}

	if "" == timeoutConfig.Handler {
		handler = defaultHandler
	} else {
		timeoutHandlerObj := k.GetContainer().GetByAlias(timeoutConfig.HandlerAlias())
		timeoutHandlerMethodValue := reflect.ValueOf(timeoutHandlerObj).MethodByName(timeoutConfig.HandlerMethod())
		if (reflect.Value{}) == timeoutHandlerMethodValue {
			panic(fmt.Sprintf("method %s not found in TimeoutHandler object %s", timeoutConfig.HandlerMethod(), timeoutConfig.HandlerAlias()))
		}
		handler = timeoutHandlerMethodValue.Interface().(func() response.Response)
	}

	return timeout, handler
}

func (k *Kernel) createRouteHandler(route *Route) http.HandlerFunc {
	plainHandlerFunc := func(responseWriter http.ResponseWriter, requestObj *http.Request) {
		RequestContextAppend(requestObj, requestCtxEventBusKey, route.eventBus)
		responseObj := k.runRequestProcessingFlow(responseWriter, requestObj, route.Controller)
		k.runSendResponse(responseWriter, requestObj, responseObj)
	}

	if 0 == route.Timeout {
		return plainHandlerFunc
	}

	return k.timeoutHandler(plainHandlerFunc, route.Timeout, route.TimeoutHandler)
}

func (k *Kernel) timeoutHandler(plainHandler http.HandlerFunc, timeout time.Duration, timeoutHandler TimeoutHandler) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, requestObj *http.Request) {
		timeoutCtx, cancelTimeoutCtx := context.WithTimeout(requestObj.Context(), timeout)
		defer cancelTimeoutCtx()

		requestObj = requestObj.WithContext(timeoutCtx)
		plainHandlerDone := make(chan struct{})
		timeoutWriterObj := newTimeoutWriter(responseWriter, requestObj)

		// Spin processing of plain handler
		go func() {
			plainHandler(timeoutWriterObj, requestObj)
			close(plainHandlerDone)
		}()

		// Waiting for completion of plain handler or timeout
		select {
		case <-plainHandlerDone:
			timeoutWriterObj.writeMutex.Lock()
			defer timeoutWriterObj.writeMutex.Unlock()
			for headerName, headerVal := range timeoutWriterObj.Header() {
				responseWriter.Header()[headerName] = headerVal
			}
			responseWriter.WriteHeader(timeoutWriterObj.httpStatus)
			_, _ = responseWriter.Write(timeoutWriterObj.bodyBytes.Bytes())
		case <-timeoutCtx.Done():
			timeoutWriterObj.writeMutex.Lock()
			defer timeoutWriterObj.writeMutex.Unlock()
			timeoutResponse := timeoutHandler()
			k.performRunRequestPostProcessing(responseWriter, requestObj, timeoutResponse)
			for headerName, headerVal := range timeoutResponse.GetHeaders() {
				responseWriter.Header()[headerName] = headerVal
			}
			responseWriter.WriteHeader(timeoutResponse.GetHttpStatus())
			_, _ = responseWriter.Write(timeoutResponse.GetBodyBytes().Bytes())
		}
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
	requestReceivedEvent := webEvent.NewRequestReceived(responseWriterObj, requestObj)
	eventBus.Dispatch(requestReceivedEvent)
	responseObj = requestReceivedEvent.GetResponse()

	// Running controller if request pre-processing has not returned response
	if nil == responseObj {
		responseObj = controller(requestObj)

		k.performRunRequestPostProcessing(responseWriterObj, requestObj, responseObj)

		// Running RequestProcessed event processing
		requestProcessedEvent := webEvent.NewRequestProcessed(responseWriterObj, requestObj, responseObj)
		eventBus.Dispatch(requestProcessedEvent)
		responseObj = requestProcessedEvent.GetResponse()
	} else {
		k.performRunRequestPostProcessing(responseWriterObj, requestObj, responseObj)
	}

	return
}

func (k *Kernel) performRunRequestPostProcessing(
	responseWriterObj http.ResponseWriter,
	requestObj *http.Request,
	responseObj response.Response,
) {
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

				recoverResponseObj := k.performRecover(recoveredError, debug.Stack(), responseWriterObj, requestObj)

				responseBody = recoverResponseObj.GetBodyBytes().Bytes()
				responseStatus = recoverResponseObj.GetHttpStatus()
				responseHeader = recoverResponseObj.GetHeaders()
			}()
		}

		// Sending headers
		for headerName, headerValues := range responseHeader {
			for _, value := range headerValues {
				responseWriterObj.Header().Add(headerName, value)
			}
		}
		responseWriterObj.WriteHeader(responseStatus)

		// Sending body
		_, _ = responseWriterObj.Write(responseBody)
	}()

	if nil == responseObj {
		errorResponse := response.NewBytesResponse()
		errorResponse.SetHttpStatus(http.StatusInternalServerError)
		errorResponse.Body.WriteString("failed to send response, no response provided")
		responseObj = errorResponse
	}

	// Running ResponseBeforeSend event processing
	eventBus := GetRequestEventBus(requestObj)
	responseBeforeSendEvent := webEvent.NewResponseBeforeSend(responseWriterObj, requestObj, responseObj)
	eventBus.Dispatch(responseBeforeSendEvent)

	// Get response body bytes before headers where sent to prevent case "Headers sent -> Panic in responseObj.GetBodyBytes()"
	responseBody = responseObj.GetBodyBytes().Bytes()
	responseStatus = responseObj.GetHttpStatus()
	responseHeader = responseObj.GetHeaders()
}

func (k *Kernel) setupGraceShutdown(terminationErrors *[]error) chan bool {
	shutdownTimeout := k.getDurationFromConfig("web.shutdown_timeout", configDefaultShutdownTimeoutMs)

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

	errorObj := webKernelError.NewNotFoundHttpError()
	runtimeErrorEvent := webEvent.NewRuntimeError(responseWriter, requestObj, commonKernelError.NewRuntimeError(errorObj, nil))
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

	runtimeError := commonKernelError.NewRuntimeError(recoveredError, trace)

	eventBus := GetRequestEventBus(requestObj)

	runtimeErrorEvent := webEvent.NewRuntimeError(responseWriterObj, requestObj, runtimeError)
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

func (k *Kernel) getDurationFromConfig(key string, defaultInMs int) time.Duration {
	result := k.config.GetDuration(key)
	if 0 >= result {
		result = time.Duration(defaultInMs)
	}

	return time.Millisecond * result
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
		config:                  viper.New(),
		container:               gioc.NewContainer(),
		routes:                  make(map[string]*Route, 0),
		notFoundHandlerEventBus: commonEventBus.NewEventBus(),
		eventsRegistry:          webEventBus.NewDefaultRegistry(),
		applicationEventBus:     commonEventBus.NewEventBus(),
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
		[]string{"services", "web", "cli", "event_listeners"},
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

	kernel.httpServer.ReadHeaderTimeout = kernel.getDurationFromConfig("web.server_read_header_timeout", defaultReadHeaderTimeoutMs)
	kernel.httpServer.ReadTimeout = kernel.getDurationFromConfig("web.server_read_timeout", defaultReadTimeoutMs)
	kernel.httpServer.WriteTimeout = kernel.getDurationFromConfig("web.server_write_timeout", defaultWriteTimeoutMs)
	kernel.httpServer.IdleTimeout = kernel.getDurationFromConfig("web.server_idle_timeout", defaultIdleTimeoutMs)

	return kernel, nil
}

//--------------------

func performRequestTermination(requestObj *http.Request, responseObj response.Response) {
	requestTerminationEvent := webEvent.NewRequestTermination(requestObj, responseObj)
	GetRequestEventBus(requestObj).Dispatch(requestTerminationEvent)
}

func GetRequestEventBus(requestObj *http.Request) commonEventBus.EventBus {
	eventBus := requestObj.Context().Value(requestCtxEventBusKey)
	if nil != eventBus {
		if eventBusTyped, isEventBus := eventBus.(commonEventBus.EventBus); isEventBus {
			return eventBusTyped
		}
	}

	panic("EventBus not set to request object")
}

func RequestContextAppend(requestObj *http.Request, key, val interface{}) {
	newContext := context.WithValue(requestObj.Context(), key, val)
	*requestObj = *requestObj.WithContext(newContext)
}
