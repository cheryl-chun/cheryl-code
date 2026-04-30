package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cheryl-chun/cheryl-code/internal/messages"
	"github.com/cheryl-chun/cheryl-code/internal/tools"
	"github.com/openai/openai-go/v3"
)

type Agent struct {
	client   *Client
	manager  *messages.MessageManager
	registry *tools.ToolRegistry

	// state manage
	state *AgentState

	// resume chanel
	resumeCh chan struct{}
}

func NewAgent(client *Client, registry *tools.ToolRegistry) *Agent {
	return &Agent{
		client:   client,
		registry: registry,
		manager:  nil,
		state:    NewAgentState(100),
		resumeCh: make(chan struct{}, 1),
	}
}

func (a *Agent) GetState() *AgentState {
	return a.state
}

func (a *Agent) ApproveToolCall(id string) error {
	return a.state.ApproveToolCall(id)
}

func (a *Agent) RejectToolCall(id string) error {
	return a.state.RejectToolCall(id)
}

func (a *Agent) ApproveAll() error {
	return a.state.ApproveAll()
}

func (a *Agent) ResumeExecution() {
	select {
	case a.resumeCh <- struct{}{}:
	default:
		// TODO: ignore
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

		a.state.Reset()

		a.manager = messages.NewMessageManager()
		a.manager.AddUser(prompt)

		for {
			stream := a.client.ChatStream(ctx, a.manager.GetAll())
			var fullContent string
			toolCallsBuilder := make(map[int64]*ToolCallBuilder)

			// read stream output
			for stream.Next() {
				chunk := stream.Current()

				if len(chunk.Choices) == 0 {
					continue
				}

				delta := chunk.Choices[0].Delta

				// content
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

			// tool calling
			a.state.Status = AgentExecutingTools

			// run tool calls
			allToolStates := []*ToolCallState{}
			hasApprovalRequired := false

			for _, builder := range toolCallsBuilder {
				if !builder.IsComplete() {
					continue
				}

				// parse args
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(builder.Arguments), &args); err != nil {
					eventCh <- StreamEvent{
						Type:  StreamError,
						Error: fmt.Errorf("failed to parse tool args: %v", err),
					}
					continue
				}

				// tool state
				tc := NewToolCallState(builder.ID, builder.Name, args)

				tool, ok := a.registry.Get(builder.Name)
				if !ok {
					eventCh <- StreamEvent{
						Type:  StreamError,
						Error: fmt.Errorf("unknown tool: %s", builder.Name),
					}
					continue
				}

				tc.NeedApproval = tool.RequiresApproval(args)

				// need user approve
				if tc.NeedApproval {
					tc.Transition(ToolStatusPendingApproval)
					a.state.AddToolCall(tc)
					hasApprovalRequired = true

					eventCh <- StreamEvent{
						Type:     StreamApprovalRequired,
						ToolCall: tc,
					}
				} else {
					tc.Transition(ToolStatusRunning)
					a.state.AddToolCall(tc)
				}

				allToolStates = append(allToolStates, tc)

			}
			if hasApprovalRequired {
				select {
				case <-a.resumeCh:
				case <-ctx.Done():
					eventCh <- StreamEvent{
						Type:  StreamError,
						Error: ctx.Err(),
					}
					return
				}
			}

			// execute tool
			var toolCalls []openai.ChatCompletionMessageToolCallUnionParam
			var toolResults []ToolResults

			for _, tc := range allToolStates {
				if tc.Status() != ToolStatusRunning && tc.Status() != ToolStatusApproved {
					continue
				}

				if tc.Status() == ToolStatusRunning {
					tc.Transition(ToolStatusRunning)
					a.state.UpdateToolCallStatus(tc.ID, ToolStatusRunning)
				}

				eventCh <- StreamEvent{
					Type:     StreamToolCall,
					ToolCall: tc,
				}

				result := a.executeToolByName(tc.Name, tc.ArgsRaw, tc.ID)

				tc.Result = result

				if strings.HasPrefix(result, "Error:") || strings.HasPrefix(result, "Failed") {
					tc.Error = fmt.Errorf("%s", result)
					tc.Transition(ToolStatusError)
					a.state.UpdateToolCallStatus(tc.ID, ToolStatusError)
				} else {
					tc.Transition(ToolStatusSuccess)
					a.state.UpdateToolCallStatus(tc.ID, ToolStatusSuccess)
				}

				// 发送工具结果事件
				eventCh <- StreamEvent{
					Type:       StreamToolResult,
					ToolCall:   tc,
					ToolResult: result,
				}

				toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: tc.ArgsRaw,
						},
						Type: "function",
					},
				})

				toolResults = append(toolResults, ToolResults{
					id:     tc.ID,
					result: result,
				})
			}

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
