package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Status string

const (
	StatusQueued      Status = "queued"
	StatusDownloading Status = "downloading"
	StatusPaused      Status = "paused"
	StatusCompleted   Status = "completed"
	StatusError       Status = "error"
	StatusCancelled   Status = "cancelled"
)

type Chunk struct {
	Index      int   `json:"index"`
	Start      int64 `json:"start"`
	End        int64 `json:"end"`
	Downloaded int64 `json:"downloaded"`
	Total      int64 `json:"total"`
	Completed  bool  `json:"completed"`
}

type Task struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Filename    string    `json:"filename"`
	Directory   string    `json:"directory"`
	TotalSize   int64     `json:"total_size"`
	Downloaded  int64     `json:"downloaded"`
	Resumable   bool      `json:"resumable"`
	Connections int       `json:"connections"`
	Status      Status    `json:"status"`
	Speed       int64     `json:"speed"` // bytes per second
	ETA         int64     `json:"eta"`   // seconds left
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Chunks      []*Chunk  `json:"chunks"`
	IsStreaming bool      `json:"is_streaming"`
	Format      string    `json:"format,omitempty"`     // "mp4", "mp3", "flac"
	Resolution  string    `json:"resolution,omitempty"` // "best", "1080p", "720p", "480p", "360p"
	IsMerged    bool      `json:"is_merged,omitempty"`
	Category    string    `json:"category,omitempty"` // "Audio", "Documents", "Program", "Archive", "Video"

	mu         sync.RWMutex
	cancelFunc context.CancelFunc
	ctx        context.Context
	file       *os.File
	client     *http.Client
	stateSaver func()
}

func NewTask(id, rawURL, dir string, connections int) *Task {
	if connections <= 0 {
		connections = 8
	}
	var initialFilename string
	if u, err := url.Parse(rawURL); err == nil && u.Path != "" {
		cleanPath := u.Path
		if idx := strings.IndexAny(cleanPath, "?#"); idx != -1 {
			cleanPath = cleanPath[:idx]
		}
		base := filepath.Base(cleanPath)
		if base != "" && base != "." && base != "/" {
			initialFilename = base
		}
	}
	initialCat := string(ResolveCategory(initialFilename, ""))
	return &Task{
		ID:          id,
		URL:         rawURL,
		Filename:    initialFilename,
		Directory:   dir,
		Connections: connections,
		Status:      StatusQueued,
		Category:    initialCat,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		client: &http.Client{
			Timeout: 0,
		},
	}
}

func (t *Task) FilePath() string {
	return filepath.Join(t.Directory, t.Filename)
}

func (t *Task) SetFormat(format string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	f := strings.ToLower(strings.TrimSpace(format))
	if f == "mp3" || f == "flac" || f == "mp4" {
		t.Format = f
	} else if IsStreamingSite(t.URL) {
		t.Format = "mp4"
	} else {
		t.Format = ""
	}
}

func (t *Task) SetResolution(resolution string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	res := strings.ToLower(strings.TrimSpace(resolution))
	if res == "2160p" || res == "1440p" || res == "1080p" || res == "720p" || res == "480p" || res == "360p" {
		t.Resolution = res
	} else {
		t.Resolution = "best"
	}
}

func (t *Task) SetStateSaver(fn func()) {
	t.stateSaver = fn
}

func (t *Task) notifyState() {
	if t.stateSaver != nil {
		t.stateSaver()
	}
}

func (t *Task) NotifyState() {
	t.notifyState()
}

func IsStreamingSite(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	return strings.Contains(host, "youtube.com") ||
		strings.Contains(host, "youtu.be") ||
		strings.Contains(host, "tiktok.com") ||
		strings.Contains(host, "instagram.com") ||
		strings.Contains(host, "twitter.com") ||
		strings.Contains(host, "x.com") ||
		strings.Contains(host, "facebook.com") ||
		strings.Contains(host, "vimeo.com")
}

func isStreamingSite(rawURL string) bool {
	return IsStreamingSite(rawURL)
}

func getNodePath() string {
	if p, err := exec.LookPath("node"); err == nil {
		return p
	}
	if p, err := exec.LookPath("node.exe"); err == nil {
		return p
	}
	for _, p := range []string{
		"/opt/homebrew/bin/node",
		"/usr/local/bin/node",
		"/usr/bin/node",
		"C:\\Program Files\\nodejs\\node.exe",
		"C:\\Program Files (x86)\\nodejs\\node.exe",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func getYtDlpPath() string {
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates := []string{
			filepath.Join(exeDir, "yt-dlp"),
			filepath.Join(exeDir, "yt-dlp.exe"),
			filepath.Join(exeDir, "..", "resources", "bin", "yt-dlp"),
			filepath.Join(exeDir, "..", "resources", "bin", "yt-dlp.exe"),
			filepath.Join(exeDir, "..", "Resources", "bin", "yt-dlp"),
			filepath.Join(exeDir, "..", "Resources", "bin", "yt-dlp.exe"),
			filepath.Join(exeDir, "..", "bin", "yt-dlp"),
			filepath.Join(exeDir, "..", "bin", "yt-dlp.exe"),
			filepath.Join(exeDir, "resources", "bin", "yt-dlp"),
			filepath.Join(exeDir, "resources", "bin", "yt-dlp.exe"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}

	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, "bin", "yt-dlp"),
		filepath.Join(cwd, "bin", "yt-dlp.exe"),
		filepath.Join(cwd, "yt-dlp"),
		filepath.Join(cwd, "yt-dlp.exe"),
		filepath.Join(cwd, "resources", "bin", "yt-dlp"),
		filepath.Join(cwd, "resources", "bin", "yt-dlp.exe"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p
	}
	if p, err := exec.LookPath("yt-dlp.exe"); err == nil {
		return p
	}

	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		for _, name := range []string{"yt-dlp", "yt-dlp.exe"} {
			p := filepath.Join(homeDir, ".mad-dm", "bin", name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	return ""
}

func getFfmpegPath() string {
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates := []string{
			filepath.Join(exeDir, "ffmpeg"),
			filepath.Join(exeDir, "ffmpeg.exe"),
			filepath.Join(exeDir, "..", "resources", "bin", "ffmpeg"),
			filepath.Join(exeDir, "..", "resources", "bin", "ffmpeg.exe"),
			filepath.Join(exeDir, "..", "Resources", "bin", "ffmpeg"),
			filepath.Join(exeDir, "..", "Resources", "bin", "ffmpeg.exe"),
			filepath.Join(exeDir, "..", "bin", "ffmpeg"),
			filepath.Join(exeDir, "..", "bin", "ffmpeg.exe"),
			filepath.Join(exeDir, "resources", "bin", "ffmpeg"),
			filepath.Join(exeDir, "resources", "bin", "ffmpeg.exe"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}

	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, "bin", "ffmpeg"),
		filepath.Join(cwd, "bin", "ffmpeg.exe"),
		filepath.Join(cwd, "ffmpeg"),
		filepath.Join(cwd, "ffmpeg.exe"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	if p, err := exec.LookPath("ffmpeg.exe"); err == nil {
		return p
	}

	for _, p := range []string{
		"/opt/homebrew/bin/ffmpeg",
		"/usr/local/bin/ffmpeg",
		"/usr/bin/ffmpeg",
		"C:\\Program Files\\ffmpeg\\bin\\ffmpeg.exe",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		for _, name := range []string{"ffmpeg", "ffmpeg.exe"} {
			p := filepath.Join(homeDir, ".mad-dm", "bin", name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	return ""
}

func SanitizeFilename(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalid {
		name = strings.ReplaceAll(name, char, "_")
	}
	return strings.TrimSpace(name)
}

func sanitizeFilename(name string) string {
	return SanitizeFilename(name)
}

// Probe retrieves file size, headers, or streaming metadata.
func (t *Task) Probe() error {
	if isStreamingSite(t.URL) {
		t.IsStreaming = true
		ytPath := getYtDlpPath()
		if ytPath == "" {
			return fmt.Errorf("streaming resolver 'yt-dlp' not found in bin/ or PATH")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		args := []string{"--dump-json", "--no-warnings", "--no-playlist"}
		if nodePath := getNodePath(); nodePath != "" {
			args = append(args, "--js-runtimes", "node:"+nodePath)
		}
		args = append(args, t.URL)

		cmd := exec.CommandContext(ctx, ytPath, args...)
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to probe streaming video: %w", err)
		}

		var info struct {
			Title          string `json:"title"`
			Ext            string `json:"ext"`
			Filesize       int64  `json:"filesize"`
			FilesizeApprox int64  `json:"filesize_approx"`
		}
		if err := json.Unmarshal(out, &info); err == nil {
			cleanTitle := sanitizeFilename(info.Title)
			if cleanTitle == "" {
				cleanTitle = fmt.Sprintf("video_%d", time.Now().Unix())
			}
			t.mu.RLock()
			reqFmt := strings.ToLower(t.Format)
			t.mu.RUnlock()

			ext := reqFmt
			if ext != "mp3" && ext != "flac" && ext != "mp4" {
				ext = info.Ext
				if ext == "" {
					ext = "mp4"
				}
			}
			t.mu.Lock()
			t.Format = ext
			t.Filename = cleanTitle + "." + ext
			if info.Filesize > 0 {
				t.TotalSize = info.Filesize
			} else if info.FilesizeApprox > 0 {
				t.TotalSize = info.FilesizeApprox
			} else {
				t.TotalSize = 50 * 1024 * 1024 // estimate ~50MB
			}
			t.Resumable = true
			if t.Connections <= 0 {
				t.Connections = 8
			}
			t.Category = string(ResolveCategory(t.Filename, t.Format))
			t.mu.Unlock()
			return nil
		}
		return fmt.Errorf("failed to parse streaming metadata")
	}

	// Normal HTTP Range Probe
	req, err := http.NewRequest("GET", t.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 MAD-DM/1.0")
	req.Header.Set("Accept", "*/*")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("probe request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		if t.Filename == "" {
			t.Filename = parseFilename(resp, t.URL)
		}
		t.Category = string(ResolveCategory(t.Filename, t.Format))
		return fmt.Errorf("server responded with status: %s", resp.Status)
	}

	if t.Filename == "" {
		t.Filename = parseFilename(resp, t.URL)
	}

	contentLength := resp.ContentLength
	acceptRanges := resp.Header.Get("Accept-Ranges")
	contentRange := resp.Header.Get("Content-Range")

	if resp.StatusCode == http.StatusPartialContent {
		t.Resumable = true
		if contentRange != "" {
			parts := strings.Split(contentRange, "/")
			if len(parts) == 2 && parts[1] != "*" {
				if total, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					contentLength = total
				}
			}
		}
	} else if strings.EqualFold(acceptRanges, "bytes") {
		t.Resumable = true
	} else {
		t.Resumable = false
	}

	if contentLength > 0 {
		t.TotalSize = contentLength
	}

	if !t.Resumable || t.TotalSize <= 0 {
		t.Connections = 1
		t.Resumable = false
	}

	t.Category = string(ResolveCategory(t.Filename, t.Format))
	return nil
}

// Start begins or resumes downloading.
func (t *Task) Start() error {
	t.mu.RLock()
	isDownloading := t.Status == StatusDownloading
	needProbe := t.Filename == "" || t.TotalSize == 0
	t.mu.RUnlock()

	if isDownloading {
		return nil
	}

	if needProbe {
		if err := t.Probe(); err != nil {
			t.mu.Lock()
			t.Status = StatusError
			t.Error = err.Error()
			t.mu.Unlock()
			t.notifyState()
			return err
		}
	}

	t.mu.Lock()
	if t.Status == StatusDownloading {
		t.mu.Unlock()
		return nil
	}

	// Route to categorized directory if currently in base downloads folder
	if strings.EqualFold(filepath.Base(t.Directory), "downloads") {
		t.Directory = CategorizedDir(t.Directory, t.Filename, t.Format)
	}
	if t.Category == "" {
		t.Category = string(ResolveCategory(t.Filename, t.Format))
	}

	if err := os.MkdirAll(t.Directory, 0755); err != nil {
		t.Status = StatusError
		t.Error = err.Error()
		t.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.ctx = ctx
	t.cancelFunc = cancel
	t.Status = StatusDownloading
	t.Error = ""
	t.UpdatedAt = time.Now()

	// If streaming site, launch yt-dlp worker
	if t.IsStreaming {
		t.initVirtualChunks()
		t.mu.Unlock()
		t.notifyState()
		go t.runStreaming(ctx)
		return nil
	}

	// Normal HTTP chunk download
	outPath := t.FilePath()
	file, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Status = StatusError
		t.Error = err.Error()
		t.mu.Unlock()
		return err
	}
	t.file = file

	if t.TotalSize > 0 {
		_ = file.Truncate(t.TotalSize)
	}

	if len(t.Chunks) == 0 {
		t.initChunks()
	}

	t.mu.Unlock()
	t.notifyState()

	go t.run(ctx)
	return nil
}

func (t *Task) initVirtualChunks() {
	if len(t.Chunks) > 0 {
		return
	}
	t.Chunks = make([]*Chunk, t.Connections)
	chunkSize := t.TotalSize / int64(t.Connections)
	for i := 0; i < t.Connections; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == t.Connections-1 {
			end = t.TotalSize - 1
		}
		t.Chunks[i] = &Chunk{
			Index:      i,
			Start:      start,
			End:        end,
			Downloaded: 0,
			Total:      end - start + 1,
			Completed:  false,
		}
	}
}

func (t *Task) initChunks() {
	if !t.Resumable || t.TotalSize <= 0 || t.Connections <= 1 {
		t.Chunks = []*Chunk{
			{
				Index:      0,
				Start:      0,
				End:        t.TotalSize - 1,
				Downloaded: 0,
				Total:      t.TotalSize,
				Completed:  false,
			},
		}
		return
	}

	chunkSize := t.TotalSize / int64(t.Connections)
	t.Chunks = make([]*Chunk, t.Connections)

	for i := 0; i < t.Connections; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == t.Connections-1 {
			end = t.TotalSize - 1
		}
		t.Chunks[i] = &Chunk{
			Index:      i,
			Start:      start,
			End:        end,
			Downloaded: 0,
			Total:      end - start + 1,
			Completed:  false,
		}
	}
}

func getYtDlpVideoFormatArgs(resolution string, hasFfmpeg bool) []string {
	res := strings.ToLower(strings.TrimSpace(resolution))
	var hFilter string
	switch res {
	case "2160p", "4k":
		hFilter = "[height<=2160]"
	case "1440p", "2k":
		hFilter = "[height<=1440]"
	case "1080p", "1080":
		hFilter = "[height<=1080]"
	case "720p", "720":
		hFilter = "[height<=720]"
	case "480p", "480":
		hFilter = "[height<=480]"
	case "360p", "360":
		hFilter = "[height<=360]"
	default:
		hFilter = ""
	}

	if hasFfmpeg {
		if hFilter != "" {
			fmtStr := fmt.Sprintf("bestvideo%s[ext=mp4]+bestaudio[ext=m4a]/bestvideo%s+bestaudio/best%s[ext=mp4]/best%s", hFilter, hFilter, hFilter, hFilter)
			return []string{
				"--format", fmtStr,
				"--merge-output-format", "mp4",
				"--postprocessor-args", "ffmpeg:-movflags +faststart",
			}
		}
		return []string{
			"--format", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best[ext=mp4]/best",
			"--merge-output-format", "mp4",
			"--postprocessor-args", "ffmpeg:-movflags +faststart",
		}
	}

	if hFilter != "" {
		return []string{"--format", fmt.Sprintf("best%s[ext=mp4]/best%s", hFilter, hFilter)}
	}
	return []string{"--format", "best[ext=mp4]/best"}
}

func (t *Task) runStreaming(ctx context.Context) {
	ytPath := getYtDlpPath()
	ffmpegPath := getFfmpegPath()

	t.mu.RLock()
	desiredFmt := strings.ToLower(t.Format)
	if desiredFmt != "mp3" && desiredFmt != "flac" {
		desiredFmt = "mp4"
	}
	resOpt := t.Resolution
	currFilename := t.Filename
	dir := t.Directory
	conns := t.Connections
	t.mu.RUnlock()

	// Ensure filename has matching extension
	currExt := filepath.Ext(currFilename)
	if currExt != "."+desiredFmt {
		currFilename = strings.TrimSuffix(currFilename, currExt) + "." + desiredFmt
		t.mu.Lock()
		t.Filename = currFilename
		t.mu.Unlock()
	}
	targetFile := filepath.Join(dir, currFilename)

	args := []string{
		"-o", targetFile,
		"--newline",
		"--concurrent-fragments", strconv.Itoa(conns),
		"--no-playlist",
	}

	if ffmpegPath != "" {
		args = append(args, "--ffmpeg-location", ffmpegPath)
	}

	if nodePath := getNodePath(); nodePath != "" {
		args = append(args, "--js-runtimes", "node:"+nodePath)
	}

	switch desiredFmt {
	case "mp3":
		args = append(args, "-x", "--audio-format", "mp3", "--audio-quality", "0")
	case "flac":
		args = append(args, "-x", "--audio-format", "flac")
	default: // mp4
		args = append(args, getYtDlpVideoFormatArgs(resOpt, ffmpegPath != "")...)
	}
	args = append(args, t.URL)

	cmd := exec.CommandContext(ctx, ytPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.mu.Lock()
		t.Status = StatusError
		t.Error = err.Error()
		t.mu.Unlock()
		t.notifyState()
		return
	}

	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		t.mu.Lock()
		t.Status = StatusError
		t.Error = err.Error()
		t.mu.Unlock()
		t.notifyState()
		return
	}

	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for scanner.Scan() {
		line := scanner.Text()
		t.parseYtDlpProgress(line)
	}

	err = cmd.Wait()

	t.mu.Lock()
	defer t.mu.Unlock()

	select {
	case <-ctx.Done():
		t.Status = StatusPaused
	default:
		if err != nil {
			t.Status = StatusError
			t.Error = err.Error()
		} else {
			t.Status = StatusCompleted
			t.Speed = 0
			t.ETA = 0
			// Verify output file existence (yt-dlp might adjust extensions)
			if fi, sErr := os.Stat(targetFile); sErr == nil {
				t.TotalSize = fi.Size()
				t.Downloaded = fi.Size()
			} else {
				// Check for format extension alternative
				altFile := strings.TrimSuffix(targetFile, filepath.Ext(targetFile)) + "." + desiredFmt
				if fi, aErr := os.Stat(altFile); aErr == nil {
					t.Filename = filepath.Base(altFile)
					t.TotalSize = fi.Size()
					t.Downloaded = fi.Size()
				}
			}
			for _, c := range t.Chunks {
				c.Completed = true
				c.Downloaded = c.Total
			}
		}
	}
	t.UpdatedAt = time.Now()
	t.notifyState()
}

var progressRegex = regexp.MustCompile(`\[download\]\s+([\d\.]+)%\s+of\s+~?([\d\.]+\w+)\s+(?:at\s+([\d\.]+\w+\/s))?\s*(?:ETA\s+([\d:]+))?`)

func (t *Task) parseYtDlpProgress(line string) {
	matches := progressRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		if pct, err := strconv.ParseFloat(matches[1], 64); err == nil {
			t.mu.Lock()
			if t.TotalSize > 0 {
				t.Downloaded = int64(float64(t.TotalSize) * (pct / 100.0))
			}
			// Update virtual chunks for the visual matrix
			if len(t.Chunks) > 0 {
				activeChunkIdx := int(pct / (100.0 / float64(len(t.Chunks))))
				if activeChunkIdx >= len(t.Chunks) {
					activeChunkIdx = len(t.Chunks) - 1
				}
				for i := 0; i < len(t.Chunks); i++ {
					if i < activeChunkIdx {
						t.Chunks[i].Completed = true
						t.Chunks[i].Downloaded = t.Chunks[i].Total
					} else if i == activeChunkIdx {
						chunkPct := (pct - float64(i)*(100.0/float64(len(t.Chunks)))) / (100.0 / float64(len(t.Chunks)))
						if chunkPct > 1.0 {
							chunkPct = 1.0
						}
						t.Chunks[i].Downloaded = int64(float64(t.Chunks[i].Total) * chunkPct)
					}
				}
			}

			// Parse Speed
			if len(matches) > 3 && matches[3] != "" {
				t.Speed = parseSpeedString(matches[3])
			}

			// Parse ETA
			if len(matches) > 4 && matches[4] != "" {
				t.ETA = parseEtaString(matches[4])
			}

			t.mu.Unlock()
			t.notifyState()
		}
	}
}

func parseSpeedString(s string) int64 {
	s = strings.TrimSuffix(s, "/s")
	s = strings.TrimSpace(s)
	multiplier := int64(1)
	if strings.HasSuffix(s, "KiB") || strings.HasSuffix(s, "KB") {
		multiplier = 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "KiB"), "KB")
	} else if strings.HasSuffix(s, "MiB") || strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "MiB"), "MB")
	} else if strings.HasSuffix(s, "GiB") || strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "GiB"), "GB")
	}

	if val, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return int64(val * float64(multiplier))
	}
	return 0
}

func parseEtaString(s string) int64 {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) == 2 {
		m, _ := strconv.ParseInt(parts[0], 10, 64)
		sec, _ := strconv.ParseInt(parts[1], 10, 64)
		return m*60 + sec
	} else if len(parts) == 3 {
		h, _ := strconv.ParseInt(parts[0], 10, 64)
		m, _ := strconv.ParseInt(parts[1], 10, 64)
		sec, _ := strconv.ParseInt(parts[2], 10, 64)
		return h*3600 + m*60 + sec
	}
	return 0
}

func (t *Task) run(ctx context.Context) {
	var wg sync.WaitGroup
	errChan := make(chan error, len(t.Chunks))

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		var lastDownloaded int64
		lastTime := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				current := atomic.LoadInt64(&t.Downloaded)
				elapsed := now.Sub(lastTime).Seconds()
				if elapsed > 0 {
					delta := current - lastDownloaded
					speed := int64(float64(delta) / elapsed)
					if speed < 0 {
						speed = 0
					}
					t.mu.Lock()
					t.Speed = speed
					if speed > 0 && t.TotalSize > current {
						t.ETA = (t.TotalSize - current) / speed
					} else {
						t.ETA = 0
					}
					t.mu.Unlock()
					lastDownloaded = current
					lastTime = now
				}
			}
		}
	}()

	for _, chunk := range t.Chunks {
		if chunk.Completed {
			continue
		}
		wg.Add(1)
		go func(c *Chunk) {
			defer wg.Done()
			if err := t.downloadChunk(ctx, c); err != nil {
				if ctx.Err() == nil {
					errChan <- err
				}
			}
		}(chunk)
	}

	wg.Wait()
	close(errChan)

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.file != nil {
		_ = t.file.Sync()
		_ = t.file.Close()
		t.file = nil
	}

	select {
	case <-ctx.Done():
		if t.Status == StatusDownloading {
			t.Status = StatusPaused
		}
	default:
		if err := <-errChan; err != nil {
			t.Status = StatusError
			t.Error = err.Error()
		} else {
			t.Status = StatusCompleted
			t.Speed = 0
			t.ETA = 0
			t.Downloaded = t.TotalSize
		}
	}

	t.UpdatedAt = time.Now()
	t.notifyState()
}

func (t *Task) downloadChunk(ctx context.Context, c *Chunk) error {
	for {
		if c.Completed || (c.Total > 0 && c.Downloaded >= c.Total) {
			c.Completed = true
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		startOffset := c.Start + c.Downloaded
		req, err := http.NewRequestWithContext(ctx, "GET", t.URL, nil)
		if err != nil {
			return err
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 MAD-DM/1.0")

		if t.Resumable && c.End > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", startOffset, c.End))
		}

		resp, err := t.client.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			return fmt.Errorf("chunk %d HTTP %s", c.Index, resp.Status)
		}

		buf := make([]byte, 64*1024)
		for {
			select {
			case <-ctx.Done():
				resp.Body.Close()
				return ctx.Err()
			default:
			}

			n, rErr := resp.Body.Read(buf)
			if n > 0 {
				writePos := c.Start + c.Downloaded
				_, wErr := t.file.WriteAt(buf[:n], writePos)
				if wErr != nil {
					resp.Body.Close()
					return fmt.Errorf("write error: %w", wErr)
				}
				c.Downloaded += int64(n)
				atomic.AddInt64(&t.Downloaded, int64(n))
			}

			if rErr != nil {
				resp.Body.Close()
				if rErr == io.EOF {
					c.Completed = true
					return nil
				}
				return rErr
			}
		}
	}
}

func (t *Task) Pause() {
	t.mu.Lock()
	if t.Status == StatusDownloading && t.cancelFunc != nil {
		t.cancelFunc()
		t.Status = StatusPaused
		t.Speed = 0
		t.ETA = 0
	}
	t.mu.Unlock()
	t.notifyState()
}

func (t *Task) Cancel(deleteFile bool) {
	t.mu.RLock()
	wasCompleted := t.Status == StatusCompleted
	t.mu.RUnlock()

	t.Pause()
	t.mu.Lock()
	t.Status = StatusCancelled
	filePath := t.FilePath()
	t.mu.Unlock()

	// Never delete completed file on normal cancel/remove from list
	if deleteFile && !wasCompleted {
		_ = os.Remove(filePath)
	}
	t.notifyState()
}

func parseFilename(resp *http.Response, rawURL string) string {
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if filename, ok := params["filename"]; ok && filename != "" {
				return filepath.Base(filename)
			}
			if filename, ok := params["filename*"]; ok && filename != "" {
				parts := strings.SplitN(filename, "''", 2)
				if len(parts) == 2 {
					if decoded, err := url.QueryUnescape(parts[1]); err == nil {
						return filepath.Base(decoded)
					}
				}
			}
		}
	}

	parsed, err := url.Parse(rawURL)
	if err == nil {
		base := filepath.Base(parsed.Path)
		if base != "" && base != "/" && base != "." {
			return base
		}
	}

	return fmt.Sprintf("download_%d", time.Now().Unix())
}
