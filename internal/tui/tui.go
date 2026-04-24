package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cheryl-chun/cheryl-code/internal/llm"
)

func Run(agent *llm.Agent) error {
	m := NewModel(agent)

	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err := p.Run()
	return err
}
