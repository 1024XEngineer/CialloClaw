package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"minimal-agent/config"
	"minimal-agent/pkg/agent"
	"minimal-agent/pkg/providers"
)

func main() {
	fmt.Println("=== Minimal Agent ===")
	fmt.Println("A simple AI agent based on PicoClaw core")
	fmt.Println()

	// Find config file
	configPaths := []string{
		"config/config.yaml",
		"../config/config.yaml",
	}
	
	exePath, _ := os.Executable()
	if exePath != "" {
		exeDir := filepath.Dir(exePath)
		configPaths = append(configPaths, filepath.Join(exeDir, "config", "config.yaml"))
	}

	var cfg *config.Config
	var loadedPath string
	for _, path := range configPaths {
		var err error
		cfg, err = config.Load(path)
		if err == nil {
			loadedPath = path
			break
		}
	}

	if loadedPath == "" {
		fmt.Println("Note: Config file not found, using environment variables...\n")
		runWithEnvVars()
		return
	}

	fmt.Printf("Loaded config from: %s\n", loadedPath)

	// Validate API key
	apiKey := cfg.Provider.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("DASHSCOPE_API_KEY")
		}
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	}

	if apiKey == "" {
		fmt.Println("\nEdit config/config.yaml to set your API key:")
		fmt.Println("  provider:")
		fmt.Println("    api_key: \"your-api-key\"")
		fmt.Println("    api_base: \"https://your-endpoint\"")
		fmt.Println("    model: \"your-model\"")
		fmt.Println("\nOr set API_KEY environment variable")
		os.Exit(1)
	}

	// Create provider
	model := cfg.Provider.Model
	if model == "" {
		model = os.Getenv("MODEL")
	}
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	apiBase := cfg.Provider.APIBase
	if apiBase == "" {
		apiBase = os.Getenv("API_BASE")
	}
	if apiBase == "" {
		apiBase = "https://api.openai.com/v1"
	}

	provider := providers.NewOpenAIProvider(apiKey, apiBase, model)
	providerName := provider.GetName()

	// Get workspace and skills directory
	workspace := cfg.Workspace.Path
	if workspace == "" {
		workspace = "workspace"
	}
	skillsDir := filepath.Join(workspace, "skills")

	// Get system prompt
	systemPrompt := cfg.Agent.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = getDefaultSystemPrompt()
	}

	runAgent(provider, providerName, systemPrompt, workspace, skillsDir)
}

func runWithEnvVars() {
	apiKey := os.Getenv("API_KEY")
	apiBase := os.Getenv("API_BASE")
	model := os.Getenv("MODEL")

	if apiBase == "" {
		apiBase = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	if apiKey == "" {
		fmt.Println("Running in echo mode (no LLM)")
		fmt.Println("\nSet API_KEY, API_BASE, MODEL environment variables")
		runEchoMode("workspace")
		return
	}

	provider := providers.NewOpenAIProvider(apiKey, apiBase, model)
	workspace := "workspace"
	skillsDir := filepath.Join(workspace, "skills")
	runAgent(provider, provider.GetName(), getDefaultSystemPrompt(), workspace, skillsDir)
}

func runEchoMode(workspace string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	skillsDir := filepath.Join(workspace, "skills")
	al := agent.NewAgentLoop(workspace, skillsDir)

	go func() {
		al.Run(ctx, nil, "Echo mode - no LLM provider")
	}()

	fmt.Println("Type a message (Ctrl+C to exit):")
	for {
		fmt.Print("> ")
		var input string
		fmt.Scanln(&input)
		if input == "exit" || input == "quit" {
			break
		}
		al.In() <- input
		resp := <-al.Out()
		fmt.Printf("Agent: %s\n", resp)
	}
}

func runAgent(provider providers.Provider, providerName string, systemPrompt string, workspace string, skillsDir string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize agent with workspace and skills
	al := agent.NewAgentLoop(workspace, skillsDir)

	// Start agent loop
	go func() {
		if err := al.Run(ctx, provider, systemPrompt); err != nil {
			fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
		}
	}()

	// Show available tools
	tools := al.GetTools()
	fmt.Printf("Connected to %s\n", providerName)
	fmt.Printf("Loaded %d tools\n", len(tools))
	for name := range tools {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println("\nType a message and press Enter (Ctrl+C to exit):\n")

	for {
		fmt.Print("> ")
		var input string
		fmt.Scanln(&input)
		if input == "exit" || input == "quit" {
			break
		}

		al.In() <- input
		resp := <-al.Out()
		fmt.Printf("Agent: %s\n", resp)
	}

	fmt.Println("Goodbye!")
}

func getDefaultSystemPrompt() string {
	return `You are a helpful AI assistant.

You have access to tools that you can use to help the user:

1. shell: Execute shell commands
   - Takes a "command" parameter
   - Returns the command output

Guidelines:
- Be concise and helpful
- If you need to run commands or access files, use the shell tool
- Explain what you're doing when using tools
- After getting tool results, summarize the findings for the user`
}
