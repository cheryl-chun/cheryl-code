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

type MessageType string
type ApprovalOption int

const (
	MessageTypeText       MessageType    = "text"
	MessageTypeToolCall   MessageType    = "tool_call"
	ApprovalOptionApprove ApprovalOption = iota
	ApprovalOptionReject
	ApprovalOptionApproveAll
	ApprovalOptionRejectAll
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

	// stream event
	currentEventCh <-chan llm.StreamEvent
	streamActive   bool

	cancelFunc context.CancelFunc

	// approval event
	approvalMode    bool
	approvalOptions []ApprovalOption
	approvalCursor  int

	// DEBUG 用
	lastKey string
}

type Message struct {
	Type    MessageType
	Role    string // "user" 或 "assistant"
	Content string

	// tool execute
	ToolCall *llm.ToolCallState
}

type agentResponseMsg struct {
	response string
	err      error
}

type streamStartMsg struct {
	eventCh    <-chan llm.StreamEvent
	cancelFunc context.CancelFunc
}

type streamEventMsg struct {
	event llm.StreamEvent
}

type userPromptMsg struct {
	prompt string
}

func NewModel(agent *llm.Agent) Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.Focus()
	ta.CharLimit = 0

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

		// approve tool
		if m.agent.GetState().HasPendingApprovals() {
			switch msg.String() {
			case "up", "k":
				// 上移光标
				if m.approvalCursor > 0 {
					m.approvalCursor--
				}

				return m, nil
			case "down", "j":
				if m.approvalCursor < len(m.approvalOptions)-1 {
					m.approvalCursor++
				}
				return m, nil
			case "enter":
				return m.executeApprovalOption()
			case "esc":
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+c":
			if m.waiting || m.streamActive {
				return m.stopAgent()
			}
			return m, tea.Quit
		case "alt+enter":
			m.textarea.SetValue(m.textarea.Value() + "\n")
			return m, nil
		case "enter":
			if !m.waiting && !m.approvalMode {
				return m.sendMessage()
			}
		}

	case streamStartMsg:
		m.streamActive = true
		m.currentEventCh = msg.eventCh
		m.cancelFunc = msg.cancelFunc

		m.addMessage(Message{
			Type:    MessageTypeText,
			Role:    "assistant",
			Content: "",
		})
		return m, listenStreamEvent(msg.eventCh)

	case streamEventMsg:
		return m.handleStreamEvent(msg.event)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 2   // 顶部标题/边距
		footerHeight := 2   // 底部帮助信息
		textareaHeight := 4 // 输入框高度

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

	if !m.agent.GetState().HasPendingApprovals() {
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

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
				Width(m.width) // 撑满宽度

		inputBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Width(m.width - 4) // 减去边框宽度
	)

	var statusText string
	var inputArea string

	if m.approvalMode && m.agent.GetState().HasPendingApprovals() {
		pendingCount := len(m.agent.GetState().PendingApprovals)
		currentTool := m.agent.GetState().PendingApprovals[0]

		statusText = fmt.Sprintf("⚠️  Approval required (%d pending) • [↑↓] Select [Enter] Confirm", pendingCount)

		// 渲染选择器
		selectorContent := renderApprovalSelector(currentTool, m.approvalOptions, m.approvalCursor, m.width-4)

		selectorStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("yellow")).
			Width(m.width-4).
			Padding(1, 2)

		inputArea = selectorStyle.Render(selectorContent)
	} else if m.waiting {
		// ========== 等待模式 ==========
		statusText = "Processing... • [Ctrl+C] Stop"

		// Processing 内容（替代输入框）
		processingStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("cyan")).
			Padding(1, 2).
			Width(m.width - 4).
			Align(lipgloss.Center)

		processingContent := fmt.Sprintf("%s Processing...", m.spinner.View())
		inputArea = processingStyle.Render(processingContent)
	} else {
		// ========== 普通模式：输入框 ==========
		statusText = "Enter: Send • Ctrl+C: Quit • Alt+Enter: NewLine"
		inputArea = inputBoxStyle.Render(m.textarea.View())
	}

	// 组装界面
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		"", // 空行
		inputArea,
		statusBarStyle.Render(statusText),
	)
}

func (m Model) sendMessage() (Model, tea.Cmd) {
	userInput := strings.TrimSpace(m.textarea.Value())
	if userInput == "" {
		return m, nil
	}

	m.waiting = true
	m.addMessage(Message{
		Type:    MessageTypeText,
		Role:    "user",
		Content: userInput,
	})
	m.textarea.Reset()
	m.viewport.GotoBottom()

	return m, m.startAgentStream(userInput)
}

func (m *Model) startAgentStream(prompt string) tea.Cmd {
	return func() tea.Msg {
		// 创建可取消的 context
		ctx, cancel := context.WithCancel(context.Background())

		// ✅ 保存到 Model（这需要返回一个特殊的消息类型）
		// 由于 Cmd 不能直接修改 Model，我们需要在 Update 中处理

		eventCh, err := m.agent.RunStream(ctx, prompt)

		if err != nil {
			cancel() // 发生错误，取消 context
			return streamEventMsg{
				event: llm.StreamEvent{
					Type:  llm.StreamError,
					Error: err,
				},
			}
		}

		// ✅ 返回一个包含 cancelFunc 的消息
		return streamStartMsg{
			eventCh:    eventCh,
			cancelFunc: cancel,
		}
	}
}

func (m Model) handleStreamEvent(event llm.StreamEvent) (Model, tea.Cmd) {
	switch event.Type {

	// ========== 文本内容（逐字追加）==========
	case llm.StreamContent:
		// 找到最后一个 assistant 消息，追加内容
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].Role == "assistant" && m.messages[i].Type == MessageTypeText {
				m.messages[i].Content += event.Content
				break
			}
		}
		m.updateViewportContent()
		m.viewport.GotoBottom()

		// 继续监听下一个事件
		return m, listenStreamEvent(m.currentEventCh)

	// ========== 需要审批 ==========
	case llm.StreamApprovalRequired:
		// 添加工具调用消息
		m.addMessage(Message{
			Type:     MessageTypeToolCall,
			ToolCall: event.ToolCall,
		})
		m.updateViewportContent()
		m.viewport.GotoBottom()

		if !m.approvalMode {
			m.approvalMode = true
			m.initApprovalOptions()
		}

		// 继续监听（可能还有更多工具）
		return m, listenStreamEvent(m.currentEventCh)

	// ========== 工具调用开始 ==========
	case llm.StreamToolCall:
		// 如果消息列表中还没有这个工具，添加
		found := false
		for i := range m.messages {
			if m.messages[i].Type == MessageTypeToolCall &&
				m.messages[i].ToolCall != nil &&
				m.messages[i].ToolCall.ID == event.ToolCall.ID {
				found = true
				break
			}
		}

		if !found {
			m.addMessage(Message{
				Type:     MessageTypeToolCall,
				ToolCall: event.ToolCall,
			})
		}

		m.updateViewportContent()
		m.viewport.GotoBottom()

		return m, listenStreamEvent(m.currentEventCh)

	// ========== 工具结果 ==========
	case llm.StreamToolResult:
		// 更新对应工具的显示
		m.updateViewportContent()
		m.viewport.GotoBottom()

		return m, listenStreamEvent(m.currentEventCh)

	// ========== 流结束 ==========
	case llm.StreamDone:
		m.waiting = false
		m.streamActive = false
		m.updateViewportContent()
		return m, nil

	// ========== 错误 ==========
	case llm.StreamError:
		m.waiting = false
		m.streamActive = false

		m.addMessage(Message{
			Type:    MessageTypeText,
			Role:    "assistant",
			Content: fmt.Sprintf("Error: %v", event.Error),
		})
		m.updateViewportContent()

		return m, nil

	default:
		// 未知事件，继续监听
		return m, listenStreamEvent(m.currentEventCh)
	}
}
func listenStreamEvent(eventCh <-chan llm.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-eventCh
		if !ok {
			return streamEventMsg{
				event: llm.StreamEvent{
					Type: llm.StreamDone,
				},
			}
		}
		return streamEventMsg{event: event}
	}
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

func (m *Model) addMessage(msg Message) {
	m.messages = append(m.messages, msg)
	m.updateViewportContent()
}

func (m *Model) updateViewportContent() {
	var sb strings.Builder

	sb.WriteString(welcomeMessage())
	sb.WriteString("\n")

	for _, msg := range m.messages {
		switch msg.Type {
		case MessageTypeText:
			if msg.Role == "user" {
				sb.WriteString(renderUserMessage(msg.Content, m.width))
			} else {
				sb.WriteString(renderAssistantMessage(msg.Content, m.width))
			}
		case MessageTypeToolCall:
			sb.WriteString(renderToolCall(msg.ToolCall, m.width))
		}

		sb.WriteString("\n")
		sb.WriteString(renderSeparator(m.width))
		sb.WriteString("\n\n")
	}

	m.viewport.SetContent(sb.String())
}

func (m Model) approveTool() (Model, tea.Cmd) {
	if !m.agent.GetState().HasPendingApprovals() {
		return m, nil
	}

	// approve first tool
	pending := m.agent.GetState().PendingApprovals
	if err := m.agent.ApproveToolCall(pending[0].ID); err != nil {
		return m, nil
	}

	m.updateViewportContent()

	if !m.agent.GetState().HasPendingApprovals() {
		m.agent.ResumeExecution()
	}

	return m, listenStreamEvent(m.currentEventCh)
}

func (m Model) rejectTool() (Model, tea.Cmd) {
	if !m.agent.GetState().HasPendingApprovals() {
		return m, nil
	}

	pending := m.agent.GetState().PendingApprovals
	if err := m.agent.RejectToolCall(pending[0].ID); err != nil {
		return m, nil
	}

	m.updateViewportContent()

	if !m.agent.GetState().HasPendingApprovals() {
		m.agent.ResumeExecution()
	}

	return m, listenStreamEvent(m.currentEventCh)
}

func (m Model) approveAll() (Model, tea.Cmd) {
	if err := m.agent.ApproveAll(); err != nil {
		return m, nil
	}

	m.updateViewportContent()
	m.agent.ResumeExecution()

	return m, listenStreamEvent(m.currentEventCh)
}

func (m Model) rejectAll() (Model, tea.Cmd) {
	if err := m.agent.RejectAll(); err != nil {
		return m, nil
	}

	m.updateViewportContent()
	m.agent.ResumeExecution()

	return m, listenStreamEvent(m.currentEventCh)
}

func (m *Model) initApprovalOptions() {
	pending := m.agent.GetState().PendingApprovals

	if len(pending) == 1 {
		m.approvalOptions = []ApprovalOption{
			ApprovalOptionApprove,
			ApprovalOptionReject,
		}
	} else {
		m.approvalOptions = []ApprovalOption{
			ApprovalOptionApprove,
			ApprovalOptionReject,
			ApprovalOptionApproveAll,
			ApprovalOptionRejectAll,
		}
	}
	m.approvalCursor = 0
}

func (m Model) executeApprovalOption() (Model, tea.Cmd) {
	if m.approvalCursor >= len(m.approvalOptions) {
		return m, nil
	}

	selectedOption := m.approvalOptions[m.approvalCursor]
	pending := m.agent.GetState().PendingApprovals

	switch selectedOption {
	case ApprovalOptionApprove:
		if len(pending) > 0 {
			if err := m.agent.ApproveToolCall(pending[0].ID); err != nil {
				return m, nil
			}
		}

	case ApprovalOptionReject:
		if len(pending) > 0 {
			if err := m.agent.RejectToolCall(pending[0].ID); err != nil {
				return m, nil
			}
		}

	case ApprovalOptionApproveAll:
		if err := m.agent.ApproveAll(); err != nil {
			return m, nil
		}

	case ApprovalOptionRejectAll:
		if err := m.agent.RejectAll(); err != nil {
			return m, nil
		}
	}

	m.updateViewportContent()

	// 检查是否还有待审批的
	if m.agent.GetState().HasPendingApprovals() {
		m.initApprovalOptions()
		return m, listenStreamEvent(m.currentEventCh)
	} else {
		m.approvalMode = false
		m.agent.ResumeExecution()
		return m, listenStreamEvent(m.currentEventCh)
	}
}

func (m Model) stopAgent() (Model, tea.Cmd) {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}

	m.waiting = false
	m.streamActive = false

	m.addMessage(Message{
		Type:    MessageTypeText,
		Role:    "assistant",
		Content: "⚠️ Stopped by user",
	})

	return m, nil
}
