package downloader

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MergeFiles concatenates multiple media files into one using FFmpeg
func MergeFiles(inputPaths []string, outputPath string, format string, resolution string) error {
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

	dir := filepath.Dir(outputPath)
	_ = os.MkdirAll(dir, 0755)

	fmtLower := strings.ToLower(strings.TrimPrefix(format, "."))

	if fmtLower == "mp3" || fmtLower == "flac" {
		return mergeAudio(inputPaths, outputPath, fmtLower, ffmpegPath)
	}

	return mergeVideo(inputPaths, outputPath, resolution, ffmpegPath)
}

func mergeAudio(inputPaths []string, outputPath, format, ffmpegPath string) error {
	dir := filepath.Dir(outputPath)
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
		escaped := strings.ReplaceAll(absPath, "'", "'\\''")
		_, _ = listFile.WriteString(fmt.Sprintf("file '%s'\n", escaped))
	}
	_ = listFile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	// 1. Try stream copy concat
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
		verifyCmd := exec.CommandContext(ctx, ffmpegPath, "-v", "error", "-i", outputPath, "-f", "null", "-")
		if verifyOut, vErr := verifyCmd.CombinedOutput(); vErr == nil && len(strings.TrimSpace(string(verifyOut))) == 0 {
			if fi, sErr := os.Stat(outputPath); sErr == nil && fi.Size() > 0 {
				return nil
			}
		}
	}

	_ = os.Remove(outputPath)
	fmt.Printf("[MAD-DM Merger] Audio stream copy concat had issues (%v, %s), executing re-encode...\n", errFast, string(outFast))

	var fallbackArgs []string
	if format == "flac" {
		fallbackArgs = []string{
			"-y",
			"-f", "concat",
			"-safe", "0",
			"-i", listPath,
			"-acodec", "flac",
			outputPath,
		}
	} else { // mp3
		fallbackArgs = []string{
			"-y",
			"-f", "concat",
			"-safe", "0",
			"-i", listPath,
			"-acodec", "libmp3lame",
			"-b:a", "320k",
			outputPath,
		}
	}

	cmdFallback := exec.CommandContext(ctx, ffmpegPath, fallbackArgs...)
	outFallback, errFallback := cmdFallback.CombinedOutput()
	if errFallback != nil {
		return fmt.Errorf("audio merge failed: %w (output: %s)", errFallback, string(outFallback))
	}

	return nil
}

// probeDimensions extracts width and height from video file using ffmpeg
func probeDimensions(ffmpegPath, inputPath string) (int, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath, "-i", inputPath)
	out, _ := cmd.CombinedOutput()

	re := regexp.MustCompile(`([0-9]{3,5})x([0-9]{3,5})`)
	match := re.FindStringSubmatch(string(out))
	if len(match) == 3 {
		w, _ := strconv.Atoi(match[1])
		h, _ := strconv.Atoi(match[2])
		if w > 0 && h > 0 {
			return w, h
		}
	}
	return 1920, 1080
}

func mergeVideo(inputPaths []string, outputPath, resolution, ffmpegPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()

	// Determine target dimensions
	targetW, targetH := 1920, 1080
	res := strings.ToLower(strings.TrimSpace(resolution))
	switch {
	case strings.Contains(res, "2160") || strings.Contains(res, "4k"):
		targetW, targetH = 3840, 2160
	case strings.Contains(res, "1440") || strings.Contains(res, "2k"):
		targetW, targetH = 2560, 1440
	case strings.Contains(res, "1080"):
		targetW, targetH = 1920, 1080
	case strings.Contains(res, "720"):
		targetW, targetH = 1280, 720
	case strings.Contains(res, "480"):
		targetW, targetH = 854, 480
	case strings.Contains(res, "360"):
		targetW, targetH = 640, 360
	default:
		// Auto: pick max resolution among input videos so no quality is lost
		maxW, maxH := 0, 0
		for _, p := range inputPaths {
			w, h := probeDimensions(ffmpegPath, p)
			if w > maxW {
				maxW = w
				maxH = h
			}
		}
		if maxW > 0 && maxH > 0 {
			targetW = maxW
			targetH = maxH
		}
	}

	// Ensure dimensions are even numbers (required by h264)
	if targetW%2 != 0 {
		targetW++
	}
	if targetH%2 != 0 {
		targetH++
	}

	// First: If all inputs have identical resolution, try concat demuxer
	allIdentical := true
	firstW, firstH := 0, 0
	for i, p := range inputPaths {
		w, h := probeDimensions(ffmpegPath, p)
		if i == 0 {
			firstW, firstH = w, h
		} else if w != firstW || h != firstH {
			allIdentical = false
			break
		}
	}

	if allIdentical && len(inputPaths) <= 30 {
		dir := filepath.Dir(outputPath)
		listFile, err := os.CreateTemp(dir, "mad_dm_concat_video_*.txt")
		if err == nil {
			listPath := listFile.Name()
			for _, p := range inputPaths {
				absPath, aErr := filepath.Abs(p)
				if aErr != nil {
					absPath = p
				}
				escaped := strings.ReplaceAll(absPath, "'", "'\\''")
				_, _ = listFile.WriteString(fmt.Sprintf("file '%s'\n", escaped))
			}
			_ = listFile.Close()

			fastArgs := []string{
				"-y",
				"-f", "concat",
				"-safe", "0",
				"-i", listPath,
				"-c", "copy",
				"-movflags", "+faststart",
				outputPath,
			}
			cmdFast := exec.CommandContext(ctx, ffmpegPath, fastArgs...)
			outFast, errFast := cmdFast.CombinedOutput()
			_ = os.Remove(listPath)

			if errFast == nil {
				// CRITICAL CHECK: Verify with -f null that timestamps are actually valid!
				verifyCmd := exec.CommandContext(ctx, ffmpegPath, "-v", "error", "-i", outputPath, "-f", "null", "-")
				verifyOut, vErr := verifyCmd.CombinedOutput()
				if vErr == nil && len(strings.TrimSpace(string(verifyOut))) == 0 {
					if fi, sErr := os.Stat(outputPath); sErr == nil && fi.Size() > 0 {
						return nil // Stream copy was truly lossless and uncorrupted!
					}
				}
				fmt.Printf("[MAD-DM Merger] Video stream copy had timestamp or codec mismatches (%s). Falling back to robust concat filter...\n", string(verifyOut))
			} else {
				fmt.Printf("[MAD-DM Merger] Video stream copy failed: %s\n", string(outFast))
			}
			_ = os.Remove(outputPath)
		}
	}

	// Robust Filter Complex Concat: Normalizes SAR, DAR, resolution, FPS, and audio channels
	fmt.Printf("[MAD-DM Merger] Normalizing %d video streams to %dx%d (H.264/AAC with faststart)...\n", len(inputPaths), targetW, targetH)

	args := []string{"-y"}
	for _, p := range inputPaths {
		args = append(args, "-i", p)
	}

	var filterParts []string
	var concatInputs string

	for i := range inputPaths {
		vLabel := fmt.Sprintf("v%d", i)
		aLabel := fmt.Sprintf("a%d", i)

		// Scale with aspect ratio preserved, pad with black bars if necessary, SAR 1, FPS 30
		filterParts = append(filterParts, fmt.Sprintf(
			"[%d:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=30[%s]",
			i, targetW, targetH, targetW, targetH, vLabel,
		))

		// Standardize audio stream to 44.1kHz stereo
		filterParts = append(filterParts, fmt.Sprintf(
			"[%d:a]aformat=sample_rates=44100:channel_layouts=stereo[%s]",
			i, aLabel,
		))

		concatInputs += fmt.Sprintf("[%s][%s]", vLabel, aLabel)
	}

	filterParts = append(filterParts, fmt.Sprintf("%sconcat=n=%d:v=1:a=1[outv][outa]", concatInputs, len(inputPaths)))
	filterComplex := strings.Join(filterParts, "; ")

	args = append(args,
		"-filter_complex", filterComplex,
		"-map", "[outv]",
		"-map", "[outa]",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "22",
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		outputPath,
	)

	cmdRobust := exec.CommandContext(ctx, ffmpegPath, args...)
	outRobust, errRobust := cmdRobust.CombinedOutput()
	if errRobust != nil {
		return fmt.Errorf("video merge failed: %w (output: %s)", errRobust, string(outRobust))
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
