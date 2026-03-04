package animesonline

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bernardofernandezz/ani-br/internal/domain"
	"github.com/bernardofernandezz/ani-br/pkg/httputil"
)

const providerID = domain.ProviderID("animesonline")

// Provider implementa AnimeRepository para o site AnimesOnline.
// baseURL e userAgent vêm da config (Viper); se userAgent for vazio, usa httputil.DefaultUserAgent().
type Provider struct {
	client    *httputil.Client
	baseURL   *url.URL
	userAgent string
}

func New(client *httputil.Client, rawBaseURL, userAgent string) (*Provider, error) {
	if client == nil {
		client = httputil.NewClient(httputil.DefaultConfig())
	}
	if rawBaseURL == "" {
		rawBaseURL = "https://animesonlinecc.to"
	}
	u, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("animesonline: baseURL inválida: %w", err)
	}
	if userAgent == "" {
		userAgent = httputil.DefaultUserAgent()
	}
	return &Provider{
		client:    client,
		baseURL:   u,
		userAgent: userAgent,
	}, nil
}

func (p *Provider) SearchAnime(ctx context.Context, query string, lang domain.Language) ([]domain.Anime, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, domain.ErrAnimeNotFound
	}

	// animesonlinecc.to e similares: busca por ?s= (WordPress) ou /buscar/
	searchURL := *p.baseURL
	searchURL.Path = "/"
	params := searchURL.Query()
	params.Set("s", q)
	searchURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Referer", p.baseURL.String())

	resp, err := p.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("animesonline: status %d na busca", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var animes []domain.Anime
	now := time.Now()
	seenHref := make(map[string]struct{})

	// animesonlinecc.to: artigos com link para o anime (article > a ou h2 > a, .item > a, etc.)
	trySelector := func(sel string, linkSel string) {
		doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
			link := s.Find(linkSel).First()
			if link.Length() == 0 {
				link = s.Find("a").First()
			}
			href, ok := link.Attr("href")
			if !ok {
				return
			}
			href = strings.TrimSpace(href)
			if href == "" || href == "#" {
				return
			}
			animeURL := p.resolveURL(href)
			if _, dup := seenHref[animeURL]; dup {
				return
			}
			// Ignorar links que são claramente de episódio, não de anime.
			if strings.Contains(href, "/episodio/") || strings.Contains(href, "/episode/") {
				return
			}
			title := strings.TrimSpace(link.Text())
			if title == "" {
				title = strings.TrimSpace(s.Find("h2").Text())
			}
			if title == "" {
				title = strings.TrimSpace(s.Find(".title").Text())
			}
			if title == "" {
				title = strings.TrimSpace(s.Find("h3").Text())
			}
			if title == "" {
				return
			}
			// Título muito curto ou só número = provável episódio
			if len(title) < 2 {
				return
			}
			seenHref[animeURL] = struct{}{}

			a := domain.Anime{
				ID:            animeURL,
				Title:         title,
				Provider:      providerID,
				PreferredLang: lang,
				LastUpdatedAt: now,
			}
			txt := strings.ToLower(s.Text())
			if strings.Contains(txt, "dublado") || strings.Contains(txt, "dub") {
				a.PreferredLang = domain.LanguagePTBRDub
			} else if strings.Contains(txt, "legendado") || strings.Contains(txt, "leg") || strings.Contains(txt, "sub") {
				a.PreferredLang = domain.LanguagePTBRSub
			}
			switch lang {
			case domain.LanguagePTBRDub:
				if a.PreferredLang != domain.LanguagePTBRDub {
					return
				}
			case domain.LanguagePTBRSub:
				if a.PreferredLang != domain.LanguagePTBRSub {
					return
				}
			}
			animes = append(animes, a)
		})
	}
	trySelector("article a[href*='anime']", "a")
	trySelector("article", "a")
	trySelector(".item", "a")
	trySelector(".anime-item", "a")
	trySelector("div.post a[href*='anime']", "a")
	if len(animes) == 0 {
		// Fallback: qualquer article com link e título
		doc.Find("article").Each(func(_ int, s *goquery.Selection) {
			link := s.Find("a").First()
			href, ok := link.Attr("href")
			if !ok {
				return
			}
			animeURL := p.resolveURL(href)
			if _, dup := seenHref[animeURL]; dup {
				return
			}
			if strings.Contains(href, "/episodio/") || strings.Contains(href, "/episode/") {
				return
			}
			title := strings.TrimSpace(link.Text())
			if title == "" {
				title = strings.TrimSpace(s.Find("h2").Text())
			}
			if title == "" || len(title) < 2 {
				return
			}
			seenHref[animeURL] = struct{}{}
			a := domain.Anime{
				ID:            animeURL,
				Title:         title,
				Provider:      providerID,
				PreferredLang: lang,
				LastUpdatedAt: now,
			}
			txt := strings.ToLower(s.Text())
			if strings.Contains(txt, "dublado") || strings.Contains(txt, "dub") {
				a.PreferredLang = domain.LanguagePTBRDub
			} else if strings.Contains(txt, "legendado") || strings.Contains(txt, "leg") {
				a.PreferredLang = domain.LanguagePTBRSub
			}
			switch lang {
			case domain.LanguagePTBRDub:
				if a.PreferredLang != domain.LanguagePTBRDub {
					return
				}
			case domain.LanguagePTBRSub:
				if a.PreferredLang != domain.LanguagePTBRSub {
					return
				}
			}
			animes = append(animes, a)
		})
	}

	if len(animes) == 0 {
		return nil, domain.ErrAnimeNotFound
	}

	return animes, nil
}

func (p *Provider) GetEpisodes(ctx context.Context, animeID string) ([]domain.Episode, error) {
	if animeID == "" {
		return nil, domain.ErrEpisodeNotFound
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, animeID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Referer", p.baseURL.String())

	resp, err := p.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("animesonline: status %d na página do anime", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Coletar todos os links que parecem ser de episódio; depois deduplicar por número.
	type epCandidate struct {
		num      int
		title    string
		playerURL string
	}
	var candidates []epCandidate

	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		hrefLower := strings.ToLower(href)
		isEpisodeLink := strings.Contains(hrefLower, "episodio") ||
			strings.Contains(hrefLower, "episode") ||
			strings.Contains(hrefLower, "/ep/") ||
			(reEpisodeNumber.MatchString(href) && (strings.Contains(hrefLower, "anime") || strings.Contains(hrefLower, "ver")))

		if !isEpisodeLink {
			return
		}

		title := strings.TrimSpace(s.Text())
		num := parseEpisodeNumber(title)
		if num == 0 {
			num = parseEpisodeNumberFromURL(href)
		}
		if num == 0 {
			return
		}

		playerURL := p.resolveURL(href)
		candidates = append(candidates, epCandidate{num: num, title: title, playerURL: playerURL})
	})

	// Deduplicar por número de episódio (manter o primeiro de cada número para ordem consistente).
	byNum := make(map[int]epCandidate)
	for _, c := range candidates {
		if _, ok := byNum[c.num]; !ok {
			byNum[c.num] = c
		}
	}
	if len(byNum) == 0 {
		return nil, domain.ErrEpisodeNotFound
	}

	nums := make([]int, 0, len(byNum))
	for n := range byNum {
		nums = append(nums, n)
	}
	sort.Ints(nums)

	eps := make([]domain.Episode, 0, len(nums))
	for _, n := range nums {
		c := byNum[n]
		title := c.title
		if title == "" {
			title = "Episódio " + fmt.Sprint(n)
		}
		eps = append(eps, domain.Episode{
			ID:            c.playerURL,
			AnimeID:       animeID,
			SeasonNumber:  1,
			EpisodeNumber: n,
			Title:         title,
		})
	}
	return eps, nil
}

// ResolveStream extrai a URL de vídeo e monta um domain.Stream com headers para o mpv.
func (p *Provider) ResolveStream(ctx context.Context, playerPageURL string) (domain.Stream, error) {
	urlStr, err := ResolveStreamURL(ctx, p.client, playerPageURL)
	if err != nil {
		return domain.Stream{}, err
	}
	referer := p.baseURL.String()
	if referer != "" && referer[len(referer)-1] != '/' {
		referer += "/"
	}
	return domain.Stream{
		URL:      urlStr,
		Quality:  domain.QualityAuto,
		Language: domain.LanguagePTBRDub,
		Type:     domain.StreamTypeHLS,
		Provider: providerID,
		Headers: map[string]string{
			"User-Agent": p.userAgent,
			"Referer":    referer,
		},
	}, nil
}

// ResolveStreamURL extrai a URL final de vídeo (.m3u8/.mp4) da página do player.
func ResolveStreamURL(ctx context.Context, client *httputil.Client, playerPageURL string) (string, error) {
	if client == nil {
		client = httputil.NewClient(httputil.DefaultConfig())
	}
	if playerPageURL == "" {
		return "", errors.New("player URL vazia")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, playerPageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", httputil.DefaultUserAgent())
	// Referer: página do anime/site principal é geralmente exigida para 200/403.
	if u, err := url.Parse(playerPageURL); err == nil {
		req.Header.Set("Referer", u.Scheme+"://"+u.Host+"/")
	}

	resp, err := client.Do(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("animesonline: status %d na página do player", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	html := string(body)

	extract := func(pattern string, raw string) string {
		re := regexp.MustCompile(pattern)
		m := re.FindStringSubmatch(raw)
		if len(m) != 2 {
			return ""
		}
		return strings.TrimSpace(unescapeJS(m[1]))
	}

	// window._video_url = "https://...m3u8"
	if u := extract(`(?i)window\._video_url\s*=\s*["']([^"']+)["']`, html); u != "" && isVideoURL(u) {
		return u, nil
	}
	// "file":"https://...m3u8" (JSON embutido)
	if u := extract(`["']file["']\s*:\s*["']([^"']+)["']`, html); u != "" && isVideoURL(u) {
		return u, nil
	}
	// jwplayer / source: file: "url"
	if u := extract(`(?i)(?:file|src)\s*:\s*["']([^"']+\.(?:m3u8|mp4)[^"']*)["']`, html); u != "" {
		return u, nil
	}
	// <source src="...m3u8">
	if u := extract(`<source[^>]+src\s*=\s*["']([^"']+)["']`, html); u != "" && isVideoURL(u) {
		return u, nil
	}
	// data-src="...m3u8"
	if u := extract(`data-src\s*=\s*["']([^"']+\.(?:m3u8|mp4))["']`, html); u != "" {
		return u, nil
	}
	// atob("Base64")
	reB64 := regexp.MustCompile(`atob\s*\(\s*["']([A-Za-z0-9+/=]+)["']\s*\)`)
	if m := reB64.FindStringSubmatch(html); len(m) == 2 {
		decoded, decErr := base64.StdEncoding.DecodeString(m[1])
		if decErr == nil {
			u := strings.TrimSpace(string(decoded))
			if isVideoURL(u) {
				return u, nil
			}
		}
	}

	return "", errors.New("animesonline: não foi possível extrair URL de vídeo")
}

func isVideoURL(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(s, "http") && (strings.Contains(s, ".m3u8") || strings.Contains(s, ".mp4") || strings.Contains(s, "video"))
}

func (p *Provider) resolveURL(href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	u := *p.baseURL
	u.Path = strings.TrimLeft(href, "/")
	return u.String()
}

var reEpisodeNumber = regexp.MustCompile(`(?i)(\d+)`)
var reEpisodeInURL = regexp.MustCompile(`(?:episodio|episode|ep)[/\-_]*(?:numero)?[\-_]*(\d+)`)

func parseEpisodeNumber(s string) int {
	match := reEpisodeNumber.FindStringSubmatch(s)
	if len(match) < 2 {
		return 0
	}
	var n int
	fmt.Sscanf(match[1], "%d", &n)
	return n
}

func parseEpisodeNumberFromURL(u string) int {
	m := reEpisodeInURL.FindStringSubmatch(strings.ToLower(u))
	if len(m) < 2 {
		return 0
	}
	var n int
	fmt.Sscanf(m[1], "%d", &n)
	return n
}

func unescapeJS(s string) string {
	s = strings.ReplaceAll(s, `\/`, `/`)
	s = strings.ReplaceAll(s, `\u002F`, `/`)
	return s
}

