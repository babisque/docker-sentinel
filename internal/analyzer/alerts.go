package analyzer

import (
	"strings"
	"time"
)

type Alert struct {
	ContainerName string
	Level         string
	Message       string
	Timestamp     time.Time
}

func Analyze(containerName string, line string, alertChan chan<- Alert) {
	upperLine := strings.ToUpper(line)

	level := ""

	if strings.Contains(upperLine, "ERROR") {
		level = "ERROR"
	} else if strings.Contains(upperLine, "PANIC") || strings.Contains(upperLine, "FATAL") {
		level = "CRITICAL"
	}

	if level != "" {
		alertChan <- Alert{
			ContainerName: containerName,
			Level:         level,
			Message:       line,
			Timestamp:     time.Now(),
		}
	}
}
