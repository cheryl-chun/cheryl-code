package llm

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

type Client struct {
	openaiClient openai.Client
	model        string
	tools []openai.ChatCompletionToolUnionParam
}

func NewClient(apiKey, baseUrl, model string, tools []openai.ChatCompletionToolUnionParam) *Client {
	return &Client{
		openaiClient: openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseUrl),
		),
		model: model,
		tools: tools,
	}
}

func (c *Client) Chat(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (*openai.ChatCompletion, error) {
	return c.openaiClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
        Model:    c.model,
        Messages: messages,
        Tools:    c.tools,
    })
}

func (c *Client) ChatStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (*ssestream.Stream[openai.ChatCompletionChunk]) {
	stream := c.openaiClient.Chat.Completions.NewStreaming(
		ctx, openai.ChatCompletionNewParams{
			Model: c.model,
			Messages: messages,
			Tools: c.tools,
		},
	)
	return stream
}

func (c *Client) HasToolCalls(resp *openai.ChatCompletion) bool {
	return len(resp.Choices) > 0 && len(resp.Choices[0].Message.ToolCalls) > 0
}