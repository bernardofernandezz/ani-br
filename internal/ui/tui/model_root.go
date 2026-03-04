package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bernardofernandezz/ani-br/internal/domain"
)

type screen int

const (
	screenSearch screen = iota
	screenEpisodes
)

type rootModel struct {
	ctx context.Context

	styles Styles

	width  int
	height int

	lang domain.Language

	active screen

	play PlayFunc

	searchModel   searchModel
	episodesModel episodesModel
}

func newRootModel(search SearchService, episodes EpisodeService, play PlayFunc, initialLang domain.Language) rootModel {
	if initialLang == "" {
		initialLang = domain.LanguagePTBRDub
	}

	return rootModel{
		ctx:    context.Background(),
		styles: NewStyles(),
		lang:   initialLang,
		active: screenSearch,
		play:   play,
		searchModel: newSearchModel(search, initialLang),
		episodesModel: newEpisodesModel(episodes, initialLang),
	}
}

func (m rootModel) Init() tea.Cmd {
	return tea.Batch(
		m.searchModel.Init(),
	)
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.searchModel = m.searchModel.withSize(msg.Width, msg.Height)
		m.episodesModel = m.episodesModel.withSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	switch m.active {
	case screenSearch:
		next, cmd := m.searchModel.Update(msg)
		m.searchModel = next
		// Transição: seleção de anime.
		if m.searchModel.selectedAnime != nil && m.searchModel.consumeSelection {
			m.searchModel.consumeSelection = false
			m.active = screenEpisodes
			m.episodesModel = m.episodesModel.withAnime(*m.searchModel.selectedAnime)
			return m, m.episodesModel.Init()
		}
		return m, cmd

	case screenEpisodes:
		next, cmd := m.episodesModel.Update(msg)
		m.episodesModel = next
		// Voltar para busca.
		if m.episodesModel.backToSearch {
			m.episodesModel.backToSearch = false
			m.active = screenSearch
			return m, nil
		}
		// Tocar episódio selecionado.
		if m.episodesModel.selectedEpisode != nil && m.episodesModel.consumeSelection {
			ep := *m.episodesModel.selectedEpisode
			m.episodesModel.consumeSelection = false
			playCmd := m.playEpisodeCmd(m.episodesModel.anime.ID, ep.EpisodeNumber)
			if playCmd == nil {
				return m, cmd
			}
			if cmd != nil {
				return m, tea.Batch(cmd, playCmd)
			}
			return m, playCmd
		}
		return m, cmd

	default:
		return m, nil
	}
}

func (m rootModel) View() string {
	switch m.active {
	case screenSearch:
		return m.styles.Frame.Render(m.searchModel.View())
	case screenEpisodes:
		return m.styles.Frame.Render(m.episodesModel.View())
	default:
		return "estado inválido"
	}
}

func (m rootModel) playEpisodeCmd(animeID string, episodeNumber int) tea.Cmd {
	if m.play == nil {
		return nil
	}
	return func() tea.Msg {
		_ = m.play(m.ctx, animeID, episodeNumber)
		return nil
	}
}

