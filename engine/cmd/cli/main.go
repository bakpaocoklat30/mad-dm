package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mad-dm/engine/pkg/downloader"
)

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(sec int64) string {
	if sec <= 0 {
		return "--:--"
	}
	m := sec / 60
	s := sec % 60
	if m >= 60 {
		h := m / 60
		m = m % 60
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func renderBar(percent float64, width int) string {
	filled := int(percent * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	return "[" + strings.Repeat("=", filled) + strings.Repeat(" ", empty) + "]"
}

func main() {
	homeDir, _ := os.UserHomeDir()
	defaultDownloads := "./downloads"
	if homeDir != "" {
		defaultDownloads = filepath.Join(homeDir, "Downloads")
	}

	connections := flag.Int("c", 8, "Number of concurrent download connections (default 8)")
	outputDir := flag.String("d", defaultDownloads, "Output directory (default ~/Downloads)")
	customFilename := flag.String("o", "", "Custom output filename")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("MAD-DM (Multi-thread Accelerated Downloader)")
		fmt.Println("Usage: mad-dm [options] <URL>")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	targetURL := args[0]
	fmt.Printf("\n🚀 MAD-DM: Connecting to %s ...\n", targetURL)

	task := downloader.NewTask("cli-task", targetURL, *outputDir, *connections)
	if *customFilename != "" {
		task.Filename = *customFilename
	}

	if err := task.Probe(); err != nil {
		fmt.Printf("❌ Failed to probe URL: %v\n", err)
		os.Exit(1)
	}

	if *outputDir == defaultDownloads || strings.EqualFold(filepath.Base(*outputDir), "downloads") {
		task.Directory = downloader.CategorizedDir(*outputDir, task.Filename, "")
		task.Category = string(downloader.ResolveCategory(task.Filename, ""))
		_ = os.MkdirAll(task.Directory, 0755)
	}

	fmt.Printf("📦 File: %s\n", task.Filename)
	if task.Category != "" {
		fmt.Printf("🏷️ Category: %s\n", task.Category)
	}
	fmt.Printf("📏 Size: %s (%d bytes)\n", formatBytes(task.TotalSize), task.TotalSize)
	fmt.Printf("⚡ Resumable / Multi-part: %v\n", task.Resumable)
	fmt.Printf("🧵 Connections: %d\n", task.Connections)
	fmt.Printf("📁 Destination: %s\n\n", task.FilePath())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n⏸  Pausing download and saving state...")
		task.Pause()
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	if err := task.Start(); err != nil {
		fmt.Printf("❌ Download error: %v\n", err)
		os.Exit(1)
	}

	// Progress loop
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if task.Status == downloader.StatusCompleted {
			fmt.Printf("\r\033[K%s 100%% | %s/%s | %s/s | Done! ✨\n",
				renderBar(1.0, 30),
				formatBytes(task.TotalSize),
				formatBytes(task.TotalSize),
				formatBytes(task.Speed),
			)
			fmt.Printf("\n✅ Download completed successfully! Saved to: %s\n", task.FilePath())
			break
		}

		if task.Status == downloader.StatusError {
			fmt.Printf("\n❌ Download failed: %s\n", task.Error)
			os.Exit(1)
		}

		if task.Status == downloader.StatusPaused {
			fmt.Println("\n⏸  Download paused.")
			break
		}

		var percent float64
		if task.TotalSize > 0 {
			percent = float64(task.Downloaded) / float64(task.TotalSize)
		}

		fmt.Printf("\r\033[K%s %5.1f%% | %s/%s | ⚡ %s/s | ETA: %s",
			renderBar(percent, 30),
			percent*100,
			formatBytes(task.Downloaded),
			formatBytes(task.TotalSize),
			formatBytes(task.Speed),
			formatDuration(task.ETA),
		)
	}
}
