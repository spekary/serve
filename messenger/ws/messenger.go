package ws

import (
	"fmt"
	"net/http"
	"path"

	"github.com/goradd/html5tag"
	"github.com/goradd/serve/config"
	http2 "github.com/goradd/serve/http"
	_ "github.com/goradd/serve/messenger/ws/assets"
	"github.com/goradd/serve/page"
)

const logModule = "websocket"

type contextKey struct{}

type WsMessenger struct {
	hub *WebSocketHub
}

const IdFormValue = "id"

func (m *WsMessenger) Start() *WebSocketHub {
	m.hub = NewWebSocketHub()
	go m.hub.run()
	return m.hub
}

func (m *WsMessenger) JavascriptInit() string {
	return fmt.Sprintf("goradd.initMessagingClient(%q);\n", http2.MakeLocalPath(config.WebsocketMessengerPath))
}

func (m *WsMessenger) JavascriptFiles() map[string]html5tag.Attributes {
	ret := make(map[string]html5tag.Attributes)
	p := path.Join(config.AssetPath, "messenger", "js", "goradd-ws.js")
	ret[p] = nil
	return ret
}

func (m *WsMessenger) Send(channel string, message string) {
	if m.hub != nil {
		m.hub.send <- clientMessage{channel, message}
	}
}

// WebSocketHandler handles web socket requests to send messages to clients.
// It gets the client id from a form value sent with the request.
func (m *WsMessenger) WebSocketHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := r.FormValue(IdFormValue)
		// clientID is the page state
		if !page.HasPage(clientID) {
			// authorize the client id, making sure the page exists in the page cache.
			return
		}
		serveWs(m.hub, w, r, clientID)
	})
}
