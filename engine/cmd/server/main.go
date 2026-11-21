package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mad-dm/engine/pkg/api"
	"github.com/mad-dm/engine/pkg/downloader"
)

func main() {
	homeDir, _ := os.UserHomeDir()
	defaultDownloads := "./downloads"
	defaultState := ".mad-dm"
	if homeDir != "" {
		defaultDownloads = filepath.Join(homeDir, "Downloads")
		defaultState = filepath.Join(homeDir, ".mad-dm")
	}

	port := flag.Int("port", 17890, "Port for the MAD-DM API server")
	defaultDir := flag.String("dir", defaultDownloads, "Default download folder")
	maxConcurrent := flag.Int("max", 3, "Maximum concurrent downloads in queue")
	stateDir := flag.String("state", defaultState, "Directory for storing tasks and state")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("   ⚡ MAD-DM Engine Server (Go Daemon)")
	fmt.Println("   Accelerated Multi-part Downloader")
	fmt.Println("==================================================")

	manager, err := downloader.NewManager(downloader.ManagerConfig{
		MaxConcurrent: *maxConcurrent,
		DefaultDir:    *defaultDir,
		StateDir:      *stateDir,
	})
	if err != nil {
		fmt.Printf("❌ Failed to initialize manager: %v\n", err)
		os.Exit(1)
	}

	server := api.NewServer(manager, *port)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n🛑 Stopping server and pausing active tasks...")
		for _, t := range manager.ListTasks() {
			if t.Status == downloader.StatusDownloading {
				_ = manager.PauseTask(t.ID)
			}
		}
		os.Exit(0)
	}()

	if err := server.Start(); err != nil {
		fmt.Printf("❌ Server error: %v\n", err)
		os.Exit(1)
	}
}
