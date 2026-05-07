package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

var _ Tool = (*GlobTool)(nil)

type GlobTool struct {
	rootPath   string
	maxResults int // 最大结果返回数
}

func NewGlobTool(rootPath string) *GlobTool {
	return &GlobTool{
		rootPath:   rootPath,
		maxResults: 100,
	}
}

// Description implements [Tool].
func (g *GlobTool) Description() string {
	return "Search for files matching a glob pattern. Supports ** for recursive matching (e.g., '**/*.go' finds all Go files)."
}

// Execute implements [Tool].
func (g *GlobTool) Execute(args map[string]any) (string, error) {
	pattern := args["pattern"].(string)

	absPattern := filepath.Join(g.rootPath, pattern)

	matches, err := doublestar.FilepathGlob(absPattern)
	if err != nil {
		return "", fmt.Errorf("glob failed: %w", err)
	}

	if len(matches) > g.maxResults {
		matches = matches[:g.maxResults]
	}

	var relMatches []string
	for _, match := range matches {
		// 路径遍历保护
		if !strings.HasPrefix(match, g.rootPath) {
			continue
		}
		rel, err := filepath.Rel(g.rootPath, match)
		if err != nil {
			continue
		}
		relMatches = append(relMatches, rel)
	}

	if len(relMatches) == 0 {
		return "No files matched the pattern.", nil
	}

	return fmt.Sprintf("Found %d file(s):\n%s",
		len(relMatches),
		strings.Join(relMatches, "\n")), nil
}

// Name implements [Tool].
func (g *GlobTool) Name() string {
	return "Glob"
}

// Parameters implements [Tool].
func (g *GlobTool) Parameters() any {
	return map[string]any{
		"type": OBJECT,
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        STRING,
				"description": "Glob pattern to match files (e.g., '*.go', 'src/**/*.ts', 'internal/*/model.go')",
			},
		},
		"required": []string{"pattern"},
	}
}

// RequiresApproval implements [Tool].
func (g *GlobTool) RequiresApproval(args map[string]any) bool {
	return false
}
