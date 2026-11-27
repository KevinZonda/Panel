package web

import (
	"maps"
	"sync"
	"time"

	"github.com/KevinZonda/Panel/lib/tokenx"
	"github.com/gin-gonic/gin"
)

const actionResultTTL = 10 * time.Minute

type storedActionResult struct {
	kind      string
	data      gin.H
	expiresAt time.Time
}

type actionResultStore struct {
	mu      sync.Mutex
	entries map[string]storedActionResult
}

func newActionResultStore() *actionResultStore {
	return &actionResultStore{entries: map[string]storedActionResult{}}
}

func (s *actionResultStore) put(kind string, data gin.H) (string, error) {
	token, err := tokenx.New(16)
	if err != nil {
		return "", err
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	s.entries[token] = storedActionResult{
		kind:      kind,
		data:      data,
		expiresAt: now.Add(actionResultTTL),
	}
	return token, nil
}

func (s *actionResultStore) get(kind, token string) (gin.H, bool) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[token]
	if !ok || entry.kind != kind {
		return nil, false
	}
	if now.After(entry.expiresAt) {
		delete(s.entries, token)
		return nil, false
	}
	// Rendering adds request-specific fields to the top-level map.
	return maps.Clone(entry.data), true
}

func (s *actionResultStore) cleanupLocked(now time.Time) {
	for token, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, token)
		}
	}
}
