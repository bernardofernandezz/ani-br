package httputil

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Config define parâmetros de tuning do cliente HTTP usado para scraping.
type Config struct {
	// Tempo máximo total por requisição.
	Timeout time.Duration

	// Número máximo de tentativas (incluindo a primeira).
	MaxRetries int

	// Delay base para backoff exponencial.
	BaseBackoff time.Duration

	// Jitter máximo adicionado ao backoff.
	MaxJitter time.Duration

	// Limite de requisições por segundo por domínio.
	RequestsPerSecond float64

	// Burst máximo por domínio.
	Burst int

	// Pooling de conexões.
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
}

// DefaultConfig retorna uma configuração razoável para scraping.
func DefaultConfig() Config {
	return Config{
		Timeout:           20 * time.Second,
		MaxRetries:        3,
		BaseBackoff:       200 * time.Millisecond,
		MaxJitter:         150 * time.Millisecond,
		RequestsPerSecond: 4,
		Burst:             4,
		MaxIdleConns:      64,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     30 * time.Second,
	}
}

// Client é um cliente HTTP com pooling, retry, rate limiting e rotação de UA.
type Client struct {
	cfg         Config
	httpClient  *http.Client
	transport   *http.Transport
	rateLimiters map[string]*rate.Limiter
	mu          sync.RWMutex
	userAgents  []string
	randSrc     *rand.Rand
}

// NewClient cria um novo cliente HTTP configurado.
func NewClient(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultConfig().Timeout
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = DefaultConfig().MaxRetries
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = DefaultConfig().BaseBackoff
	}
	if cfg.MaxJitter <= 0 {
		cfg.MaxJitter = DefaultConfig().MaxJitter
	}
	if cfg.RequestsPerSecond <= 0 {
		cfg.RequestsPerSecond = DefaultConfig().RequestsPerSecond
	}
	if cfg.Burst <= 0 {
		cfg.Burst = DefaultConfig().Burst
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = DefaultConfig().MaxIdleConns
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = DefaultConfig().MaxIdleConnsPerHost
	}
	if cfg.IdleConnTimeout <= 0 {
		cfg.IdleConnTimeout = DefaultConfig().IdleConnTimeout
	}

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	return &Client{
		cfg:         cfg,
		httpClient:  httpClient,
		transport:   transport,
		rateLimiters: make(map[string]*rate.Limiter),
		userAgents:  defaultUserAgents(),
		randSrc:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Do executa uma requisição HTTP com retry, rate limiting e UA rotation.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	if host == "" {
		return nil, fmt.Errorf("httputil: host vazio em URL %q", req.URL.String())
	}

	if err := c.acquireTokenForDomain(ctx, host); err != nil {
		return nil, err
	}

	// Rotaciona User-Agent por requisição.
	req = req.Clone(ctx)
	if ua := c.pickUserAgent(); ua != "" {
		req.Header.Set("User-Agent", ua)
	}

	var lastErr error
	for attempt := 0; attempt < c.maxRetries(); attempt++ {
		resp, err := c.httpClient.Do(req)
		if err == nil && !shouldRetry(resp.StatusCode, nil) {
			return resp, nil
		}

		if err != nil {
			// Decide se erro é transitório.
			if !shouldRetry(0, err) {
				return nil, err
			}
			lastErr = err
		} else {
			// Temos response com status de retry.
			lastErr = fmt.Errorf("httputil: status %d", resp.StatusCode)
			_ = resp.Body.Close()
		}

		// Respeita contexto e backoff.
		backoff := c.backoffDuration(attempt)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	if lastErr == nil {
		lastErr = errors.New("httputil: falha desconhecida após retries")
	}
	return nil, lastErr
}

// Get é um helper para requisições GET simples.
func (c *Client) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req)
}

func (c *Client) acquireTokenForDomain(ctx context.Context, host string) error {
	limiter := c.limiterForHost(host)
	if limiter == nil {
		return nil
	}
	if err := limiter.Wait(ctx); err != nil {
		return err
	}
	return nil
}

func (c *Client) limiterForHost(host string) *rate.Limiter {
	c.mu.RLock()
	lim, ok := c.rateLimiters[host]
	c.mu.RUnlock()
	if ok {
		return lim
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	lim, ok = c.rateLimiters[host]
	if ok {
		return lim
	}
	lim = rate.NewLimiter(rate.Limit(c.cfg.RequestsPerSecond), c.cfg.Burst)
	c.rateLimiters[host] = lim
	return lim
}

func (c *Client) backoffDuration(attempt int) time.Duration {
	base := c.cfg.BaseBackoff
	jitter := time.Duration(c.randSrc.Int63n(int64(c.cfg.MaxJitter)))
	return base*time.Duration(1<<attempt) + jitter
}

func (c *Client) maxRetries() int {
	return c.cfg.MaxRetries
}

func shouldRetry(status int, err error) bool {
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Temporary() {
			return true
		}
		// Outros erros de rede genéricos.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return true
		}
		return false
	}

	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (c *Client) pickUserAgent() string {
	if len(c.userAgents) == 0 {
		return ""
	}
	idx := c.randSrc.Intn(len(c.userAgents))
	return c.userAgents[idx]
}

// DefaultUserAgent retorna um User-Agent de navegador padrão para scraping.
// Usado por providers (ex.: AnimesOnline) quando providers.user_agent não está definido na config.
func DefaultUserAgent() string {
	return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
}

func defaultUserAgents() []string {
	return []string{
		DefaultUserAgent(),
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	}
}

