package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type Styles struct {
	Title        lipgloss.Style
	Subtle       lipgloss.Style
	Error        lipgloss.Style
	Help         lipgloss.Style
	Frame        lipgloss.Style
}

func NewStyles() Styles {
	// Detecção simples de tema, evitando hardcode de cores quebradas.
	isDark := termenv.HasDarkBackground()

	var accent, subtle, errc string

	if isDark {
		accent = "#7AA2F7"
		subtle = "#7A7A7A"
		errc = "#F7768E"
	} else {
		accent = "#2B59C3"
		subtle = "#666666"
		errc = "#C1121F"
	}

	return Styles{
		Title:  lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true),
		Subtle: lipgloss.NewStyle().Foreground(lipgloss.Color(subtle)),
		Error:  lipgloss.NewStyle().Foreground(lipgloss.Color(errc)).Bold(true),
		Help:   lipgloss.NewStyle().Foreground(lipgloss.Color(subtle)),
		Frame:  lipgloss.NewStyle().Padding(1, 2),
	}
}

