package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) handleToolSelectMode(key string) (Model, tea.Cmd) {
	indices := m.getToolCallIndices()

	switch key {
	case "up", "k":
		// 上移
		if m.toolSelectCursor > 0 {
			m.toolSelectCursor--
		}
		m.updateViewportContent()

		// 自动滚动到选中的工具
		m.scrollToSelectedTool()

		return m, nil

	case "down", "j":
		// 下移
		if m.toolSelectCursor < len(indices)-1 {
			m.toolSelectCursor++
		}
		m.updateViewportContent()
		m.scrollToSelectedTool()
		return m, nil

	case "enter", "e", " ":
		// 展开/折叠选中的工具
		if tool := m.getSelectedToolMessage(); tool != nil {
			tool.ToolExpanded = !tool.ToolExpanded
			m.updateViewportContent()
		}
		return m, nil

	case "J":
		// Shift+J 向下滚动 viewport
		m.viewport.LineDown(3)
		return m, nil

	case "K":
		// Shift+K 向上滚动 viewport
		m.viewport.LineUp(3)
		return m, nil

	case "ctrl+d":
		// 向下翻半页
		m.viewport.HalfViewDown()
		return m, nil

	case "ctrl+u":
		// 向上翻半页
		m.viewport.HalfViewUp()
		return m, nil

	case "esc", "q":
		// 退出选择模式
		return m.exitToolSelectMode()

	default:
		return m, nil
	}
}

func (m *Model) getToolCallIndices() []int {
	var indices []int
	for i, msg := range m.messages {
		if msg.Type == MessageTypeToolCall {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *Model) getSelectedToolMessage() *Message {
	indices := m.getToolCallIndices()
	if m.toolSelectCursor >= 0 && m.toolSelectCursor < len(indices) {
		idx := indices[m.toolSelectCursor]
		return &m.messages[idx]
	}
	return nil
}

func (m Model) enterToolSelectMode() (Model, tea.Cmd) {
	indices := m.getToolCallIndices()
	if len(indices) == 0 {
		return m, nil
	}

	m.toolSelectMode = true

	return m, nil
}

func (m Model) exitToolSelectMode() (Model, tea.Cmd) {
	m.toolSelectMode = false
	m.updateViewportContent()
	return m, nil
}

func (m Model) scrollToSelectedTool() tea.Cmd {
	// 这里可以实现自动滚动到选中的工具
	indices := m.getToolCallIndices()
	if m.toolSelectCursor == 0 {
		m.viewport.GotoTop()
	} else if m.toolSelectCursor == len(indices)-1 {
		m.viewport.GotoBottom()
	}
	return nil
}
