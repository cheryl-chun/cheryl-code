package llm

import "github.com/openai/openai-go/v3"

type ToolCallBuilder struct {
	ID        string
	Name      string
	Arguments string
}

type ToolResults struct {
	id     string
	result string
}

func NewToolCallBuilder() *ToolCallBuilder {
	return &ToolCallBuilder{}
}

func (b *ToolCallBuilder) Append(tc openai.ChatCompletionChunkChoiceDeltaToolCall) {
	if tc.ID != "" {
		b.ID = tc.ID
	}

	if tc.Function.Name != "" {
		b.Name += tc.Function.Name
	}

	if tc.Function.Arguments != "" {
		b.Arguments += tc.Function.Arguments
	}
}

func (b *ToolCallBuilder) IsComplete() bool {
	return b.ID != "" && b.Name != "" && b.Arguments != ""
}

func (b *ToolCallBuilder) Build() openai.ChatCompletionMessageToolCallUnionParam {
	return openai.ChatCompletionMessageToolCallUnionParam{
		OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
			ID: b.ID,
			Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
				Name:      b.Name,
				Arguments: b.Arguments,
			},
			Type: "function",
		},
	}
}
