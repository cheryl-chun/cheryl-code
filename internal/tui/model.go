package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cheryl-chun/cheryl-code/internal/llm"
)

type Model struct {
	viewport viewport.Model // 对话历史显示区
	textarea textarea.Model // 用户输入框

	messages []Message // 对话历史
	width    int       // 终端宽度
	height   int       // 终端高度
	waiting  bool      // 等待状态
	ready    bool      // 是否准备好显示界面

	agent *llm.Agent

	// DEBUG 用
	lastKey string
}

type Message struct {
	Role    string // "user" 或 "assistant"
	Content string
}

type agentResponseMsg struct {
	response string
	err      error
}

func NewModel(agent *llm.Agent) Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.Focus()

	vp := viewport.New(80, 20)
	vp.SetContent(welcomeMessage())

	return Model{
		textarea: ta,
		viewport: vp,
		messages: []Message{},
		ready:    false,
		agent:    agent,
	}
}

// Init 初始化命令
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.lastKey = fmt.Sprintf("Key: %s | Type: %d | Alt: %v",
			msg.String(), msg.Type, msg.Alt)

        if msg.Type == tea.KeyEnter {
            // Alt+Enter 发送
            return m.sendMessage()
        }
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "shift+enter":
			return m.sendMessage()
		}
	case agentResponseMsg:
		m.waiting = false
		if msg.err != nil {
			m.addMessage("assistant", "Error: "+msg.err.Error())
		} else {
			m.addMessage("assistant", msg.response)
		}
		m.viewport.GotoBottom()
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
			m.textarea.SetWidth(msg.Width)
			m.textarea.SetHeight(4)
			m.ready = true
		}
	}
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		m.viewport.View(),                 // 对话历史
		m.textarea.View(),                 // 输入框
		"Ctrl+Enter: Send • Ctrl+C: Quit", // 帮助
	)
}

func (m Model) sendMessage() (Model, tea.Cmd) {
	userInput := strings.TrimSpace(m.textarea.Value())
	if userInput == "" {
		return m, nil
	}

	m.addMessage("user", userInput)

	m.textarea.Reset()

	m.waiting = true

	return m, m.callAgent(userInput)
}

func (m *Model) callAgent(prompt string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		response, err := m.agent.Run(ctx, prompt)

		return agentResponseMsg{
			response: response,
			err:      err,
		}
	}
}

func (m *Model) addMessage(role, content string) {
	m.messages = append(m.messages, Message{
		Role:    role,
		Content: content,
	})
	m.updateViewportContent()
}

func (m *Model) updateViewportContent() {
	var sb strings.Builder

	sb.WriteString(welcomeMessage())
	sb.WriteString("\n")

	for _, msg := range m.messages {
		if msg.Role == "user" {
			sb.WriteString(fmt.Sprintf("👤 You:\n%s\n\n", msg.Content))
		} else {
			sb.WriteString(fmt.Sprintf("🤖 Assistant:\n%s\n\n", msg.Content))
		}
		sb.WriteString("────────────────────────────────────\n\n")
	}

	if m.waiting {
		sb.WriteString("⏳ Thinking...\n")
	}

	m.viewport.SetContent(sb.String())
}

func welcomeMessage() string {
	return `
╔════════════════════════════════════════╗
║       Cheryl Code - AI Assistant       ║
╚════════════════════════════════════════╝

Welcome! Type your message below.

Commands:
  • Ctrl+Enter - Send message
  • Ctrl+C     - Quit
  • ↑/↓        - Scroll history

`
}
