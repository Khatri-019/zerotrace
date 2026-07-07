package api

import (
	"net/http"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func SetupWebSocket(r *mux.Router, log *zap.Logger) {
	r.HandleFunc("/ws/traces", func(w http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			log.Error("websocket upgrade failed", zap.Error(err))
			return
		}
		defer conn.Close()
		
		// Wait for closing
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	})
}
