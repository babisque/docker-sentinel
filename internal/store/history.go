package store

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type ContainerHistory struct {
	Name      string
	Image     string
	Logs      []string
	StoppedAt time.Time
}

type HistoryStore struct {
	mu   sync.RWMutex
	data map[string]*ContainerHistory
}

func NewHistoryStore() *HistoryStore {
	return &HistoryStore{
		data: make(map[string]*ContainerHistory),
	}
}

func (s *HistoryStore) Save(id string, history *ContainerHistory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = history
}

func (s *HistoryStore) Get(id string) (*ContainerHistory, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	history, exists := s.data[id]
	return history, exists
}

func (s *HistoryStore) ExportJSON(filename string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	enconder := json.NewEncoder(file)
	enconder.SetIndent("", "  ")
	return enconder.Encode(s.data)
}
