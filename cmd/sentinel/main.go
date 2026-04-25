package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/babisque/docker-sentinel/internal/analyzer"
	"github.com/babisque/docker-sentinel/internal/docker"
	"github.com/babisque/docker-sentinel/internal/hub"
	"github.com/babisque/docker-sentinel/internal/server"
	"github.com/babisque/docker-sentinel/internal/store"
	"github.com/babisque/docker-sentinel/pkg/models"
	"github.com/docker/docker/api/types"
)

func main() {
	fmt.Println("Docker-Sentinel starting up...")

	statsChan := make(chan models.StatsSnapshot, 100)
	alertChan := make(chan analyzer.Alert, 100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wsHub := hub.NewHub()
	go wsHub.Run()

	go func() {
		for s := range statsChan {
			if s.CPUPercentage > 80.0 {
				log.Printf("PEAK DETECTED: %s at %.2f%%", s.ContainerName, s.CPUPercentage)

				wsHub.Broadcast <- analyzer.Alert{
					ContainerName: s.ContainerName,
					Level:         "CRITICAL",
					Message:       fmt.Sprintf("High CPU Usage: %.2f%%", s.CPUPercentage),
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

	cli, err := docker.NewClient()
	if err != nil {
		log.Fatalf("Critic error: %v", err)
	}
	defer cli.Close()

	hStore := store.NewHistoryStore()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	msgs, errs := docker.ListenEvents(ctx, cli)

	go server.Start("localhost:8080", wsHub)

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		log.Printf("Error listing containers: %v", err)
	}

	for _, c := range containers {
		name := "unknown"
		if len(c.Names) > 0 {
			name = c.Names[0]
		}

		log.Printf("Syncing existing container: %s", name)
		go docker.StreamStats(ctx, cli, c.ID, name, statsChan)

		go func(id, n string) {
			stram, err := docker.GetContainerLogs(ctx, cli, id)
			if err != nil {
				return
			}
			defer stram.Close()
		}(c.ID, name)
	}

	for {
		select {
		case msg := <-msgs:
			containerName := msg.Actor.Attributes["name"]
			image := msg.From

			switch msg.Action {
			case "start":
				log.Printf("Container started: %s (%s)", containerName, image)
				go docker.StreamStats(ctx, cli, msg.ID, containerName, statsChan)

				go func(id, name, img string) {
					stream, err := docker.GetContainerLogs(ctx, cli, id)
					if err != nil {
						log.Printf("Error getting logs for %s: %v", name, err)
						return
					}
					defer stream.Close()

					var capturedLogs []string
					scanner := bufio.NewScanner(stream)

					for scanner.Scan() {
						line := scanner.Text()
						capturedLogs = append(capturedLogs, line)
						analyzer.Analyze(name, line, alertChan)
					}

					hStore.Save(id, &store.ContainerHistory{
						Name:      name,
						Image:     img,
						Logs:      capturedLogs,
						StoppedAt: time.Now(),
					})
				}(msg.ID, containerName, image)

			case "die":
				log.Printf("Container stopped: %s (%s)", containerName, image)
				wsHub.Broadcast <- map[string]interface{}{
					"type":         "lifecycle",
					"action":       "die",
					"container_id": msg.ID,
				}

				time.Sleep(500 * time.Millisecond)
				if h, ok := hStore.Get(msg.ID); ok {
					log.Printf("History for %s: %d log lines saved.", h.Name, len(h.Logs))
				}
			}
		case err := <-errs:
			log.Printf("Error listening to Docker events: %v", err)
			return

		case <-stop:
			log.Println("Shutting down Docker-Sentinel...")
			cancel()

			err := hStore.ExportJSON("history.json")
			if err != nil {
				log.Printf("Error exporting history: %v", err)
			}
			return
		}
	}
}
