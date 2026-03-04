package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bernardofernandezz/ani-br/internal/domain"
	"github.com/bernardofernandezz/ani-br/pkg/httputil"
)

const (
	apiURL     = "https://graphql.anilist.co"
	providerID = domain.ProviderID("anilist")
)

type Provider struct {
	client *httputil.Client
}

func New(client *httputil.Client) *Provider {
	if client == nil {
		client = httputil.NewClient(httputil.DefaultConfig())
	}
	return &Provider{client: client}
}

// SearchAnime usa a API pública do AniList para buscar animes por título.
func (p *Provider) SearchAnime(ctx context.Context, query string, _ domain.Language) ([]domain.Anime, error) {
	payload := map[string]any{
		"query": searchQuery,
		"variables": map[string]any{
			"search":  query,
			"page":    1,
			"perPage": 25,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("anilist: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var out struct {
		Data struct {
			Page struct {
				Media []struct {
					ID          int    `json:"id"`
					Title       struct {
						Romaji  string `json:"romaji"`
						English string `json:"english"`
						Native  string `json:"native"`
					} `json:"title"`
					Description string `json:"description"`
					Episodes    int    `json:"episodes"`
				} `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	now := time.Now()
	animes := make([]domain.Anime, 0, len(out.Data.Page.Media))
	for _, m := range out.Data.Page.Media {
		title := pickTitle(m.Title)
		if title == "" {
			continue
		}
		animes = append(animes, domain.Anime{
			ID:              strconv.Itoa(m.ID),
			Title:           title,
			Synopsis:        stripHTML(m.Description),
			Provider:        providerID,
			PreferredLang:   domain.LanguagePTBRDub,
			TotalEpisodes:   m.Episodes,
			LastUpdatedAt:   now,
			ProviderMetadata: map[string]any{"anilist_id": m.ID},
		})
	}

	if len(animes) == 0 {
		return nil, domain.ErrAnimeNotFound
	}
	return animes, nil
}

// GetEpisodes retorna uma lista sequencial de episódios baseada no número total.
func (p *Provider) GetEpisodes(ctx context.Context, animeID string) ([]domain.Episode, error) {
	id, err := strconv.Atoi(animeID)
	if err != nil {
		return nil, domain.ErrEpisodeNotFound
	}

	payload := map[string]any{
		"query": episodesQuery,
		"variables": map[string]any{
			"id": id,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("anilist: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var out struct {
		Data struct {
			Media struct {
				Episodes int `json:"episodes"`
			} `json:"Media"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	total := out.Data.Media.Episodes
	if total <= 0 {
		return nil, domain.ErrEpisodeNotFound
	}

	// Para demonstração de streaming local, associamos cada episódio a um stream HLS público
	// de teste. Em providers reais, isso seria substituído pela extração de URLs de um
	// site de streaming, como feito em projetos como GoAnime ou ani-cli.
	const sampleHLS = "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8"

	episodes := make([]domain.Episode, 0, total)
	for i := 1; i <= total; i++ {
		episodes = append(episodes, domain.Episode{
			ID:            fmt.Sprintf("%d-%02d", id, i),
			AnimeID:       animeID,
			SeasonNumber:  1,
			EpisodeNumber: i,
			Title:         fmt.Sprintf("Episódio %02d", i),
			Streams: []domain.Stream{
				{
					URL:      sampleHLS,
					Quality:  domain.Quality720p,
					Language: domain.LanguagePTBRDub,
					Type:     domain.StreamTypeHLS,
					Provider: providerID,
				},
			},
		})
	}
	return episodes, nil
}

func pickTitle(t struct {
	Romaji  string `json:"romaji"`
	English string `json:"english"`
	Native  string `json:"native"`
}) string {
	if t.English != "" {
		return t.English
	}
	if t.Romaji != "" {
		return t.Romaji
	}
	return t.Native
}

func stripHTML(s string) string {
	// AniList description vem com tags HTML simples; remove algumas mais comuns.
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<i>", "")
	s = strings.ReplaceAll(s, "</i>", "")
	s = strings.ReplaceAll(s, "<b>", "")
	s = strings.ReplaceAll(s, "</b>", "")
	return s
}

const searchQuery = `
query ($search: String, $page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    media(search: $search, type: ANIME) {
      id
      title {
        romaji
        english
        native
      }
      description(asHtml: false)
      episodes
    }
  }
}
`

const episodesQuery = `
query ($id: Int) {
  Media(id: $id, type: ANIME) {
    episodes
  }
}
`

