package playepisode

import (
	"context"
	"errors"
	"testing"

	"github.com/bernardofernandezz/ani-br/internal/domain"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	episodes []domain.Episode
	err      error
}

func (f *fakeRepo) SearchAnime(_ context.Context, _ string, _ domain.Language) ([]domain.Anime, error) {
	return nil, nil
}

func (f *fakeRepo) GetEpisodes(_ context.Context, _ string) ([]domain.Episode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.episodes, nil
}

type fakePlayer struct {
	lastEp *domain.Episode
	err    error
}

func (f *fakePlayer) PlayEpisode(_ context.Context, ep domain.Episode, _ domain.Language, _ domain.Quality) error {
	f.lastEp = &ep
	return f.err
}

type fakeResolver struct {
	stream domain.Stream
	err    error
}

func (f *fakeResolver) ResolveStream(_ context.Context, _ string) (domain.Stream, error) {
	if f.err != nil {
		return domain.Stream{}, f.err
	}
	return f.stream, nil
}

func TestPlayByNumber_EpisodeNotFound(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{episodes: nil, err: domain.ErrEpisodeNotFound}
	svc := New(repo, &fakePlayer{}, nil)
	err := svc.PlayByNumber(context.Background(), "any", 1, domain.LanguagePTBRDub, domain.QualityAuto)
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrEpisodeNotFound))
}

func TestPlayByNumber_NumberNotInList(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{episodes: []domain.Episode{
		{ID: "e1", EpisodeNumber: 1, Title: "Ep 1"},
	}}
	svc := New(repo, &fakePlayer{}, nil)
	err := svc.PlayByNumber(context.Background(), "anime", 99, domain.LanguagePTBRDub, domain.QualityAuto)
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrEpisodeNotFound))
}

func TestPlayByNumber_WithStreams_NoResolverCalled(t *testing.T) {
	t.Parallel()
	ep := domain.Episode{
		ID:            "e1",
		EpisodeNumber: 1,
		Title:         "Ep 1",
		Streams:       []domain.Stream{{URL: "https://example.com/v.m3u8"}},
	}
	repo := &fakeRepo{episodes: []domain.Episode{ep}}
	player := &fakePlayer{}
	svc := New(repo, player, &fakeResolver{err: errors.New("resolver should not be called")})
	err := svc.PlayByNumber(context.Background(), "anime", 1, domain.LanguagePTBRDub, domain.QualityAuto)
	require.NoError(t, err)
	require.NotNil(t, player.lastEp)
	require.Len(t, player.lastEp.Streams, 1)
	require.Equal(t, "https://example.com/v.m3u8", player.lastEp.Streams[0].URL)
}

func TestPlayByNumber_NoStreams_ResolverEnriches(t *testing.T) {
	t.Parallel()
	ep := domain.Episode{
		ID:            "https://animesonline.site/player/123",
		EpisodeNumber: 1,
		Title:         "Ep 1",
		Streams:       nil,
	}
	repo := &fakeRepo{episodes: []domain.Episode{ep}}
	player := &fakePlayer{}
	resolver := &fakeResolver{stream: domain.Stream{
		URL:     "https://cdn.example.com/ep1.m3u8",
		Quality: domain.Quality720p,
		Headers: map[string]string{"Referer": "https://animesonline.site/"},
	}}
	svc := New(repo, player, resolver)
	err := svc.PlayByNumber(context.Background(), "anime", 1, domain.LanguagePTBRSub, domain.QualityAuto)
	require.NoError(t, err)
	require.NotNil(t, player.lastEp)
	require.Len(t, player.lastEp.Streams, 1)
	require.Equal(t, "https://cdn.example.com/ep1.m3u8", player.lastEp.Streams[0].URL)
	require.Equal(t, "https://animesonline.site/", player.lastEp.Streams[0].Headers["Referer"])
}

func TestPlayByNumber_ResolverReturnsError(t *testing.T) {
	t.Parallel()
	ep := domain.Episode{ID: "player-url", EpisodeNumber: 1, Streams: nil}
	repo := &fakeRepo{episodes: []domain.Episode{ep}}
	resolver := &fakeResolver{err: errors.New("resolve failed")}
	svc := New(repo, &fakePlayer{}, resolver)
	err := svc.PlayByNumber(context.Background(), "anime", 1, domain.LanguagePTBRDub, domain.QualityAuto)
	require.Error(t, err)
	require.Contains(t, err.Error(), "resolve failed")
}
