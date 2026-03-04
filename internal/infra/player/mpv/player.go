package mpv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bernardofernandezz/ani-br/internal/domain"
	"github.com/bernardofernandezz/ani-br/internal/infra/errorx"
)

type Player struct {
	mpvPath string
}

func New() (*Player, error) {
	p, err := exec.LookPath("mpv")
	if err != nil {
		return nil, &errorx.AppError{
			Code:    errorx.CodeDependencyMissing,
			Message: "mpv não encontrado. Instale o mpv para reproduzir animes.",
			Err:     err,
		}
	}
	return &Player{mpvPath: p}, nil
}

func (p *Player) PlayEpisode(ctx context.Context, ep domain.Episode, preferredLang domain.Language, preferredQuality domain.Quality) error {
	stream, err := pickStreamWithMeta(ep.Streams, preferredLang, preferredQuality)
	if err != nil {
		if errors.Is(err, domain.ErrStreamUnavailable) {
			return &errorx.AppError{
				Code:    errorx.CodeNotFound,
				Message: "Nenhum stream disponível para este episódio.",
				Err:     err,
			}
		}
		return err
	}

	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("ani-br-mpv-%d.sock", time.Now().UnixNano()))
	defer os.Remove(socketPath)

	args := []string{
		"--input-ipc-server=" + socketPath,
		"--force-window=no",
		"--keep-open=no",
		"--quiet",
	}
	if len(stream.Headers) > 0 {
		args = append(args, buildHTTPHeaderFields(stream.Headers))
	}
	args = append(args, "--", stream.URL)

	cmd := exec.CommandContext(ctx, p.mpvPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	// Espera o socket aparecer (curto).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return ctx.Err()
		default:
		}
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Conecta no IPC e tenta aplicar preferências básicas.
	// (Seleção de trilhas e persistência de progresso serão adicionadas em etapas seguintes.)
	ipcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	ipc, err := DialIPC(ipcCtx, socketPath)
	cancel()
	if err == nil {
		_ = ipc.Command(ctx, "set_property", "pause", false)
		_ = ipc.Close()
	}

	// Aguarda término do processo.
	if err := cmd.Wait(); err != nil {
		// Se ctx cancelou, CommandContext pode retornar erro de kill.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}

func pickStreamWithMeta(streams []domain.Stream, lang domain.Language, q domain.Quality) (domain.Stream, error) {
	if len(streams) == 0 {
		return domain.Stream{}, domain.ErrStreamUnavailable
	}

	// 1) Preferência por idioma + qualidade exata.
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
		return s, nil
	}

	// 2) Preferência por idioma, qualquer qualidade.
	for _, s := range streams {
		if s.URL == "" {
			continue
		}
		if lang != "" && s.Language != lang {
			continue
		}
		return s, nil
	}

	// 3) Fallback: primeiro stream válido.
	for _, s := range streams {
		if s.URL != "" {
			return s, nil
		}
	}

	return domain.Stream{}, domain.ErrStreamUnavailable
}

// buildHTTPHeaderFields monta --http-header-fields="Key: Value, Key2: Value2" para mpv.
func buildHTTPHeaderFields(h map[string]string) string {
	if len(h) == 0 {
		return ""
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(h[k])
	}
	return "--http-header-fields=" + b.String()
}

