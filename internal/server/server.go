package server

import (
	"log"
	"net/http"

	"github.com/babisque/docker-sentinel/internal/hub"
	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func Start(address string, wsHub *hub.Hub, cli *client.Client) {
	http.Handle("/", http.FileServer(http.Dir("./web")))

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Error on upgrade: %v", err)
			return
		}

		wsHub.Register(conn, cli)
	})

	log.Printf("Server starting on %s", address)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
