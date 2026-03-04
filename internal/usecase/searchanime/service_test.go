package searchanime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bernardofernandezz/ani-br/internal/domain"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	mu    sync.Mutex
	calls int
	resp  []domain.Anime
	err   error
}

func (r *fakeRepo) SearchAnime(_ context.Context, _ string, _ domain.Language) ([]domain.Anime, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return r.resp, nil
}

func (r *fakeRepo) GetEpisodes(_ context.Context, _ string) ([]domain.Episode, error) {
	return nil, domain.ErrEpisodeNotFound
}

func (r *fakeRepo) Calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type fakeCache[K comparable, V any] struct {
	mu    sync.Mutex
	items map[K]V
}

func newFakeCache[K comparable, V any]() *fakeCache[K, V] {
	return &fakeCache[K, V]{items: make(map[K]V)}
}

func (c *fakeCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.items[key]
	return v, ok
}

func (c *fakeCache[K, V]) Set(key K, value V, _ time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = value
}

func TestService_Execute_CacheHitSkipsRepo(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		resp: []domain.Anime{{ID: "1", Title: "Naruto", Provider: "p"}},
	}
	cache := newFakeCache[SearchKey, []domain.Anime]()
	svc := New(repo, cache, 10*time.Second)

	key := SearchKey{Query: "naruto", Lang: domain.LanguagePTBRDub}
	cache.Set(key, []domain.Anime{{ID: "cached", Title: "Naruto", Provider: "p"}}, time.Minute)

	got, err := svc.Execute(context.Background(), "Naruto", domain.LanguagePTBRDub)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "cached", got[0].ID)
	require.Equal(t, 0, repo.Calls())
}

func TestService_Execute_CacheMissCallsRepoAndStores(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		resp: []domain.Anime{{ID: "1", Title: "Naruto", Provider: "p"}},
	}
	cache := newFakeCache[SearchKey, []domain.Anime]()
	svc := New(repo, cache, 10*time.Second)

	got, err := svc.Execute(context.Background(), "Naruto", domain.LanguagePTBRSub)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 1, repo.Calls())

	key := SearchKey{Query: "naruto", Lang: domain.LanguagePTBRSub}
	cached, ok := cache.Get(key)
	require.True(t, ok)
	require.Len(t, cached, 1)
	require.Equal(t, "1", cached[0].ID)
}

func TestService_Execute_EmptyQueryReturnsNotFound(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{resp: []domain.Anime{{ID: "1", Title: "X", Provider: "p"}}}
	svc := New(repo, nil, time.Minute)

	_, err := svc.Execute(context.Background(), "   ", domain.LanguagePTBRDub)
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrAnimeNotFound))
	require.Equal(t, 0, repo.Calls())
}

func TestService_Execute_RepoNotFoundIsSentinel(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{err: domain.ErrAnimeNotFound}
	svc := New(repo, nil, time.Minute)

	_, err := svc.Execute(context.Background(), "qualquer", domain.LanguagePTBRDub)
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrAnimeNotFound))
	require.Equal(t, 1, repo.Calls())
}

