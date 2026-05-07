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
		BorderForeground(lipgloss.Color("39")). // 蓝色
		Padding(0, 1).
		Width(width - 10).               // 留出左侧图标和边距
		Foreground(lipgloss.Color("15")) // 白色文字

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
		Foreground(lipgloss.Color("252")). // 浅灰色
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
		Foreground(lipgloss.Color("240")). // 深灰色
		Width(width)

	line := strings.Repeat("─", width)
	return separatorStyle.Render(line)
}

// 渲染工具调用
func renderToolCall(tc *llm.ToolCallState, width int, expanded bool, isSelected bool) string {
	if tc == nil {
		return ""
	}

	state := tc.State()

	if !expanded {
		return renderToolCallCompact(tc, state, isSelected)
	}

	return renderToolCallExpanded(tc, state, width, isSelected)

}

func renderToolCallCompact(tc *llm.ToolCallState, state llm.ToolCallStatusState, isSelected bool) string {
	summary := getToolSummary(tc)

	// ✅ 选中时使用强烈的视觉反馈
	if isSelected {
		// 选中样式：反色 + 粗体
		selectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("black")).
			Background(lipgloss.Color("cyan")).
			Bold(true).
			Padding(0, 1).
			Width(60) // 固定宽度，保持对齐

		compact := fmt.Sprintf("▶ %s %s [%s] → %s ",
			state.Icon(),
			tc.Name,
			tc.Status(),
			summary)

		return selectedStyle.Render(compact) + "\n"
	}

	// 未选中样式
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(state.Color()))

	compact := fmt.Sprintf("  %s %s [%s] → %s",
		state.Icon(),
		tc.Name,
		tc.Status(),
		summary)

	return normalStyle.Render(compact) + "\n"
}

// 提取工具摘要
func getToolSummary(tc *llm.ToolCallState) string {
	switch tc.Name {
	case "write_file", "read_file":
		if path, ok := tc.Args["path"].(string); ok {
			return path
		}
	case "bash":
		if cmd, ok := tc.Args["command"].(string); ok {
			if len(cmd) > 40 {
				return cmd[:40] + "..."
			}
			return cmd
		}
	case "Glob":
		if pattern, ok := tc.Args["pattern"].(string); ok {
			return pattern
		}
	}
	return ""
}

func renderToolCallExpanded(tc *llm.ToolCallState, state llm.ToolCallStatusState, width int, isSelected bool) string {
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

	// 根据选中状态选择边框样式
	cardStyle := lipgloss.NewStyle().
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1).
		Width(width - 6)

	if isSelected {
		// 选中时使用粗边框
		cardStyle = cardStyle.Border(lipgloss.ThickBorder())
	} else {
		// 未选中时使用圆角边框
		cardStyle = cardStyle.Border(lipgloss.RoundedBorder())
	}

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
		argsStr := string(argsJSON)

		// 截断过长的参数（最多10行）
		lines := strings.Split(argsStr, "\n")
		if len(lines) > 10 {
			argsStr = strings.Join(lines[:10], "\n") + "\n... (truncated)"
		}

		argsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Faint(true)

		cardContent.WriteString(argsStyle.Render("Args:"))
		cardContent.WriteString("\n")
		cardContent.WriteString(argsStyle.Render(argsStr))
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

		// 截断过长的结果（最多1000个字符或20行）
		if len(result) > 1000 {
			result = result[:1000] + "... (truncated)"
		}
		lines := strings.Split(result, "\n")
		if len(lines) > 20 {
			result = strings.Join(lines[:20], "\n") + "\n... (truncated)"
		}

		resultStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("117")) // 浅蓝色
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

	// 工具标题
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("yellow")).
		Bold(true)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("🔧 %s", toolCall.Name)))
	sb.WriteString("\n\n")

	// 显示完整的参数
	argsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	if len(toolCall.Args) > 0 {
		argsJSON, _ := json.MarshalIndent(toolCall.Args, "", "  ")
		sb.WriteString(argsStyle.Render("Parameters:"))
		sb.WriteString("\n")
		sb.WriteString(argsStyle.Render(string(argsJSON)))
		sb.WriteString("\n\n")
	}

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

func renderToolSection(width int, toolCount int) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("cyan")).
		Bold(true)

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// 标题文本
	title := fmt.Sprintf("🔧 Tool Calls (%d)", toolCount)
	titleLen := len(title) - 3 // 减去 emoji 占用（emoji 显示宽度约为1，但字符长度为3-4）

	// 计算左右两侧的分割线长度
	remainingWidth := width - titleLen - 4 // 4 是左右各2个空格
	if remainingWidth < 10 {
		remainingWidth = 10
	}
	leftLen := remainingWidth / 2
	rightLen := remainingWidth - leftLen

	leftLine := strings.Repeat("─", leftLen)
	rightLine := strings.Repeat("─", rightLen)

	return fmt.Sprintf("%s %s %s",
		separatorStyle.Render(leftLine),
		titleStyle.Render(title),
		separatorStyle.Render(rightLine))
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
