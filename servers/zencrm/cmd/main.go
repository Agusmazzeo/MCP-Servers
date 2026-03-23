package main

import (
	"flag"
	"log"
	"os"

	"github.com/Agusmazzeo/ZenCRM/app/observability"
	"github.com/Agusmazzeo/zencrm-mcp/internal/apiclient"
	"github.com/Agusmazzeo/zencrm-mcp/internal/config"
	"github.com/Agusmazzeo/zencrm-mcp/internal/handlers"
	"github.com/Agusmazzeo/zencrm-mcp/internal/server"
	"github.com/Agusmazzeo/zencrm-mcp/internal/tools"
)

// getToolsPath returns the tools.json path from TOOLS_PATH env var or default
func getToolsPath() string {
	if path := os.Getenv("TOOLS_PATH"); path != "" {
		return path
	}
	return "tools/tools.json" // default for local development
}

func main() {
	log.SetOutput(os.Stderr) // Log to stderr so it doesn't interfere with stdio communication

	// Parse command-line flags
	mode := flag.String("mode", "stdio", "Transport mode: stdio or http")
	port := flag.Int("port", 8080, "HTTP port (only used in http mode)")
	toolsPath := flag.String("tools", getToolsPath(), "Path to tools.json file")
	flag.Parse()

	// 1. Load configuration
	log.Println("Loading configuration...")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("✓ Configuration loaded (API: %s)", cfg.APIBaseURL)

	// 2. Initialize observability
	log.Println("Initializing observability...")
	obs, err := observability.New(cfg.Observability)
	if err != nil {
		log.Fatalf("Failed to initialize observability: %v", err)
	}
	defer obs.Stop()
	obs.Logger.Info("Observability initialized",
		"service", "zencrm-mcp",
		"version", "1.0.0",
		"api_url", cfg.APIBaseURL,
	)

	// 3. Create API client
	obs.Logger.Info("Creating API client")
	client := apiclient.NewClient(cfg)
	obs.Logger.Info("API client created")

	// 4. Load tool definitions
	obs.Logger.Info("Loading tool definitions", "path", *toolsPath)
	toolDefs, err := tools.LoadTools(*toolsPath)
	if err != nil {
		obs.Logger.Error("Failed to load tools", "error", err, "path", *toolsPath)
		log.Fatalf("Failed to load tools: %v", err)
	}
	obs.Logger.Info("Tool definitions loaded", "count", len(toolDefs))

	// 5. Create MCP server
	obs.Logger.Info("Creating MCP server")
	mcpServer := server.NewServer(&server.ServerConfig{
		Name:    "zencrm",
		Version: "1.0.0",
	})
	obs.Logger.Info("MCP server created")

	// 6. Create handler factory and register tools
	obs.Logger.Info("Registering tools with MCP server", "count", len(toolDefs))
	handlerFactory := handlers.NewHandlerFactory(client, cfg.FrontendBaseURL)
	mcpServer.RegisterTools(handlerFactory, toolDefs, true) // true = include login tool
	obs.Logger.Info("All tools registered successfully", "count", len(toolDefs))

	// 7. Run server based on transport mode
	obs.Logger.Info("========================================")
	obs.Logger.Info("ZenCRM MCP Server Starting",
		"version", "1.0.0",
		"mode", *mode,
		"port", *port,
		"tools", len(toolDefs),
		"api", cfg.APIBaseURL,
		"frontend", cfg.FrontendBaseURL,
		"environment", cfg.Observability.Environment,
	)
	obs.Logger.Info("========================================")

	switch *mode {
	case "stdio":
		obs.Logger.Info("Ready to accept MCP requests!")
		if err := mcpServer.RunStdio(); err != nil {
			obs.Logger.Error("Server error", "error", err)
			log.Fatalf("Server error: %v", err)
		}

	case "http":
		obs.Logger.Info("Starting HTTP server", "port", *port)
		if err := mcpServer.RunHTTP(*port); err != nil {
			obs.Logger.Error("Server error", "error", err)
			log.Fatalf("Server error: %v", err)
		}

	default:
		log.Fatalf("Invalid mode: %s (must be 'stdio' or 'http')", *mode)
	}

	obs.Logger.Info("Server stopped gracefully")
}
