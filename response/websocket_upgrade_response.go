package response

import (
	"github.com/gorilla/websocket"
	"net/http"
)

type WebSocketController func(*websocket.Conn)

type WebsocketUpgradeResponse struct {
	*BytesResponseWriter
	upgrader   *websocket.Upgrader
	controller WebSocketController
}

func (r *WebsocketUpgradeResponse) UpgradeToWebsocket(request *http.Request, responseWriter http.ResponseWriter) {
	wsConnection, wsUpgradeError := r.upgrader.Upgrade(responseWriter, request, nil)
	if wsUpgradeError != nil {
		errorText := "WS upgrade error: " + wsUpgradeError.Error()
		panic("WS upgrade error: " + errorText)
		//logger.Critical(errorText, nil)
		//
		//r := kernelResponse.NewBytesResponseWriter()
		//r.Write([]byte(errorText))
		//
		//return r
	}
	//defer func() {
	//	wsCloseError := wsConnection.Close()
	//	if nil != wsCloseError {
	//		logger.Critical("WS close error: "+wsCloseError.Error(), nil)
	//	}
	//}()

	go r.controller(wsConnection)
}

//--------------------

func NewWebsocketUpgradeResponse(upgrader *websocket.Upgrader, controller WebSocketController) *WebsocketUpgradeResponse {
	result := &WebsocketUpgradeResponse{
		BytesResponseWriter: NewBytesResponseWriter(),
		upgrader:            upgrader,
		controller:          controller,
	}

	return result
}
