package searchanime

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bernardofernandezz/ani-br/internal/domain"
	"github.com/bernardofernandezz/ani-br/pkg/norm"
)

type Cache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V, ttl time.Duration)
}

type SearchKey struct {
	Query string
	Lang  domain.Language
}

type Service struct {
	repo  domain.AnimeRepository
	cache Cache[SearchKey, []domain.Anime]
	ttl   time.Duration
}

func New(repo domain.AnimeRepository, cache Cache[SearchKey, []domain.Anime], ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Service{
		repo:  repo,
		cache: cache,
		ttl:   ttl,
	}
}

func (s *Service) Execute(ctx context.Context, query string, lang domain.Language) ([]domain.Anime, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, domain.ErrAnimeNotFound
	}

	key := SearchKey{Query: norm.NormalizeTitle(query), Lang: lang}
	if s.cache != nil {
		if cached, ok := s.cache.Get(key); ok {
			return cached, nil
		}
	}

	animes, err := s.repo.SearchAnime(ctx, query, lang)
	if err != nil {
		if errors.Is(err, domain.ErrAnimeNotFound) {
			return nil, domain.ErrAnimeNotFound
		}
		return nil, err
	}

	if s.cache != nil {
		s.cache.Set(key, animes, s.ttl)
	}
	return animes, nil
}

