package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bernardofernandezz/ani-br/internal/domain"
)

type HTTPGetter interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type Service struct {
	http       HTTPGetter
	maxWorkers int
}

func New(http HTTPGetter, maxWorkers int) *Service {
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	return &Service{http: http, maxWorkers: maxWorkers}
}

func (s *Service) DownloadEpisode(ctx context.Context, ep domain.Episode, lang domain.Language, q domain.Quality, destDir string) (string, error) {
	url, err := pickStreamURL(ep.Streams, lang, q)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	filename := sanitize(fmt.Sprintf("%s-ep-%02d.mp4", ep.AnimeID, ep.EpisodeNumber))
	out := filepath.Join(destDir, filename)

	resp, err := s.http.Get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download: status %d", resp.StatusCode)
	}

	tmp := out + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}

	if err := os.Rename(tmp, out); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return out, nil
}

func (s *Service) DownloadBatch(ctx context.Context, episodes []domain.Episode, lang domain.Language, q domain.Quality, destDir string) ([]string, error) {
	sem := make(chan struct{}, s.maxWorkers)

	type res struct {
		path string
		err  error
	}

	out := make([]string, 0, len(episodes))
	results := make(chan res, len(episodes))

	var wg sync.WaitGroup
	for _, ep := range episodes {
		ep := ep
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				results <- res{err: ctx.Err()}
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			p, err := s.DownloadEpisode(ctx, ep, lang, q, destDir)
			results <- res{path: p, err: err}
		}()
	}

	wg.Wait()
	close(results)

	var firstErr error
	for r := range results {
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		if r.path != "" {
			out = append(out, r.path)
		}
	}
	return out, firstErr
}

func pickStreamURL(streams []domain.Stream, lang domain.Language, q domain.Quality) (string, error) {
	if len(streams) == 0 {
		return "", domain.ErrStreamUnavailable
	}

	for _, s := range streams {
		if s.URL == "" {
			continue
		}
		if lang != "" && s.Language != lang {
			continue
		}
		if q != "" && q != domain.QualityAuto && s.Quality != q {
			continue
		}
		return s.URL, nil
	}
	for _, s := range streams {
		if s.URL == "" {
			continue
		}
		if lang != "" && s.Language != lang {
			continue
		}
		return s.URL, nil
	}
	for _, s := range streams {
		if s.URL != "" {
			return s.URL, nil
		}
	}
	return "", domain.ErrStreamUnavailable
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Sprintf("download-%d.mp4", time.Now().UnixNano())
	}
	repl := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-",
	)
	return repl.Replace(s)
}

var ErrDownloadCancelled = errors.New("download cancelado")

