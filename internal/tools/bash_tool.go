package tools

import (
	"os/exec"
	"strings"
)

var _ Tool = (*BashTool)(nil)

type BashTool struct{}

func NewBashTool() *BashTool {
	return &BashTool{}
}

// Description implements [Tool].
func (b *BashTool) Description() string {
	return "Execute a shell command"
}

// Execute implements [Tool].
func (b *BashTool) Execute(args map[string]any) (string, error) {
	command := args["command"].(string)
	content, err := executeCommand(command)
	if err != nil {
		return "", err
	}
	return content, nil
}

// Name implements [Tool].
func (b *BashTool) Name() string {
	return "Bash"
}

// Parameters implements [Tool].
func (b *BashTool) Parameters() any {
	return map[string]any{
		"type": OBJECT,
		"properties": map[string]any{
			"command": map[string]any{
				"type":        STRING,
				"description": "The command to execute",
			},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) RequiresApproval(args map[string]any) bool {
	command, ok := args["command"].(string)
	if !ok {
		return true 
	}

	// only read
	safeCommands := []string{"ls", "pwd", "echo", "cat", "head", "tail", "grep"}

	for _, safe := range safeCommands {
		if strings.HasPrefix(strings.TrimSpace(command), safe+" ") ||
			strings.TrimSpace(command) == safe {
			return false // 安全命令，不需要审批
		}
	}

	// dangerous command
	dangerousPatterns := []string{"rm ", "mv ", "chmod ", "sudo ", "> ", ">>"}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(command, pattern) {
			return true
		}
	}

	return true
}

func executeCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	results, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(results), nil
}
