package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu       sync.Mutex
	filePath string
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Store{
		filePath: filepath.Join(dir, "state.json"),
	}, nil
}

func (s *Store) Save(data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	tempPath := s.filePath + ".tmp"
	if err := os.WriteFile(tempPath, bytes, 0644); err != nil {
		return err
	}
	return os.Rename(tempPath, s.filePath)
}

func (s *Store) Load(target interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil
	}

	bytes, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, target)
}
