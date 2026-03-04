package tui

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/bernardofernandezz/ani-br/internal/domain"
)

type episodesModel struct {
	episodes EpisodeService
	lang     domain.Language

	width  int
	height int

	anime domain.Anime

	list    list.Model
	spin    spinner.Model
	loading bool
	err     error

	backToSearch bool

	selectedEpisode   *domain.Episode
	consumeSelection  bool
}

type episodeItem struct {
	ep domain.Episode
}

func (i episodeItem) FilterValue() string { return i.ep.Title }
func (i episodeItem) Title() string {
	if i.ep.EpisodeNumber > 0 {
		return "Episódio " + itoa(i.ep.EpisodeNumber) + " — " + i.ep.Title
	}
	return i.ep.Title
}
func (i episodeItem) Description() string { return "" }

type episodesLoadedMsg struct {
	eps []domain.Episode
	err error
}

func newEpisodesModel(episodes EpisodeService, lang domain.Language) episodesModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Episódios"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	sp := spinner.New()
	sp.Spinner = spinner.Line

	return episodesModel{
		episodes: episodes,
		lang:     lang,
		list:     l,
		spin:     sp,
	}
}

func (m episodesModel) withSize(w, h int) episodesModel {
	m.width, m.height = w, h
	m.list.SetSize(w-6, h-8)
	return m
}

func (m episodesModel) withAnime(a domain.Anime) episodesModel {
	m.anime = a
	m.list.Title = "Episódios — " + a.Title
	m.list.SetItems(nil)
	m.err = nil
	return m
}

func (m episodesModel) Init() tea.Cmd {
	m.loading = true
	return tea.Batch(m.spin.Tick, m.loadCmd())
}

func (m episodesModel) Update(msg tea.Msg) (episodesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			m.backToSearch = true
			return m, nil
		case "enter":
			if it, ok := m.list.SelectedItem().(episodeItem); ok {
				ep := it.ep
				m.selectedEpisode = &ep
				m.consumeSelection = true
				return m, nil
			}
		}

	case episodesLoadedMsg:
		m.loading = false
		m.err = msg.err
		if msg.err != nil {
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.eps))
		for _, ep := range msg.eps {
			items = append(items, episodeItem{ep: ep})
		}
		m.list.SetItems(items)
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.loading {
		var spinCmd tea.Cmd
		m.spin, spinCmd = m.spin.Update(msg)
		return m, tea.Batch(cmd, spinCmd)
	}
	return m, cmd
}

func (m episodesModel) View() string {
	s := ""
	if m.loading {
		s += m.spin.View() + " carregando episódios…\n\n"
	}
	if m.err != nil {
		if errors.Is(m.err, domain.ErrEpisodeNotFound) {
			s += "Nenhum episódio encontrado.\n\n"
		} else {
			s += "Erro: " + m.err.Error() + "\n\n"
		}
	}
	s += m.list.View()
	s += "\n\n(Enter = reproduzir no mpv | esc = voltar | q = sair)"
	return s
}

func (m episodesModel) loadCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		eps, err := m.episodes.GetEpisodes(ctx, m.anime.ID)
		return episodesLoadedMsg{eps: eps, err: err}
	}
}

func itoa(v int) string {
	// evita fmt para não puxar dependência desnecessária na hot path da renderização.
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [16]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

