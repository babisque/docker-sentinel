package hub

import (
	"context"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
)

type Hub struct {
	clients map[*websocket.Conn]bool
	mu      sync.Mutex

	Broadcast chan interface{}
}

func NewHub() *Hub {
	return &Hub{
		clients:   make(map[*websocket.Conn]bool),
		Broadcast: make(chan interface{}, 100),
	}
}

func (h *Hub) Run() {
	for {
		msg := <-h.Broadcast
		h.mu.Lock()
		for client := range h.clients {
			err := client.WriteJSON(msg)
			if err != nil {
				client.Close()
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}
}

func (h *Hub) Register(conn *websocket.Conn, cli *client.Client) {
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
			var cmd map[string]string
			if err := conn.ReadJSON(&cmd); err != nil {
				break
			}

			if action, ok := cmd["action"]; ok {
				id := cmd["container_id"]
				ctx := context.Background()

				switch action {
				case "stop":
					cli.ContainerStop(ctx, id, container.StopOptions{})
				case "restart":
					cli.ContainerRestart(ctx, id, container.StopOptions{})
				}
			}
		}
	}()
}
