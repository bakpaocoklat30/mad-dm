package downloader

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mad-dm/engine/pkg/state"
)

type ManagerConfig struct {
	MaxConcurrent int    `json:"max_concurrent"`
	DefaultDir    string `json:"default_dir"`
	StateDir      string `json:"state_dir"`
}

type Manager struct {
	mu            sync.RWMutex
	tasks         map[string]*Task
	taskOrder     []string
	config        ManagerConfig
	store         *state.Store
	eventChannels []chan *Task
	eventMu       sync.Mutex
}

func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.DefaultDir == "" {
		cfg.DefaultDir = "downloads"
	}
	if cfg.StateDir == "" {
		cfg.StateDir = ".mad-dm"
	}

	store, err := state.NewStore(cfg.StateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init state store: %w", err)
	}

	mgr := &Manager{
		tasks:     make(map[string]*Task),
		taskOrder: make([]string, 0),
		config:    cfg,
		store:     store,
	}

	// Restore state
	if err := mgr.restore(); err != nil {
		// Non-fatal, start fresh
		fmt.Printf("Notice: could not restore state: %v\n", err)
	}

	// Start auto-queue scheduler loop
	go mgr.scheduleLoop()

	return mgr, nil
}

func generateID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (m *Manager) AddTask(rawURL, dir string, connections int) (*Task, error) {
	return m.AddTaskWithFormat(rawURL, dir, connections, "")
}

func (m *Manager) AddTaskWithFormat(rawURL, dir string, connections int, format string) (*Task, error) {
	homeDir, _ := os.UserHomeDir()
	userDownloads := ""
	if homeDir != "" {
		userDownloads = filepath.Join(homeDir, "Downloads")
	}

	isDefaultDir := (dir == "" || dir == m.config.DefaultDir || (userDownloads != "" && dir == userDownloads) || strings.EqualFold(filepath.Base(dir), "downloads"))
	if dir == "" {
		dir = m.config.DefaultDir
	}

	id := generateID()
	task := NewTask(id, rawURL, dir, connections)
	task.SetFormat(format)
	task.SetStateSaver(func() {
		_ = m.save()
		m.broadcast(task)
	})

	// Pre-probe to get filename and size
	_ = task.Probe()

	// Automatically route to category subfolder if default download folder is used
	if isDefaultDir {
		task.Directory = CategorizedDir(m.config.DefaultDir, task.Filename, task.Format)
	}
	task.Category = string(ResolveCategory(task.Filename, task.Format))
	_ = os.MkdirAll(task.Directory, 0755)

	m.mu.Lock()
	m.tasks[id] = task
	m.taskOrder = append(m.taskOrder, id)
	m.mu.Unlock()

	_ = m.save()
	m.broadcast(task)

	return task, nil
}

func (m *Manager) AddMergedTask(filename, dir, format string, totalSize int64) *Task {
	homeDir, _ := os.UserHomeDir()
	userDownloads := ""
	if homeDir != "" {
		userDownloads = filepath.Join(homeDir, "Downloads")
	}

	isDefaultDir := (dir == "" || dir == m.config.DefaultDir || (userDownloads != "" && dir == userDownloads) || strings.EqualFold(filepath.Base(dir), "downloads"))
	if dir == "" {
		dir = m.config.DefaultDir
	}

	cat := string(ResolveCategory(filename, format))
	if isDefaultDir {
		dir = CategorizedDir(m.config.DefaultDir, filename, format)
	}
	_ = os.MkdirAll(dir, 0755)

	id := generateID()
	task := NewTask(id, "local://merged-playlist", dir, 1)
	task.Filename = filename
	task.Format = format
	task.Category = cat
	task.IsMerged = true
	task.TotalSize = totalSize
	task.Status = StatusQueued
	task.Resumable = true
	task.SetStateSaver(func() {
		_ = m.save()
		m.broadcast(task)
	})

	m.mu.Lock()
	m.tasks[id] = task
	m.taskOrder = append(m.taskOrder, id)
	m.mu.Unlock()

	_ = m.save()
	m.broadcast(task)
	return task
}

func (m *Manager) CoordinatePlaylistMerge(parts []*Task, mergedTask *Task, format string, keepParts bool) {
	go func() {
		// Wait for all parts to finish
		for {
			allDone := true
			hasError := false
			var downloadedBytes int64

			for _, t := range parts {
				t.mu.RLock()
				status := t.Status
				downloadedBytes += t.Downloaded
				t.mu.RUnlock()

				if status == StatusError {
					hasError = true
				}
				if status != StatusCompleted && status != StatusError && status != StatusCancelled {
					allDone = false
				}
			}

			mergedTask.mu.Lock()
			mergedTask.Downloaded = downloadedBytes
			if mergedTask.Downloaded > mergedTask.TotalSize {
				mergedTask.TotalSize = mergedTask.Downloaded
			}
			mergedTask.mu.Unlock()
			mergedTask.notifyState()

			if allDone {
				if hasError {
					mergedTask.mu.Lock()
					mergedTask.Status = StatusError
					mergedTask.Error = "One or more playlist tracks failed to download"
					mergedTask.mu.Unlock()
					mergedTask.notifyState()
					return
				}
				break
			}
			time.Sleep(1 * time.Second)
		}

		// Update to active merge status
		mergedTask.mu.Lock()
		mergedTask.Status = StatusDownloading
		mergedTask.Speed = 0
		mergedTask.ETA = 0
		mergedTask.mu.Unlock()
		mergedTask.notifyState()

		var inputPaths []string
		for _, t := range parts {
			inputPaths = append(inputPaths, t.FilePath())
		}

		outputPath := mergedTask.FilePath()
		err := MergeFiles(inputPaths, outputPath, format)
		if err != nil {
			mergedTask.mu.Lock()
			mergedTask.Status = StatusError
			mergedTask.Error = "Merge failed: " + err.Error()
			mergedTask.mu.Unlock()
			mergedTask.notifyState()
			return
		}

		finalSize := int64(0)
		if fi, sErr := os.Stat(outputPath); sErr == nil {
			finalSize = fi.Size()
		}

		mergedTask.mu.Lock()
		mergedTask.Status = StatusCompleted
		mergedTask.TotalSize = finalSize
		mergedTask.Downloaded = finalSize
		mergedTask.Speed = 0
		mergedTask.ETA = 0
		mergedTask.UpdatedAt = time.Now()
		mergedTask.mu.Unlock()
		mergedTask.notifyState()

		if !keepParts {
			for _, t := range parts {
				_ = os.Remove(t.FilePath())
				_ = m.CancelTask(t.ID, true)
			}
		}
	}()
}

func (m *Manager) StartTask(id string) error {
	m.mu.RLock()
	task, exists := m.tasks[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task %s not found", id)
	}

	return task.Start()
}

func (m *Manager) PauseTask(id string) error {
	m.mu.RLock()
	task, exists := m.tasks[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task %s not found", id)
	}

	task.Pause()
	return nil
}

func (m *Manager) CancelTask(id string, deleteFile bool) error {
	m.mu.Lock()
	task, exists := m.tasks[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("task %s not found", id)
	}

	delete(m.tasks, id)
	var newOrder []string
	for _, oid := range m.taskOrder {
		if oid != id {
			newOrder = append(newOrder, oid)
		}
	}
	m.taskOrder = newOrder
	m.mu.Unlock()

	task.Cancel(deleteFile)
	_ = m.save()
	m.broadcast(task)
	return nil
}

func (m *Manager) GetTask(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, exists := m.tasks[id]
	return task, exists
}

func (m *Manager) ListTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Task, 0, len(m.taskOrder))
	for _, id := range m.taskOrder {
		if t, ok := m.tasks[id]; ok {
			result = append(result, t)
		}
	}
	return result
}

func (m *Manager) Subscribe() chan *Task {
	m.eventMu.Lock()
	defer m.eventMu.Unlock()

	ch := make(chan *Task, 100)
	m.eventChannels = append(m.eventChannels, ch)
	return ch
}

func (m *Manager) Unsubscribe(ch chan *Task) {
	m.eventMu.Lock()
	defer m.eventMu.Unlock()

	for i, c := range m.eventChannels {
		if c == ch {
			m.eventChannels = append(m.eventChannels[:i], m.eventChannels[i+1:]...)
			close(ch)
			break
		}
	}
}

func (m *Manager) broadcast(task *Task) {
	m.eventMu.Lock()
	defer m.eventMu.Unlock()

	for _, ch := range m.eventChannels {
		select {
		case ch <- task:
		default:
			// avoid blocking if channel full
		}
	}
}

func (m *Manager) scheduleLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		activeCount := 0
		for _, task := range m.tasks {
			if task.Status == StatusDownloading {
				activeCount++
			}
		}

		var toStart []*Task
		if activeCount < m.config.MaxConcurrent {
			for _, id := range m.taskOrder {
				task := m.tasks[id]
				if task.Status == StatusQueued {
					toStart = append(toStart, task)
					activeCount++
					if activeCount >= m.config.MaxConcurrent {
						break
					}
				}
			}
		}
		m.mu.Unlock()

		// Start tasks outside the manager lock to prevent deadlock
		for _, task := range toStart {
			go func(t *Task) {
				_ = t.Start()
			}(task)
		}
	}
}

func (m *Manager) save() error {
	m.mu.RLock()
	tasks := make([]*Task, 0, len(m.tasks))
	for _, id := range m.taskOrder {
		if t, ok := m.tasks[id]; ok {
			tasks = append(tasks, t)
		}
	}
	m.mu.RUnlock()

	return m.store.Save(tasks)
}

func (m *Manager) restore() error {
	var savedTasks []*Task
	if err := m.store.Load(&savedTasks); err != nil {
		return err
	}

	for _, t := range savedTasks {
		// If task was downloading when shut down, mark as paused
		if t.Status == StatusDownloading {
			t.Status = StatusPaused
		}
		t.client = NewTask("", "", "", 0).client
		t.SetStateSaver(func() {
			_ = m.save()
			m.broadcast(t)
		})
		m.tasks[t.ID] = t
		m.taskOrder = append(m.taskOrder, t.ID)
	}

	return nil
}
