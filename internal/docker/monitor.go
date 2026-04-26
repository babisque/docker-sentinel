package docker

import (
	"bufio"
	"context"
	"time"

	"github.com/babisque/docker-sentinel/internal/analyzer"
	"github.com/babisque/docker-sentinel/internal/store"
	"github.com/babisque/docker-sentinel/pkg/models"
	"github.com/docker/docker/client"
)

func StartMonitoring(ctx context.Context, cli *client.Client, id, name string, statsChan chan<- models.StatsSnapshot, alertChan chan<- analyzer.Alert, hStore *store.HistoryStore) {
	go StreamStats(ctx, cli, id, name, statsChan)

	go func() {
		stream, err := GetContainerLogs(ctx, cli, id)
		if err != nil {
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
			StoppedAt: time.Now(),
			Logs:      capturedLogs,
		})
	}()
}
