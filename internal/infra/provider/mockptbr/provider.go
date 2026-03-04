package mockptbr

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bernardofernandezz/ani-br/internal/domain"
	"github.com/bernardofernandezz/ani-br/pkg/httputil"
	"github.com/bernardofernandezz/ani-br/pkg/robots"
)

// Provider implementa um provider PT-BR de exemplo para desenvolvimento e testes.
type Provider struct {
	id      domain.ProviderID
	baseURL *url.URL
	client  *httputil.Client
	robots  *robots.Checker
	randSrc *rand.Rand
}

// New cria um novo provider mock PT-BR.
func New(baseURL string, client *httputil.Client, robotsChecker *robots.Checker) (*Provider, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("mockptbr: baseURL inválida: %w", err)
	}
	if client == nil {
		client = httputil.NewClient(httputil.DefaultConfig())
	}
	if robotsChecker == nil {
		robotsChecker = robots.NewChecker(nil)
	}

	return &Provider{
		id:      domain.ProviderID("mock-ptbr"),
		baseURL: u,
		client:  client,
		robots:  robotsChecker,
		randSrc: rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func (p *Provider) ID() domain.ProviderID {
	return p.id
}

// SearchAnime implementa a busca de animes na página /search.
func (p *Provider) SearchAnime(ctx context.Context, query string, lang domain.Language) ([]domain.Anime, error) {
	searchURL := *p.baseURL
	searchURL.Path = "/search"
	q := searchURL.Query()
	q.Set("q", query)
	searchURL.RawQuery = q.Encode()

	if !p.robots.Allowed(ctx, searchURL.String()) {
		return nil, domain.ErrAnimeNotFound
	}

	if err := p.randomDelay(ctx); err != nil {
		return nil, err
	}

	resp, err := p.client.Get(ctx, searchURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mockptbr: status inesperado %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var result []domain.Anime
	doc.Find(".anime").Each(func(_ int, s *goquery.Selection) {
		title := s.Find(".title").Text()
		href, ok := s.Find("a").Attr("href")
		if !ok {
			return
		}

		a := domain.Anime{
			ID:            href,
			Title:         title,
			Provider:      p.id,
			PreferredLang: lang,
		}
		result = append(result, a)
	})

	return result, nil
}

// GetEpisodes lê episódios de uma página HTML simples.
func (p *Provider) GetEpisodes(ctx context.Context, animeID string) ([]domain.Episode, error) {
	episodesURL := *p.baseURL
	episodesURL.Path = animeID

	if !p.robots.Allowed(ctx, episodesURL.String()) {
		return nil, domain.ErrEpisodeNotFound
	}

	if err := p.randomDelay(ctx); err != nil {
		return nil, err
	}

	resp, err := p.client.Get(ctx, episodesURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mockptbr: status inesperado %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var episodes []domain.Episode
	doc.Find(".episode").Each(func(i int, s *goquery.Selection) {
		epID, ok := s.Attr("data-id")
		if !ok {
			return
		}
		title := s.Find(".title").Text()
		episodes = append(episodes, domain.Episode{
			ID:            epID,
			AnimeID:       animeID,
			EpisodeNumber: i + 1,
			Title:         title,
		})
	})

	if len(episodes) == 0 {
		return nil, domain.ErrEpisodeNotFound
	}
	return episodes, nil
}

func (p *Provider) randomDelay(ctx context.Context) error {
	// Delay aleatório entre 100–500ms.
	ms := p.randSrc.Intn(401) + 100
	d := time.Duration(ms) * time.Millisecond

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

