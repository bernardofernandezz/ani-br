package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bernardofernandezz/ani-br/internal/domain"
)

type SearchService interface {
	Execute(ctx context.Context, query string, lang domain.Language) ([]domain.Anime, error)
}

type EpisodeService interface {
	GetEpisodes(ctx context.Context, animeID string) ([]domain.Episode, error)
}

// PlayFunc representa a operação de tocar um episódio específico.
type PlayFunc func(ctx context.Context, animeID string, episodeNumber int) error

// NewProgram cria um programa Bubbletea pronto para rodar.
func NewProgram(search SearchService, episodes EpisodeService, play PlayFunc, initialLang domain.Language) *tea.Program {
	m := newRootModel(search, episodes, play, initialLang)
	return tea.NewProgram(m, tea.WithAltScreen())
}

