package history

import (
	"context"

	"github.com/bernardofernandezz/ani-br/internal/domain"
)

type Service struct {
	repo domain.HistoryRepository
}

func New(repo domain.HistoryRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, limit int) ([]domain.HistoryEntry, error) {
	return s.repo.ListEntries(ctx, limit)
}

