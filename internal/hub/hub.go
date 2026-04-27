package hub

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Commander interface {
	Execute(action, containerID string)
}

type Hub struct {
	clients   map[*websocket.Conn]bool
	mu        sync.Mutex
	Broadcast chan interface{}
	commander Commander
}

func NewHub(c Commander) *Hub {
	return &Hub{
		clients:   make(map[*websocket.Conn]bool),
		Broadcast: make(chan interface{}, 100),
		commander: c,
	}
}

func (h *Hub) Run() {
	for msg := range h.Broadcast {
		h.mu.Lock()
		for client := range h.clients {
			if err := client.WriteJSON(msg); err != nil {
				client.Close()
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}
}

func (h *Hub) Register(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	go func() {
		defer func() {
			conn.Close()
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
		}()

		for {
			var cmd struct {
				Action      string `json:"action"`
				ContainerID string `json:"container_id"`
			}

			if err := conn.ReadJSON(&cmd); err != nil {
				break
			}

			if h.commander != nil {
				h.commander.Execute(cmd.Action, cmd.ContainerID)
			}
		}
	}()
}
