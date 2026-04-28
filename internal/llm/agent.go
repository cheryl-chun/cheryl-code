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

func (a *Agent) RunStream(ctx context.Context, prompt string) (<-chan StreamEvent, error) {
	eventCh := make(chan StreamEvent, 100)
	go func() {
		defer close(eventCh)

		defer func() {
			if r := recover(); r != nil {
				eventCh <- StreamEvent{
					Type:  StreamError,
					Error: fmt.Errorf("panic: %v", r),
				}
			}
		}()

		a.manager = messages.NewMessageManager()
		a.manager.AddUser(prompt)

		for {
			stream := a.client.ChatStream(ctx, a.manager.GetAll())
			var fullContent string
			toolCallsBuilder := make(map[int64]*ToolCallBuilder)

			// 读取流式响应
			for stream.Next() {
				chunk := stream.Current()

				if len(chunk.Choices) == 0 {
					continue
				}

				delta := chunk.Choices[0].Delta

				if delta.Content != "" {
					fullContent += delta.Content
					eventCh <- StreamEvent{
						Type:    StreamContent,
						Content: delta.Content,
					}
				}

				// handle tool calls
				if len(delta.ToolCalls) > 0 {
					for _, tc := range delta.ToolCalls {
						idx := tc.Index

						if toolCallsBuilder[idx] == nil {
							toolCallsBuilder[idx] = NewToolCallBuilder()
						}

						toolCallsBuilder[idx].Append(tc)
					}
				}
			}

			if err := stream.Err(); err != nil {
				eventCh <- StreamEvent{
					Type:  StreamError,
					Error: err,
				}
				stream.Close()
				return
			}

			// no tool calls
			if len(toolCallsBuilder) == 0 {
				eventCh <- StreamEvent{Type: StreamDone}
				return
			}

			// run tool calls
			var toolCalls []openai.ChatCompletionMessageToolCallUnionParam
			var toolResults []ToolResults
			for _, builder := range toolCallsBuilder {
				if !builder.IsComplete() {
					continue
				}

				// parse args
				var args map[string]interface{}
				json.Unmarshal([]byte(builder.Arguments), &args)

				// execute tool
				result := a.executeToolByName(builder.Name, builder.Arguments, builder.ID)

				eventCh <- StreamEvent{
					Type:     StreamToolCall,
					ToolName: builder.Name,
					ToolArgs: args,
				}

				// save tool call result
				toolCalls = append(toolCalls, builder.Build())
				toolResults = append(toolResults, ToolResults{
					id:     builder.ID,
					result: result,
				})
			}
			// add agent message
			a.manager.AddAssistant(fullContent, toolCalls)

			// add tool message
			for _, tr := range toolResults {
				a.manager.AddTool(tr.result, tr.id)
			}
		}
	}()
	return eventCh, nil
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

func (a *Agent) executeToolByName(name, arguments, id string) string {
	tool, ok := a.registry.Get(name)
	if !ok {
		return fmt.Sprintf("Unknown tool: %s", name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return fmt.Sprintf("Failed to parse tool arguments: %v", err)
	}

	result, err := tool.Execute(args)
	if err != nil {
		return fmt.Sprintf("Failed to execute tool: %v", err)
	}
	return result
}
