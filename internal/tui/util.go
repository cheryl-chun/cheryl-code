package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cheryl-chun/cheryl-code/internal/llm"
)

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

// 渲染用户消息（带边框）
func renderUserMessage(content string, width int) string {
	// 用户消息样式：浅蓝色边框
	userBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).  // 蓝色
		Padding(0, 1).
		Width(width - 10).  // 留出左侧图标和边距
		Foreground(lipgloss.Color("15"))  // 白色文字

	// 用户标签样式
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	var sb strings.Builder
	sb.WriteString(labelStyle.Render("👤 You:"))
	sb.WriteString("\n")
	sb.WriteString(userBoxStyle.Render(content))
	sb.WriteString("\n")

	return sb.String()
}

// 渲染助手消息
func renderAssistantMessage(content string, width int) string {
	if content == "" {
		return ""
	}

	// 助手标签样式
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("green")).
		Bold(true)

	// 内容样式
	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).  // 浅灰色
		Width(width - 6)

	var sb strings.Builder
	sb.WriteString(labelStyle.Render("🤖 Assistant:"))
	sb.WriteString("\n")
	sb.WriteString(contentStyle.Render(content))
	sb.WriteString("\n")

	return sb.String()
}

// 渲染分割线（占满宽度）
func renderSeparator(width int) string {
	if width <= 0 {
		width = 80
	}

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).  // 深灰色
		Width(width)

	line := strings.Repeat("─", width)
	return separatorStyle.Render(line)
}
// 渲染工具调用
func renderToolCall(tc *llm.ToolCallState, width int) string {
	if tc == nil {
		return ""
	}

	state := tc.State()
	var sb strings.Builder

	// 工具卡片样式
	var borderColor string
	switch tc.Status() {
	case llm.ToolStatusPendingApproval:
		borderColor = "yellow"
	case llm.ToolStatusRunning:
		borderColor = "cyan"
	case llm.ToolStatusSuccess:
		borderColor = "green"
	case llm.ToolStatusError, llm.ToolStatusRejected:
		borderColor = "red"
	default:
		borderColor = "240"
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1).
		Width(width - 6)

	var cardContent strings.Builder

	// 标题行
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(state.Color())).
		Bold(true)

	cardContent.WriteString(titleStyle.Render(
		fmt.Sprintf("%s %s [%s]", state.Icon(), tc.Name, tc.Status())))
	cardContent.WriteString("\n\n")

	// 参数
	if len(tc.Args) > 0 {
		argsJSON, _ := json.MarshalIndent(tc.Args, "", "  ")
		argsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Faint(true)

		cardContent.WriteString(argsStyle.Render("Args:"))
		cardContent.WriteString("\n")
		cardContent.WriteString(argsStyle.Render(string(argsJSON)))
		cardContent.WriteString("\n")
	}

	// 等待审批提示
	if tc.Status() == llm.ToolStatusPendingApproval {
		cardContent.WriteString("\n")
		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("yellow")).
			Italic(true)
		cardContent.WriteString(hintStyle.Render("⏳ Waiting for approval..."))
		cardContent.WriteString("\n")
	}

	// 结果
	if tc.Result != "" {
		cardContent.WriteString("\n")
		
		resultLabelStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("blue")).
			Bold(true)
		cardContent.WriteString(resultLabelStyle.Render("Result:"))
		cardContent.WriteString("\n")

		result := tc.Result
		if len(result) > 500 {
			result = result[:500] + "..."
		}

		resultStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("117"))  // 浅蓝色
		cardContent.WriteString(resultStyle.Render(result))
		cardContent.WriteString("\n")
	}

	// 错误
	if tc.Error != nil {
		cardContent.WriteString("\n")
		
		errorLabelStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("red")).
			Bold(true)
		cardContent.WriteString(errorLabelStyle.Render("Error:"))
		cardContent.WriteString("\n")

		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("red"))
		cardContent.WriteString(errorStyle.Render(fmt.Sprintf("%v", tc.Error)))
		cardContent.WriteString("\n")
	}

	// 耗时
	if !tc.CompletedAt.IsZero() {
		cardContent.WriteString("\n")
		durationStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Faint(true)
		cardContent.WriteString(durationStyle.Render(
			fmt.Sprintf("⏱  Duration: %v", tc.Duration())))
	}

	sb.WriteString(cardStyle.Render(cardContent.String()))
	sb.WriteString("\n")

	return sb.String()
}

// 渲染审批选择器
func renderApprovalSelector(toolCall *llm.ToolCallState, options []ApprovalOption, cursor int, width int) string {
	var sb strings.Builder

	// 工具描述（加强视觉效果）
	descBoxStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("yellow")).
		Background(lipgloss.Color("235")).
		Bold(true).
		Padding(0, 2).
		Width(width - 4).
		Align(lipgloss.Center)

	desc := getToolDescription(toolCall)
	sb.WriteString(descBoxStyle.Render(desc))
	sb.WriteString("\n\n")

	// 选项列表
	optionTexts := map[ApprovalOption]string{
		ApprovalOptionApprove:    "✓ Approve",
		ApprovalOptionReject:     "✗ Reject",
		ApprovalOptionApproveAll: "✓✓ Approve All",
		ApprovalOptionRejectAll:  "✗✗ Reject All",
	}

	optionColors := map[ApprovalOption]string{
		ApprovalOptionApprove:    "green",
		ApprovalOptionReject:     "red",
		ApprovalOptionApproveAll: "green",
		ApprovalOptionRejectAll:  "red",
	}

	for i, option := range options {
		isSelected := i == cursor
		text := optionTexts[option]
		color := optionColors[option]

		if isSelected {
			// 选中：带背景高亮
			selectedStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("black")).
				Background(lipgloss.Color(color)).
				Bold(true).
				Padding(0, 2).
				Width(width - 8)

			sb.WriteString("  ")
			sb.WriteString(selectedStyle.Render(text))
		} else {
			// 未选中：只有文字颜色
			normalStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(color)).
				Padding(0, 2)

			sb.WriteString("  ")
			sb.WriteString(normalStyle.Render(text))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

func getToolDescription(tc *llm.ToolCallState) string {
	switch tc.Name {
	case "write_file":
		if path, ok := tc.Args["path"].(string); ok {
			return fmt.Sprintf("📝 需要创建文件: %s", path)
		}
		return "📝 需要创建文件"

	case "bash":
		if cmd, ok := tc.Args["command"].(string); ok {
			if len(cmd) > 50 {
				cmd = cmd[:50] + "..."
			}
			return fmt.Sprintf("⚡ 需要执行命令: %s", cmd)
		}
		return "⚡ 需要执行命令"

	case "read_file":
		if path, ok := tc.Args["path"].(string); ok {
			return fmt.Sprintf("📖 需要读取文件: %s", path)
		}
		return "📖 需要读取文件"

	default:
		return fmt.Sprintf("🔧 需要执行工具: %s", tc.Name)
	}
}
