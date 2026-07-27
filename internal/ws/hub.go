// Package ws is the real-time sync layer: the Go server is the
// authoritative source of truth, and connected clients (GM and players)
// are kept in sync by broadcasting messages through the Hub.
package ws

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

// Hub tracks connected clients and broadcasts messages to all of them.
// The message format/protocol isn't designed yet, so Broadcast currently
// just fans out raw bytes; that will become a typed envelope once the
// core tabletop protocol (map/token/fog state) is defined.
type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]struct{})}
}

// Broadcast sends msg to every connected client.
func (h *Hub) Broadcast(ctx context.Context, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.clients {
		if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
			slog.Warn("ws: broadcast write failed", "error", err)
		}
	}
}

// ServeHTTP upgrades the request to a WebSocket connection, registers it,
// and echoes/broadcasts anything it receives until the client disconnects.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		slog.Warn("ws: accept failed", "error", err)
		return
	}
	defer conn.CloseNow()

	h.register(conn)
	defer h.unregister(conn)

	ctx := r.Context()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return // client disconnected or context canceled
		}
		h.Broadcast(ctx, data)
	}
}

func (h *Hub) register(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = struct{}{}
}

func (h *Hub) unregister(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, conn)
}
