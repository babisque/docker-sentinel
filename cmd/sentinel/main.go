package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/babisque/docker-sentinel/internal/analyzer"
	"github.com/babisque/docker-sentinel/internal/docker"
	"github.com/babisque/docker-sentinel/internal/engine"
	"github.com/babisque/docker-sentinel/internal/hub"
	"github.com/babisque/docker-sentinel/internal/server"
	"github.com/babisque/docker-sentinel/internal/store"
	"github.com/babisque/docker-sentinel/pkg/models"
	"github.com/docker/docker/api/types/container"
)

func main() {
	fmt.Println("Docker sentinel is starting now.")

	cli, err := docker.NewClient()
	if err != nil {
		log.Fatalf("Critical error: %v", err)
	}
	defer cli.Close()

	hStore := store.NewHistoryStore()
	statsChan := make(chan models.StatsSnapshot, 100)
	alertChan := make(chan analyzer.Alert, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	commandHandler := func(action, id string) {
		cmdCtx := context.Background()
		switch action {
		case "stop":
			log.Printf("Executing stop command for container %s", id[:12])
			cli.ContainerStop(cmdCtx, id, container.StopOptions{})
		case "restart":
			log.Printf("Executing restart: %s", id[:12])
			cli.ContainerRestart(cmdCtx, id, container.StopOptions{})
		}
	}

	wsHub := hub.NewHub(commandHandler)
	go wsHub.Run()
	go server.Start("localhost:8080", wsHub, cli)

	go func() {
		for s := range statsChan {
			if s.CPUPercentage > 80.0 {
				log.Printf("High CPU usage detected for container %s (%s): %.2f%%", s.ContainerID[:12], s.ContainerName, s.CPUPercentage)
				wsHub.Broadcast <- analyzer.Alert{
					ContainerName: s.ContainerName,
					Level:         "CRITICAL",
					Message:       fmt.Sprintf("High CPU usage detected: %.2f%%", s.CPUPercentage),
					Timestamp:     time.Now(),
				}
			}
			wsHub.Broadcast <- s
		}
	}()

	go func() {
		for alert := range alertChan {
			wsHub.Broadcast <- alert
		}
	}()

	engine := &engine.Engine{
		Cli:       cli,
		StatsChan: statsChan,
		AlertChan: alertChan,
		HStore:    hStore,
	}
	engine.Bootstrap(ctx)

	msgs, errs := docker.ListenEvents(ctx, cli)

	for {
		select {
		case msg := <-msgs:
			containerName := msg.Actor.Attributes["name"]

			switch msg.Action {
			case "start":
				log.Printf("Container started: %s (%s)", containerName, msg.ID[:12])
				engine.StartWorker(ctx, msg.ID, containerName)
			case "die":
				log.Printf("Container stopped: %s (%s)", containerName, msg.ID[:12])
				wsHub.Broadcast <- map[string]interface{}{
					"type":         "lifecycle",
					"action":       "die",
					"container_id": msg.ID,
				}
			}
		case err := <-errs:
			log.Printf("Error listening to Docker events: %v", err)
			return

		case <-stop:
			log.Println("Shutting down gracefully...")
			cancel()

			if err := hStore.ExportJSON("history.json"); err != nil {
				log.Printf("Error exporting history: %v", err)
			}
			return
		}
	}
}
