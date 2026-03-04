package robots

import (
	"bufio"
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Checker implementa uma verificação simples de robots.txt focada em User-agent: *.
type Checker struct {
	client *http.Client

	mu     sync.RWMutex
	rules  map[string]*rules // host -> rules
}

type rules struct {
	// caminhos iniciando com prefixos proibidos.
	disallow []string
}

// NewChecker cria um novo Checker com client opcional.
func NewChecker(client *http.Client) *Checker {
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Second,
		}
	}
	return &Checker{
		client: client,
		rules:  make(map[string]*rules),
	}
}

// Allowed retorna se a URL é permitida para User-agent: *.
func (c *Checker) Allowed(ctx context.Context, rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := u.Host
	if host == "" {
		return false
	}

	rs := c.rulesForHost(ctx, u.Scheme, host)
	if rs == nil {
		// Sem regras conhecidas, assume permitido.
		return true
	}

	path := u.Path
	for _, p := range rs.disallow {
		if strings.HasPrefix(path, p) {
			return false
		}
	}

	return true
}

func (c *Checker) rulesForHost(ctx context.Context, scheme, host string) *rules {
	c.mu.RLock()
	rs, ok := c.rules[host]
	c.mu.RUnlock()
	if ok {
		return rs
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if rs, ok = c.rules[host]; ok {
		return rs
	}

	u := scheme + "://" + host + "/robots.txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	rs = parseRobots(resp)
	c.rules[host] = rs
	return rs
}

func parseRobots(resp *http.Response) *rules {
	s := bufio.NewScanner(resp.Body)

	var (
		currentUAStar bool
		disallow      []string
	)

	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		field := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch field {
		case "user-agent":
			if value == "*" {
				currentUAStar = true
			} else {
				currentUAStar = false
			}
		case "disallow":
			if currentUAStar {
				if value == "" {
					continue
				}
				if !strings.HasPrefix(value, "/") {
					value = "/" + value
				}
				disallow = append(disallow, value)
			}
		}
	}

	return &rules{disallow: disallow}
}

