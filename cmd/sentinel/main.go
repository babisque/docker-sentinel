package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	fmt.Println("Docker-Sentinel starting the monitoring")

	cpus := runtime.NumCPU()
	goVersion := runtime.Version()

	fmt.Printf("Mechanism: %s | Detected CPUs: %d\n", goVersion, cpus)

	host, _ := os.Hostname()
	fmt.Printf("Running on host: %s\n", host)
}
