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
	"github.com/babisque/docker-sentinel/internal/store"
)

func main() {
	fmt.Println("Docker-Sentinel starting up...")

	cli, err := docker.NewClient()
	if err != nil {
		log.Fatalf("Critic error: %v", err)
	}
	defer cli.Close()

	hStore := store.NewHistoryStore()

	alertChan := make(chan analyzer.Alert, 100)

	go func() {
		for alert := range alertChan {
			fmt.Printf("[%s] ALERT: %s - %s\n", alert.Timestamp.Format(time.RFC3339), alert.Level, alert.Message)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	msgs, errs := docker.ListenEvents(ctx, cli)

	for {
		select {
		case msg := <-msgs:
			containerName := msg.Actor.Attributes["name"]
			image := msg.From

			switch msg.Action {
			case "start":
				log.Printf("Container started: %s (%s)", containerName, image)

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
				time.Sleep(500 * time.Millisecond)
				if h, ok := hStore.Get(msg.ID); ok {
					log.Printf("History for %s: %d log lines, stopped at %s", h.Name, len(h.Logs), h.StoppedAt.Format(time.RFC3339))
				}
			}
		case err := <-errs:
			log.Printf("Error listening to Docker events: %v", err)
			return

		case <-stop:
			log.Println("Shutting down Docker-Sentinel...")

			err := hStore.ExportJSON("history.json")
			if err != nil {
				log.Printf("Error exporting history: %v", err)
			} else {
				log.Println("History exported to history.json")
			}
			return
		}
	}
}
