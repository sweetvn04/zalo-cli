package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"zalo-cli/internal/ipc"
)

// A notification during an outstanding request must survive until its reply.
func (m *model) refreshFromArchive() tea.Cmd {
	if !m.connected {
		return nil
	}
	var commands []tea.Cmd
	if m.listDirty && !m.loading {
		m.listDirty = false
		m.loading, m.listID = true, ""
		m.earlyList = make(map[string]ipc.Event)
		m.listAttempt++
		commands = append(commands, m.loadList(), expires("list", m.listAttempt))
	}
	if m.historyDirty[m.active] {
		state := m.historyStates[m.active]
		if state != nil && !state.loading {
			delete(m.historyDirty, m.active)
			commands = append(commands, m.loadHistory(true))
		}
	}
	return tea.Batch(commands...)
}
