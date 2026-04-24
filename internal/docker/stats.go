package docker

import (
	"context"
	"encoding/json"

	"github.com/babisque/docker-sentinel/pkg/models"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func StreamStats(ctx context.Context, cli *client.Client, id, name string, statsChan chan<- models.StatsSnapshot) {
	resp, err := cli.ContainerStats(ctx, id, true)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	var v types.StatsJSON

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := decoder.Decode(&v); err != nil {
				return
			}

			cpuPercent := calculateCPUPercent(&v)
			memUsage := float64(v.MemoryStats.Usage) / 1024 / 1024
			memLimit := float64(v.MemoryStats.Limit) / 1024 / 1024

			statsChan <- models.StatsSnapshot{
				ContainerID:   id,
				ContainerName: name,
				CPUPercentage: cpuPercent,
				MemoryUsage:   memUsage,
				MemoryLimit:   memLimit,
				MemPercent:    (memUsage / memLimit) * 100,
				Timestamp:     v.Read,
			}
		}
	}
}

func calculateCPUPercent(v *types.StatsJSON) float64 {
	var (
		cpuPercent  = 0.0
		cpuDelta    = float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta = float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)
		onlineCPUs  = float64(v.CPUStats.OnlineCPUs)
	)

	if onlineCPUs == 0 {
		onlineCPUs = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
	}

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	return cpuPercent
}
