package tools

import (
	"io"
	"os"
	"path/filepath"
)

var _ Tool = (*ReadTool)(nil)

type ReadTool struct {
	rootPath string
}

func (t *ReadTool) Name() string {
	return "Read"
}

func NewReadTool(rootPath string) *ReadTool {
	return &ReadTool{
		rootPath: rootPath,
	}
}

func (t *ReadTool) Description() string {
	return "Read and return the contents of a file"
}

func (t *ReadTool) Parameters() any {
	return map[string]any{
		"type": OBJECT,
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        STRING,
				"description": "The path to the file to read",
			},
		},
		"required": []string{"file_path"},
	}
}

func (t *ReadTool) Execute(args map[string]any) (string, error) {
	fileName := args["file_path"].(string)
	filePath := filepath.Join(t.rootPath, fileName)
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
