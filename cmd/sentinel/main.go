package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/babisque/docker-sentinel/internal/analyzer"
	"github.com/babisque/docker-sentinel/internal/docker"
	"github.com/babisque/docker-sentinel/internal/engine"
	"github.com/babisque/docker-sentinel/internal/hub"
	"github.com/babisque/docker-sentinel/internal/server"
	"github.com/babisque/docker-sentinel/internal/store"
	"github.com/babisque/docker-sentinel/pkg/models"
)

func main() {
	cli, _ := docker.NewClient()
	defer cli.Close()

	hStore := store.NewHistoryStore()
	statsChan := make(chan models.StatsSnapshot, 100)
	alertChan := make(chan analyzer.Alert, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng := &engine.Engine{
		Cli: cli, StatsChan: statsChan, AlertChan: alertChan, HStore: hStore,
	}

	wsHub := hub.NewHub(eng)
	eng.Broadcast = wsHub.Broadcast

	go wsHub.Run()
	go server.Start("localhost:8080", wsHub, cli)

	go syncStatsToHub(statsChan, alertChan, wsHub)

	log.Println("Sentinel engine is running...")
	eng.Run(ctx)
}

func syncStatsToHub(statsChan <-chan models.StatsSnapshot, alertChan <-chan analyzer.Alert, wsHub *hub.Hub) {
	go func() {
		for s := range statsChan {
			if s.CPUPercentage > 80.0 {
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
}
