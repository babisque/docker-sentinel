package models

import "time"

type StatsSnapshot struct {
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	CPUPercentage float64   `json:"cpu_percentage"`
	MemoryUsage   float64   `json:"memory_usage"`
	MemoryLimit   float64   `json:"memory_limit"`
	MemPercent    float64   `json:"memory_percentage"`
	Timestamp     time.Time `json:"timestamp"`
}
