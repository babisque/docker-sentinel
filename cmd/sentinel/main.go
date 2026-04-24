package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/babisque/docker-sentinel/internal/docker"
)

func main() {
	fmt.Println("Docker-Sentinel starting up...")

	cli, err := docker.NewClient()
	if err != nil {
		log.Fatalf("Critic error: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	msgs, errs := docker.ListenEvents(ctx, cli)

	for {
		select {
		case msg := <-msgs:
			fmt.Printf("Event: %s | Container: %s | Image: %s\n", msg.Action, msg.Actor.Attributes["name"], msg.From)

		case err := <-errs:
			log.Printf("Stream error: %v", err)
			return

		case <-stop:
			fmt.Println("Shutting down gracefully...")
			return
		}
	}
}
