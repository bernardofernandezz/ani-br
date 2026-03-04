package mem

import (
	"context"
	"sort"
	"sync"

	"github.com/bernardofernandezz/ani-br/internal/domain"
)

type HistoryStore struct {
	mu      sync.RWMutex
	entries []domain.HistoryEntry
}

func NewHistoryStore() *HistoryStore {
	return &HistoryStore{}
}

func (s *HistoryStore) AddEntry(_ context.Context, entry domain.HistoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	return nil
}

func (s *HistoryStore) ListEntries(_ context.Context, limit int) ([]domain.HistoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := append([]domain.HistoryEntry(nil), s.entries...)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].WatchedAt.After(cp[j].WatchedAt)
	})
	if limit > 0 && len(cp) > limit {
		cp = cp[:limit]
	}
	return cp, nil
}

type ProgressStore struct {
	mu    sync.RWMutex
	items map[string]domain.Progress
}

func NewProgressStore() *ProgressStore {
	return &ProgressStore{items: make(map[string]domain.Progress)}
}

func (s *ProgressStore) SaveProgress(_ context.Context, p domain.Progress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[p.EpisodeID] = p
	return nil
}

func (s *ProgressStore) GetProgress(_ context.Context, episodeID string) (domain.Progress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.items[episodeID]
	if !ok {
		return domain.Progress{}, domain.ErrProgressNotFound
	}
	return p, nil
}

