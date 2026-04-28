package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cheryl-chun/cheryl-code/internal/llm"
)

type Model struct {
	viewport viewport.Model // 对话历史显示区
	textarea textarea.Model // 用户输入框
	spinner  spinner.Model  // 等待动画

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

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		textarea: ta,
		viewport: vp,
		spinner:  sp,
		messages: []Message{},
		ready:    false,
		agent:    agent,
	}
}

// Init 初始化命令
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
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
		case "ctrl+c":
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

		headerHeight := 2       // 顶部标题/边距
		footerHeight := 2       // 底部帮助信息
		textareaHeight := 4     // 输入框高度
		
		// viewport 占据剩余空间
		vpHeight := msg.Height - headerHeight - textareaHeight - footerHeight
		
		m.viewport.Width = msg.Width
		m.viewport.Height = vpHeight
		
		m.textarea.SetWidth(msg.Width)
		m.textarea.SetHeight(textareaHeight)
		
		if !m.ready {
			m.viewport.SetContent(welcomeMessage())
			m.ready = true
		} else {
			// 窗口大小变化时，重新渲染内容
			m.updateViewportContent()
		}
	}

	if m.waiting {
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
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
    
    // 定义样式
    var (
        subtle = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
        
        statusBarStyle = lipgloss.NewStyle().
            Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
            Background(subtle).
            Padding(0, 1).
            Width(m.width)  // ← 撑满宽度
        
        inputBoxStyle = lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("62")).
            Width(m.width - 4)  // 减去边框宽度
    )
    
    // 状态栏内容
    statusText := "Enter: Send • Ctrl+C: Quit"
    if m.waiting {
        statusText = fmt.Sprintf("%s Processing...", m.spinner.View())
    }
    
    // 组装界面
    return lipgloss.JoinVertical(
        lipgloss.Left,
        m.viewport.View(),
        "",  // 空行
        inputBoxStyle.Render(m.textarea.View()),
        statusBarStyle.Render(statusText),
    )
}

func (m Model) sendMessage() (Model, tea.Cmd) {
	userInput := strings.TrimSpace(m.textarea.Value())
	if userInput == "" {
		return m, nil
	}

	m.waiting = true
	m.addMessage("user", userInput)
	m.textarea.Reset()


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
