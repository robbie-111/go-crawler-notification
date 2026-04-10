package state

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
)

const FilePath = "monitor_state.json"

type Entry struct {
	LastSeenVersion string    `json:"last_seen_version"`
	LastCheckedAt   time.Time `json:"last_checked_at"`
}

type Store struct {
	mu      sync.Mutex
	path    string
	entries map[string]Entry
}

func New(path string) *Store {
	return &Store{path: path, entries: map[string]Entry{}}
}

func Load(path string) (*Store, error) {
	store := New(path)

	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return nil, err
	}

	if len(body) == 0 {
		return store, nil
	}

	if err := json.Unmarshal(body, &store.entries); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *Store) Get(key string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	return entry, ok
}

func (s *Store) Set(key string, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = entry

	body, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, body, 0644)
}
