package provider

import (
	"context"
	"sync"
	"time"

	"github.com/bernardofernandezz/ani-br/internal/domain"
	"github.com/bernardofernandezz/ani-br/pkg/norm"
	"golang.org/x/sync/errgroup"
)

// Registry implementa AnimeRepository fazendo fanout em múltiplos providers.
type Registry struct {
	providers []domain.AnimeRepository
	timeout   time.Duration
}

// NewRegistry cria um novo registry com providers em ordem de prioridade.
func NewRegistry(timeout time.Duration, providers ...domain.AnimeRepository) *Registry {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Registry{
		providers: providers,
		timeout:   timeout,
	}
}

// SearchAnime faz busca concorrente em todos os providers e deduplica resultados.
func (r *Registry) SearchAnime(ctx context.Context, query string, lang domain.Language) ([]domain.Anime, error) {
	if len(r.providers) == 0 {
		return nil, domain.ErrAnimeNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var (
		mu       sync.Mutex
		results  []domain.Anime
		seen     = make(map[string]struct{})
	)

	g, gctx := errgroup.WithContext(ctx)

	for _, p := range r.providers {
		p := p
		g.Go(func() error {
			animes, err := p.SearchAnime(gctx, query, lang)
			if err != nil {
				// Em fanout, erros individuais não derrubam os demais providers.
				return nil
			}

			mu.Lock()
			defer mu.Unlock()

			for _, a := range animes {
				key := norm.NormalizeTitle(a.Title)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				results = append(results, a)
			}
			return nil
		})
	}

	_ = g.Wait()

	if len(results) == 0 {
		return nil, domain.ErrAnimeNotFound
	}

	return results, nil
}

// GetEpisodes tenta obter episódios a partir dos providers em ordem de prioridade.
func (r *Registry) GetEpisodes(ctx context.Context, animeID string) ([]domain.Episode, error) {
	if len(r.providers) == 0 {
		return nil, domain.ErrEpisodeNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	for _, p := range r.providers {
		episodes, err := p.GetEpisodes(ctx, animeID)
		if err == nil && len(episodes) > 0 {
			return episodes, nil
		}
	}

	return nil, domain.ErrEpisodeNotFound
}

