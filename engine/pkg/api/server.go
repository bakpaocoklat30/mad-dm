package api

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mad-dm/engine/pkg/downloader"
)

//go:embed web/*
var webFS embed.FS

func openInFileManager(filePath string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", "-R", filePath).Start()
	case "windows":
		_ = exec.Command("explorer", "/select,", filePath).Start()
	default:
		dir := filepath.Dir(filePath)
		_ = exec.Command("xdg-open", dir).Start()
	}
}

type Server struct {
	manager *downloader.Manager
	port    int
}

func NewServer(manager *downloader.Manager, port int) *Server {
	if port <= 0 {
		port = 17890
	}
	return &Server{
		manager: manager,
		port:    port,
	}
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Web Dashboard UI (embedded)
	sub, err := fs.Sub(webFS, "web")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(sub)))
	}

	// Endpoints
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTaskAction)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/sniff", s.handleSniff)
	mux.HandleFunc("/api/playlist/inspect", s.handlePlaylistInspect)
	mux.HandleFunc("/api/playlist/download", s.handlePlaylistDownload)
	mux.HandleFunc("/api/tasks/merge", s.handleTasksMerge)
	mux.HandleFunc("/api/extension/info", s.handleExtensionInfo)
	mux.HandleFunc("/api/extension/install", s.handleExtensionInstall)
	mux.HandleFunc("/api/extension/reveal", s.handleExtensionReveal)
	mux.HandleFunc("/api/extension/launch", s.handleExtensionLaunch)
	mux.HandleFunc("/api/shutdown", s.handleShutdown)

	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	fmt.Printf("[MAD-DM Server] Listening on http://localhost:%d\n", s.port)
	return http.ListenAndServe(addr, s.corsMiddleware(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": "1.0.0",
		"engine":  "mad-dm-go",
	})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		tasks := s.manager.ListTasks()
		_ = json.NewEncoder(w).Encode(tasks)

	case http.MethodPost:
		var req struct {
			URL         string `json:"url"`
			Directory   string `json:"directory"`
			Connections int    `json:"connections"`
			AutoStart   bool   `json:"auto_start"`
			Format      string `json:"format"`
			Resolution  string `json:"resolution"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.URL == "" {
			http.Error(w, "URL is required", http.StatusBadRequest)
			return
		}

		task, err := s.manager.AddTaskWithFormatAndResolution(req.URL, req.Directory, req.Connections, req.Format, req.Resolution)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if req.AutoStart {
			_ = s.manager.StartTask(task.ID)
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(task)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTaskAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// URL format: /api/tasks/{id}/{action}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid task URL", http.StatusBadRequest)
		return
	}

	taskID := parts[2]

	if len(parts) == 3 {
		// GET /api/tasks/{id}
		task, ok := s.manager.GetTask(taskID)
		if !ok {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(task)
		return
	}

	action := parts[3]
	switch action {
	case "start", "resume":
		if err := s.manager.StartTask(taskID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "task started"})

	case "pause":
		if err := s.manager.PauseTask(taskID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "task paused"})

	case "open_folder":
		task, ok := s.manager.GetTask(taskID)
		if !ok {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		go openInFileManager(task.FilePath())
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "folder opened"})

	case "cancel", "delete":
		deleteFile := r.URL.Query().Get("delete_file") == "true"
		if err := s.manager.CancelTask(taskID, deleteFile); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "task cancelled"})

	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleEvents streams Server-Sent Events (SSE) to connected clients.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.manager.Subscribe()
	defer s.manager.Unsubscribe(ch)

	// Send initial list of all tasks
	tasks := s.manager.ListTasks()
	initialData, _ := json.Marshal(tasks)
	fmt.Fprintf(w, "event: init\ndata: %s\n\n", initialData)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case task := <-ch:
			data, err := json.Marshal(task)
			if err == nil {
				fmt.Fprintf(w, "event: update\ndata: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

func (s *Server) handleSniff(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tempTask := downloader.NewTask("sniff", req.URL, "", 1)
	err := tempTask.Probe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"url":        req.URL,
		"filename":   tempTask.Filename,
		"total_size": tempTask.TotalSize,
		"resumable":  tempTask.Resumable,
	})
}

func (s *Server) handlePlaylistInspect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	info, err := downloader.InspectPlaylist(req.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(info)
}

func (s *Server) handlePlaylistDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL            string   `json:"url"`
		Directory      string   `json:"directory"`
		Connections    int      `json:"connections"`
		Format         string   `json:"format"` // "mp4", "mp3", "flac"
		Resolution     string   `json:"resolution"`
		AutoStart      bool     `json:"auto_start"`
		SelectedIDs    []string `json:"selected_ids"` // optional
		Merge          bool     `json:"merge"`
		KeepParts      bool     `json:"keep_parts"`
		MergedFilename string   `json:"merged_filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	info, err := downloader.InspectPlaylist(req.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	selectedMap := make(map[string]bool)
	for _, id := range req.SelectedIDs {
		selectedMap[id] = true
	}

	var createdTasks []*downloader.Task
	var totalEstimatedSize int64

	for _, entry := range info.Entries {
		if len(selectedMap) > 0 && !selectedMap[entry.ID] {
			continue
		}

		task, err := s.manager.AddTaskWithFormatAndResolution(entry.URL, req.Directory, req.Connections, req.Format, req.Resolution)
		if err == nil {
			if req.AutoStart {
				_ = s.manager.StartTask(task.ID)
			}
			createdTasks = append(createdTasks, task)
			if task.TotalSize > 0 {
				totalEstimatedSize += task.TotalSize
			} else {
				totalEstimatedSize += 50 * 1024 * 1024 // estimate
			}
		}
	}

	var mergedTask *downloader.Task
	if req.Merge && len(createdTasks) > 0 {
		ext := strings.ToLower(req.Format)
		if ext != "mp3" && ext != "flac" {
			ext = "mp4"
		}
		mergedName := req.MergedFilename
		if mergedName == "" {
			cleanTitle := downloader.SanitizeFilename(info.Title)
			if cleanTitle == "" {
				cleanTitle = "Playlist_Compilation"
			}
			mergedName = fmt.Sprintf("[MERGED] %s.%s", cleanTitle, ext)
		}

		mergedTask = s.manager.AddMergedTask(mergedName, req.Directory, ext, req.Resolution, totalEstimatedSize)
		s.manager.CoordinatePlaylistMerge(createdTasks, mergedTask, ext, req.Resolution, req.KeepParts)
	}

	res := map[string]interface{}{
		"message": fmt.Sprintf("queued %d tracks from playlist '%s'", len(createdTasks), info.Title),
		"count":   len(createdTasks),
		"tasks":   createdTasks,
		"merge":   req.Merge,
	}
	if mergedTask != nil {
		res["merged_task"] = mergedTask
	}

	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleTasksMerge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TaskIDs        []string `json:"task_ids"`
		OutputFilename string   `json:"output_filename"`
		Format         string   `json:"format"`
		Resolution     string   `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.TaskIDs) < 2 {
		http.Error(w, "At least 2 tasks are required to merge", http.StatusBadRequest)
		return
	}

	var inputPaths []string
	var targetDir string
	var totalSize int64

	for _, id := range req.TaskIDs {
		t, ok := s.manager.GetTask(id)
		if !ok {
			http.Error(w, fmt.Sprintf("task %s not found", id), http.StatusNotFound)
			return
		}
		if t.Status != downloader.StatusCompleted {
			http.Error(w, fmt.Sprintf("task '%s' is not completed yet", t.Filename), http.StatusBadRequest)
			return
		}
		inputPaths = append(inputPaths, t.FilePath())
		totalSize += t.TotalSize
		if targetDir == "" {
			targetDir = t.Directory
		}
	}

	ext := strings.ToLower(req.Format)
	if ext != "mp3" && ext != "flac" {
		ext = "mp4"
	}

	outName := req.OutputFilename
	if outName == "" {
		outName = fmt.Sprintf("[MERGED] Manual_Merge_%d.%s", time.Now().Unix(), ext)
	}

	mergedTask := s.manager.AddMergedTask(outName, targetDir, ext, req.Resolution, totalSize)
	mergedTask.Status = downloader.StatusDownloading

	go func() {
		err := downloader.MergeFiles(inputPaths, mergedTask.FilePath(), ext, req.Resolution)
		if err != nil {
			mergedTask.Status = downloader.StatusError
			mergedTask.Error = err.Error()
			mergedTask.NotifyState()
			return
		}
		fi, _ := os.Stat(mergedTask.FilePath())
		if fi != nil {
			mergedTask.TotalSize = fi.Size()
			mergedTask.Downloaded = fi.Size()
		}
		mergedTask.Status = downloader.StatusCompleted
		mergedTask.NotifyState()
	}()

	_ = json.NewEncoder(w).Encode(mergedTask)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "shutting_down",
		"message": "Engine daemon shutting down gracefully",
	})

	go func() {
		time.Sleep(300 * time.Millisecond)
		for _, t := range s.manager.ListTasks() {
			if t.Status == downloader.StatusDownloading {
				_ = s.manager.PauseTask(t.ID)
			}
		}
		os.Exit(0)
	}()
}
