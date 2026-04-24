package messages

import "github.com/openai/openai-go/v3"

type MessageManager struct {
	messages []openai.ChatCompletionMessageParamUnion
}

func NewMessageManager() *MessageManager{
	return &MessageManager{
		messages: make([]openai.ChatCompletionMessageParamUnion, 0),
	}
}

func (m *MessageManager) AddUser(content string) {
	m.messages = append(m.messages, openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(content),
			},
		},
	})
}

func (m *MessageManager) AddTool(content string, toolCallId string) {
	m.messages = append(m.messages, openai.ChatCompletionMessageParamUnion{
		OfTool: &openai.ChatCompletionToolMessageParam{
			Content: openai.ChatCompletionToolMessageParamContentUnion{
				OfString: openai.String(content),
			},
			ToolCallID: toolCallId,
		},
	})
}

func (m *MessageManager) AddAssistant(content string, toolCalls []openai.ChatCompletionMessageToolCallUnionParam) {
	m.messages = append(m.messages, openai.ChatCompletionMessageParamUnion{
		OfAssistant: &openai.ChatCompletionAssistantMessageParam{
			Content: openai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: openai.String(content),
			},
			ToolCalls: toolCalls,
		},
	})
}

func (m *MessageManager) GetAll() []openai.ChatCompletionMessageParamUnion {
	return m.messages
}