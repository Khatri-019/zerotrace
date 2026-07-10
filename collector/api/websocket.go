package api

import (
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	proto "github.com/zerotrace/zerotrace/proto"
)

// Hub manages a set of active WebSocket connections and broadcasts
// new trace data to all of them.
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
	log     *zap.Logger
}

// NewHub creates an empty Hub.
func NewHub(log *zap.Logger) *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]struct{}),
		log:     log,
	}
}

// Broadcast sends a JSON-serialisable message to all connected clients.
// Slow or dead clients are removed.
func (h *Hub) Broadcast(msg interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var dead []*websocket.Conn
	for conn := range h.clients {
		if err := conn.WriteJSON(msg); err != nil {
			h.log.Debug("websocket write failed, removing client", zap.Error(err))
			dead = append(dead, conn)
		}
	}
	for _, conn := range dead {
		conn.Close()
		delete(h.clients, conn)
	}
}

// BroadcastSpans broadcasts a batch of spans to live-tail subscribers.
func (h *Hub) BroadcastSpans(spans []*proto.Span) {
	if len(spans) == 0 {
		return
	}
	h.Broadcast(spans)
}

// add registers a new connection.
func (h *Hub) add(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()
	h.log.Debug("websocket client connected", zap.Int("total", len(h.clients)))
}

// remove unregisters a connection.
func (h *Hub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	h.log.Debug("websocket client disconnected", zap.Int("total", len(h.clients)))
}

// ---------------------------------------------------------------------------
// HTTP handler
// ---------------------------------------------------------------------------

var upgrader = websocket.Upgrader{
	// Allow all origins (no auth requirement per PRD)
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
}

// SetupWebSocket registers the /ws/traces endpoint on the router.
func SetupWebSocket(r *mux.Router, hub *Hub, log *zap.Logger) {
	r.HandleFunc("/ws/traces", func(w http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			log.Error("websocket upgrade failed", zap.Error(err))
			return
		}
		hub.add(conn)
		defer hub.remove(conn)

		// Keep the connection alive by reading (and discarding) client messages
		// until the client disconnects.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
}
