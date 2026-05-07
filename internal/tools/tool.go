package tools

import (
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

const (
	STRING  = "string"
	NUMBER  = "number"
	INTEGER = "integer"
	BOOLEAN = "boolean"
	OBJECT  = "object"
	ARRAY   = "array"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() any
	Execute(args map[string]any) (string, error)
	RequiresApproval(args map[string]any) bool
}

type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

func GetDefaultRegistry() *ToolRegistry {
	registry := NewToolRegistry()

	rootPath, err := os.Getwd()
	if err != nil {
		rootPath = "."
	}

	registry.Register(NewReadTool(rootPath))
	registry.Register(NewWriteTool(rootPath))
	registry.Register(NewBashTool())
	registry.Register(NewGlobTool(rootPath))

	return registry
}

func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *ToolRegistry) GetAll() []Tool {
	toolList := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		toolList = append(toolList, tool)
	}
	return toolList
}

func (r *ToolRegistry) ToOpenAITools() []openai.ChatCompletionToolUnionParam {
	tools := r.GetAll()
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		params := tool.Parameters().(map[string]any)

		result = append(result, openai.ChatCompletionFunctionTool(
			openai.FunctionDefinitionParam{
				Name:        tool.Name(),
				Description: openai.String(tool.Description()),
				Parameters:  shared.FunctionParameters(params),
			},
		))
	}
	return result
}
