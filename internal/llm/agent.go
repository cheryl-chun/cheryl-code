package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cheryl-chun/cheryl-code/internal/messages"
	"github.com/cheryl-chun/cheryl-code/internal/tools"
	"github.com/openai/openai-go/v3"
)

type Agent struct {
	client   *Client
	manager  *messages.MessageManager
	registry *tools.ToolRegistry
}

func NewAgent(client *Client, registry *tools.ToolRegistry) *Agent {
	return &Agent{
		client:   client,
		registry: registry,
		manager:  nil,
	}
}

func (a *Agent) Run(ctx context.Context, prompt string) (string, error) {
	a.manager = messages.NewMessageManager()
	a.manager.AddUser(prompt)

	// TODO: 没有限制轮数
	for {
		resp, err := a.client.Chat(ctx, a.manager.GetAll())
		if err != nil {
			return "", err
		}

		if !a.client.HasToolCalls(resp) {
			return resp.Choices[0].Message.Content, nil
		}

		a.manager.AddAssistant(resp.Choices[0].Message.Content, resp.Choices[0].Message.ToAssistantMessageParam().ToolCalls)

		for _, toolCall := range resp.Choices[0].Message.ToolCalls {
			result := a.executeTool(toolCall)
			a.manager.AddTool(result, toolCall.ID)
		}
	}

	return "", fmt.Errorf("unexpected error")
}

func (a *Agent) executeTool(toolCall openai.ChatCompletionMessageToolCallUnion) string {
	tool, ok := a.registry.Get(toolCall.Function.Name)
	if !ok {
		return fmt.Sprintf("Unknown tool: %s", toolCall.Function.Name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("Failed to parse tool arguments: %v", err)
	}
	result, err := tool.Execute(args)
	if err != nil {
		return fmt.Sprintf("Failed to execute tool: %v", err)
	}
	return result
}
