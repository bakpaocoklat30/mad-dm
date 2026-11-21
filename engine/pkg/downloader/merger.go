package downloader

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MergeFiles concatenates multiple media files into one using FFmpeg
func MergeFiles(inputPaths []string, outputPath string, format string) error {
	if len(inputPaths) == 0 {
		return fmt.Errorf("no input files provided to merge")
	}
	if len(inputPaths) == 1 {
		return copySingleFile(inputPaths[0], outputPath)
	}

	ffmpegPath := getFfmpegPath()
	if ffmpegPath == "" {
		return fmt.Errorf("ffmpeg binary not found, unable to merge media files")
	}

	// Create temporary concat demuxer text file
	dir := filepath.Dir(outputPath)
	_ = os.MkdirAll(dir, 0755)

	listFile, err := os.CreateTemp(dir, "mad_dm_concat_*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp concat manifest: %w", err)
	}
	listPath := listFile.Name()
	defer os.Remove(listPath)

	for _, p := range inputPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			absPath = p
		}
		// Escape single quotes for ffmpeg concat demuxer format
		escaped := strings.ReplaceAll(absPath, "'", "'\\''")
		_, _ = listFile.WriteString(fmt.Sprintf("file '%s'\n", escaped))
	}
	_ = listFile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	// 1. Fast attempt: Stream Copy (Zero-reencode, maximum speed)
	fastArgs := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		outputPath,
	}

	cmdFast := exec.CommandContext(ctx, ffmpegPath, fastArgs...)
	outFast, errFast := cmdFast.CombinedOutput()
	if errFast == nil {
		if fi, sErr := os.Stat(outputPath); sErr == nil && fi.Size() > 0 {
			return nil
		}
	}

	// 2. Fallback: Re-encode if files have different codecs/resolutions
	_ = os.Remove(outputPath)
	fmt.Printf("[MAD-DM Merger] Stream copy concat failed, executing re-encode fallback: %s\n", string(outFast))

	var fallbackArgs []string
	switch strings.ToLower(format) {
	case "mp3":
		fallbackArgs = []string{
			"-y",
			"-f", "concat",
			"-safe", "0",
			"-i", listPath,
			"-acodec", "libmp3lame",
			"-b:a", "320k",
			outputPath,
		}
	case "flac":
		fallbackArgs = []string{
			"-y",
			"-f", "concat",
			"-safe", "0",
			"-i", listPath,
			"-acodec", "flac",
			outputPath,
		}
	default: // mp4 video
		fallbackArgs = []string{
			"-y",
			"-f", "concat",
			"-safe", "0",
			"-i", listPath,
			"-c:v", "libx264",
			"-preset", "fast",
			"-crf", "22",
			"-c:a", "aac",
			"-b:a", "192k",
			outputPath,
		}
	}

	cmdFallback := exec.CommandContext(ctx, ffmpegPath, fallbackArgs...)
	outFallback, errFallback := cmdFallback.CombinedOutput()
	if errFallback != nil {
		return fmt.Errorf("ffmpeg merge failed: %w (output: %s)", errFallback, string(outFallback))
	}

	return nil
}

func copySingleFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
