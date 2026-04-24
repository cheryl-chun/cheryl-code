package tools

import (
	"fmt"
	"os"
	"path/filepath"
)

var _ Tool = (*WriteTool)(nil)

type WriteTool struct {
	rootPath string
}

func NewWriteTool(rootPath string) *WriteTool {
	return &WriteTool{
		rootPath: rootPath,
	}
}

// Description implements [Tool].
func (w *WriteTool) Description() string {
	return "Write content to a file"
}

// Execute implements [Tool].
func (w *WriteTool) Execute(args map[string]any) (string, error) {
	fileName := args["file_path"].(string)
	content := args["content"].(string)

	filePath := filepath.Join(w.rootPath, fileName)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Content written to %s", filePath), nil
}

// Name implements [Tool].
func (w *WriteTool) Name() string {
	return "Write"
}

// Parameters implements [Tool].
func (w *WriteTool) Parameters() any {
	return map[string]any{
		"type": OBJECT,
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        STRING,
				"description": "The path of the file to write to",
			},
			"content": map[string]any{
				"type":        STRING,
				"description": "The content to write to the file",
			},
		},
		"required": []string{"file_path", "content"},
	}
}
