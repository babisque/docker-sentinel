package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

func NewClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return cli, nil
}

func ListenEvents(ctx context.Context, cli *client.Client) (<-chan events.Message, <-chan error) {
	return cli.Events(ctx, types.EventsOptions{})
}

func GetContainerLogs(ctx context.Context, cli *client.Client, containerId string) (io.ReadCloser, error) {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	}

	return cli.ContainerLogs(ctx, containerId, options)
}
