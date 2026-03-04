package tui

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/bernardofernandezz/ani-br/internal/domain"
)

type searchModel struct {
	search SearchService
	lang   domain.Language

	width  int
	height int

	input   textinput.Model
	results list.Model
	spin    spinner.Model

	loading bool
	err     error

	// debounce
	lastQuery   string
	lastToken   int

	// selection
	selectedAnime    *domain.Anime
	consumeSelection bool
}

type animeItem struct {
	anime domain.Anime
}

func (i animeItem) FilterValue() string { return i.anime.Title }
func (i animeItem) Title() string       { return i.anime.Title }
func (i animeItem) Description() string {
	lang := string(i.anime.PreferredLang)
	if lang == "" {
		lang = "pt-BR"
	}
	return string(i.anime.Provider) + " · " + lang
}

type searchDebounceMsg struct {
	token int
	query string
}

type searchResultMsg struct {
	token  int
	animes []domain.Anime
	err    error
}

func newSearchModel(search SearchService, lang domain.Language) searchModel {
	in := textinput.New()
	in.Placeholder = "Buscar anime (PT-BR)…"
	in.Focus()
	in.CharLimit = 120
	in.Width = 40

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Resultados"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	sp := spinner.New()
	sp.Spinner = spinner.Line

	return searchModel{
		search:  search,
		lang:    lang,
		input:   in,
		results: l,
		spin:    sp,
	}
}

func (m searchModel) withSize(w, h int) searchModel {
	m.width, m.height = w, h
	m.results.SetSize(w-6, h-8)
	return m
}

func (m searchModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick)
}

func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if it, ok := m.results.SelectedItem().(animeItem); ok {
				a := it.anime
				m.selectedAnime = &a
				m.consumeSelection = true
				return m, nil
			}
		}

	case searchDebounceMsg:
		if msg.token != m.lastToken || msg.query != m.lastQuery {
			return m, nil
		}
		if msg.query == "" {
			m.results.SetItems(nil)
			m.err = nil
			return m, nil
		}
		m.loading = true
		m.err = nil
		return m, tea.Batch(m.spin.Tick, m.searchCmd(msg.token, msg.query))

	case searchResultMsg:
		if msg.token != m.lastToken {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		items := make([]list.Item, 0, len(msg.animes))
		for _, a := range msg.animes {
			items = append(items, animeItem{anime: a})
		}
		m.results.SetItems(items)
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.results, _ = m.results.Update(msg)

	// debounce sempre que o input mudar.
	q := m.input.Value()
	if q != m.lastQuery {
		m.lastQuery = q
		m.lastToken++
		token := m.lastToken
		return m, tea.Batch(cmd, tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg {
			return searchDebounceMsg{token: token, query: q}
		}))
	}

	return m, cmd
}

func (m searchModel) View() string {
	s := "ani-br\n\n" + m.input.View() + "\n\n"
	if m.loading {
		s += m.spin.View() + " buscando…\n\n"
	}
	if m.err != nil && !errors.Is(m.err, domain.ErrAnimeNotFound) {
		s += "Erro: " + m.err.Error() + "\n\n"
	}
	return s + m.results.View() + "\n\n(q para sair)"
}

func (m searchModel) searchCmd(token int, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		animes, err := m.search.Execute(ctx, query, m.lang)
		return searchResultMsg{token: token, animes: animes, err: err}
	}
}

