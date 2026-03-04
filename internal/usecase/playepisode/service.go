package playepisode

import (
	"context"

	"github.com/bernardofernandezz/ani-br/internal/domain"
)

type Service struct {
	repo           domain.AnimeRepository
	player         domain.PlayerPort
	streamResolver domain.StreamResolver
}

func New(repo domain.AnimeRepository, player domain.PlayerPort, streamResolver domain.StreamResolver) *Service {
	return &Service{
		repo:           repo,
		player:         player,
		streamResolver: streamResolver,
	}
}

func (s *Service) PlayByNumber(ctx context.Context, animeID string, episodeNumber int, lang domain.Language, q domain.Quality) error {
	episodes, err := s.repo.GetEpisodes(ctx, animeID)
	if err != nil {
		return err
	}
	for _, ep := range episodes {
		if ep.EpisodeNumber != episodeNumber {
			continue
		}
		// Se o episódio não tem streams (ex.: AnimesOnline guarda URL do player em ID), resolver.
		if len(ep.Streams) == 0 && s.streamResolver != nil {
			stream, resolveErr := s.streamResolver.ResolveStream(ctx, ep.ID)
			if resolveErr != nil {
				return resolveErr
			}
			if stream.Language == "" {
				stream.Language = lang
			}
			ep.Streams = []domain.Stream{stream}
		}
		return s.player.PlayEpisode(ctx, ep, lang, q)
	}
	return domain.ErrEpisodeNotFound
}

