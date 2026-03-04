package continuewatching

import (
	"context"
	"errors"

	"github.com/bernardofernandezz/ani-br/internal/domain"
)

type Service struct {
	history  domain.HistoryRepository
	progress domain.ProgressRepository
}

func New(history domain.HistoryRepository, progress domain.ProgressRepository) *Service {
	return &Service{history: history, progress: progress}
}

// GetLast retorna o último item do histórico (se existir).
func (s *Service) GetLast(ctx context.Context) (domain.HistoryEntry, error) {
	entries, err := s.history.ListEntries(ctx, 1)
	if err != nil {
		return domain.HistoryEntry{}, err
	}
	if len(entries) == 0 {
		return domain.HistoryEntry{}, domain.ErrAnimeNotFound
	}
	return entries[0], nil
}

func (s *Service) GetProgress(ctx context.Context, episodeID string) (domain.Progress, error) {
	p, err := s.progress.GetProgress(ctx, episodeID)
	if err != nil {
		if errors.Is(err, domain.ErrProgressNotFound) {
			return domain.Progress{}, domain.ErrProgressNotFound
		}
		return domain.Progress{}, err
	}
	return p, nil
}

