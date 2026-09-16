package websocket

import (
	"log/slog"
	"sync"
)

// HubInterface defines the operations supported by the WebSocket Hub.
type HubInterface interface {
	Register(client *Client)
	Unregister(client *Client)
	RegisterWindow(client *Client, windowNumber int)
	Broadcast(env Envelope)
	BroadcastToWindow(windowNumber int, env Envelope)
	ClientCount() int
	WindowClientCount(windowNumber int) int
	SetWindowRegisterHandler(fn func(client *Client, windowNumber int))
	Shutdown()
}

// Hub maintains the set of active clients and broadcasts messages to clients.
type Hub struct {
	mu               sync.RWMutex
	clients          map[*Client]bool
	windowClients    map[int]map[*Client]bool
	onWindowRegister func(client *Client, windowNumber int)
	closed           bool
}

// NewHub constructs a new thread-safe WebSocket Hub.
func NewHub() *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		windowClients: make(map[int]map[*Client]bool),
	}
}

// SetWindowRegisterHandler registers a callback invoked when a client binds to a window.
func (h *Hub) SetWindowRegisterHandler(fn func(client *Client, windowNumber int)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onWindowRegister = fn
}

// Register adds a client to the hub's active client registry.
func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		client.Close()
		return
	}
	h.clients[client] = true
	wNum := client.WindowNumber()
	if wNum > 0 {
		if h.windowClients[wNum] == nil {
			h.windowClients[wNum] = make(map[*Client]bool)
		}
		h.windowClients[wNum][client] = true
	}
	onRegister := h.onWindowRegister
	h.mu.Unlock()

	slog.Info("websocket client connected",
		"window", wNum,
		"total_clients", h.ClientCount())

	if wNum > 0 && onRegister != nil {
		onRegister(client, wNum)
	}
}

// RegisterWindow associates an existing client with a window number.
func (h *Hub) RegisterWindow(client *Client, windowNumber int) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}

	oldWindow := client.WindowNumber()
	if oldWindow > 0 && h.windowClients[oldWindow] != nil {
		delete(h.windowClients[oldWindow], client)
		if len(h.windowClients[oldWindow]) == 0 {
			delete(h.windowClients, oldWindow)
		}
	}

	client.SetWindowNumber(windowNumber)
	if windowNumber > 0 {
		if h.windowClients[windowNumber] == nil {
			h.windowClients[windowNumber] = make(map[*Client]bool)
		}
		h.windowClients[windowNumber][client] = true
	}
	onRegister := h.onWindowRegister
	h.mu.Unlock()

	slog.Info("websocket client registered window",
		"window", windowNumber,
		"prev_window", oldWindow)

	if windowNumber > 0 && onRegister != nil {
		onRegister(client, windowNumber)
	}
}

// Unregister removes a client from all internal maps.
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		wNum := client.WindowNumber()
		if wNum > 0 && h.windowClients[wNum] != nil {
			delete(h.windowClients[wNum], client)
			if len(h.windowClients[wNum]) == 0 {
				delete(h.windowClients, wNum)
			}
		}
	}
	h.mu.Unlock()

	slog.Info("websocket client disconnected",
		"window", client.WindowNumber(),
		"total_clients", h.ClientCount())
}

// Broadcast dispatches an envelope to every connected client.
func (h *Hub) Broadcast(env Envelope) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		client.SendEnvelope(env)
	}
}

// BroadcastToWindow dispatches an envelope strictly to clients assigned to a specific window.
func (h *Hub) BroadcastToWindow(windowNumber int, env Envelope) {
	h.mu.RLock()
	var clients []*Client
	if m, ok := h.windowClients[windowNumber]; ok {
		clients = make([]*Client, 0, len(m))
		for client := range m {
			clients = append(clients, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range clients {
		client.SendEnvelope(env)
	}
}

// ClientCount returns the total number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// WindowClientCount returns the number of clients attached to a window.
func (h *Hub) WindowClientCount(windowNumber int) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if m, ok := h.windowClients[windowNumber]; ok {
		return len(m)
	}
	return 0
}

// Shutdown gracefully terminates all connected clients.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	h.closed = true
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.clients = make(map[*Client]bool)
	h.windowClients = make(map[int]map[*Client]bool)
	h.mu.Unlock()

	for _, client := range clients {
		client.Close()
	}
	slog.Info("websocket hub shut down cleanly")
}
