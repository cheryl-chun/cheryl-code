package tui

import (
	"strings"
)

func (m *Model) updateViewportContent() {
	var sb strings.Builder

	sb.WriteString(welcomeMessage())
	sb.WriteString("\n")

	// 获取工具调用索引
	toolIndices := m.getToolCallIndices()
	var selectedMsgIdx int = -1
	if m.toolSelectMode && m.toolSelectCursor >= 0 && m.toolSelectCursor < len(toolIndices) {
		selectedMsgIdx = toolIndices[m.toolSelectCursor]
	}

	// 分离 LLM 输出和工具调用
	var llmOutput strings.Builder
	var toolCalls strings.Builder

	for i, msg := range m.messages {
		isSelected := m.toolSelectMode && i == selectedMsgIdx

		switch msg.Type {
		case MessageTypeText:
			// LLM 输出单独处理
			if msg.Role == "user" {
				llmOutput.WriteString(renderUserMessage(msg.Content, m.width))
			} else {
				llmOutput.WriteString(renderAssistantMessage(msg.Content, m.width))
			}
			llmOutput.WriteString("\n")

		case MessageTypeToolCall:
			// 工具调用单独收集
			toolCalls.WriteString(renderToolCall(msg.ToolCall, m.width, msg.ToolExpanded, isSelected))
			toolCalls.WriteString("\n")
		}
	}

	// 先显示 LLM 输出
	sb.WriteString(llmOutput.String())

	// 如果有工具调用，显示分隔区域
	if toolCalls.Len() > 0 {
		sb.WriteString("\n")
		sb.WriteString(renderToolSection(m.width, len(toolIndices)))
		sb.WriteString("\n")
		sb.WriteString(toolCalls.String())
	}

	m.viewport.SetContent(sb.String())

	// 如果不在工具选择模式，自动滚动到底部
	if !m.toolSelectMode {
		m.viewport.GotoBottom()
	}
}
