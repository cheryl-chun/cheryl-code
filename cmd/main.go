package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cheryl-chun/cheryl-code/internal/config"
	"github.com/cheryl-chun/cheryl-code/internal/llm"
	"github.com/cheryl-chun/cheryl-code/internal/tools"
	"github.com/cheryl-chun/cheryl-code/internal/tui"
)

func main() {
	var (
		configPath string
	)

	flag.StringVar(&configPath, "config", "configs/config_intsig.yaml", "Path to config file")
	flag.Parse()

	// Load config
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve config path: %v\n", err)
		os.Exit(1)
	}

	if err := config.Load(absPath, config.WithEnv("CHERYL_")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	cfg := config.Get()

	// Validate config
	if cfg.Llm.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: LLM API key not configured")
		os.Exit(1)
	}

	if cfg.Llm.BaseURL == "" {
		fmt.Fprintln(os.Stderr, "Error: LLM base URL not configured")
		os.Exit(1)
	}

	if cfg.Llm.Model == "" {
		fmt.Fprintln(os.Stderr, "Error: LLM model not configured")
		os.Exit(1)
	}

	// Initialize and run agent
	registry := tools.GetDefaultRegistry()
	client := llm.NewClient(cfg.Llm.APIKey, cfg.Llm.BaseURL, cfg.Llm.Model, registry.ToOpenAITools())
	agent := llm.NewAgent(client, registry)

	if err := tui.Run(agent); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
