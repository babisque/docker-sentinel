package engine

import (
	"bufio"
	"context"
	"log"
	"time"

	"github.com/babisque/docker-sentinel/internal/analyzer"
	"github.com/babisque/docker-sentinel/internal/docker"
	"github.com/babisque/docker-sentinel/internal/store"
	"github.com/babisque/docker-sentinel/pkg/models"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Engine struct {
	Cli       *client.Client
	StatsChan chan models.StatsSnapshot
	AlertChan chan analyzer.Alert
	HStore    *store.HistoryStore
	Broadcast chan<- interface{}
}

func (e *Engine) Execute(action, id string) {
	ctx := context.Background()
	switch action {
	case "stop":
		e.Cli.ContainerStop(ctx, id, container.StopOptions{})
	case "restart":
		e.Cli.ContainerRestart(ctx, id, container.StopOptions{})
	}
}

func (e *Engine) Run(ctx context.Context) {
	e.Bootstrap(ctx)

	msgs, err := docker.ListenEvents(ctx, e.Cli)
	for {
		select {
		case msg := <-msgs:
			name := msg.Actor.Attributes["name"]
			switch msg.Action {
			case "start":
				e.StartWorker(ctx, msg.ID, name)
			case "die":
				e.Broadcast <- map[string]interface{}{
					"type": "lifecycle", "action": "die", "container_id": msg.ID,
				}
			}
		case err := <-err:
			log.Printf("Error listening to Docker events: %v", err)
			return
		case <-ctx.Done():
			return
		}
	}
}

func (e *Engine) Bootstrap(ctx context.Context) {
	containers, _ := e.Cli.ContainerList(ctx, types.ContainerListOptions{})
	for _, c := range containers {
		name := "unknown"
		if len(c.Names) > 0 {
			name = c.Names[0]
		}
		e.StartWorker(ctx, c.ID, name)
	}
}

func (e *Engine) StartWorker(ctx context.Context, id, name string) {
	go docker.StreamStats(ctx, e.Cli, id, name, e.StatsChan)

	go func() {
		stream, err := docker.GetContainerLogs(ctx, e.Cli, id)
		if err != nil {
			log.Printf("Error attaching to logs for %s: %v", name, err)
			return
		}
		defer stream.Close()

		var capturedLogs []string
		scanner := bufio.NewScanner(stream)

		for scanner.Scan() {
			line := scanner.Text()
			capturedLogs = append(capturedLogs, line)

			analyzer.Analyze(name, line, e.AlertChan)
		}

		e.HStore.Save(id, &store.ContainerHistory{
			Name:      name,
			Logs:      capturedLogs,
			StoppedAt: time.Now(),
		})
	}()
}
