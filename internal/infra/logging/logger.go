package logging

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Verbose bool
}

// NewLogger cria um logger slog com handler texto (terminal) e JSON (arquivo).
func NewLogger(cfg Config) (*slog.Logger, func() error, error) {
	termHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: levelForVerbose(cfg.Verbose),
	})

	logPath, err := defaultLogPath()
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, nil, err
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}

	jsonHandler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	h := &teeHandler{a: termHandler, b: jsonHandler}
	logger := slog.New(h)

	cleanup := func() error { return f.Close() }
	return logger, cleanup, nil
}

func defaultLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "ani-br", "ani-br.log"), nil
}

func levelForVerbose(verbose bool) slog.Level {
	if verbose {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

type teeHandler struct {
	a slog.Handler
	b slog.Handler
}

func (h *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.a.Enabled(ctx, level) || h.b.Enabled(ctx, level)
}

func (h *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	// Clona record para evitar consumo de attrs em múltiplos handlers.
	r2 := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		r2.AddAttrs(a)
		return true
	})

	_ = h.a.Handle(ctx, r)
	_ = h.b.Handle(ctx, r2)
	return nil
}

func (h *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &teeHandler{a: h.a.WithAttrs(attrs), b: h.b.WithAttrs(attrs)}
}

func (h *teeHandler) WithGroup(name string) slog.Handler {
	return &teeHandler{a: h.a.WithGroup(name), b: h.b.WithGroup(name)}
}

// nowRFC3339 is kept for future structured timestamps.
var _ = time.RFC3339

