package main

import (
	"flag"
	"log"
	"os"

	"github.com/Agusmazzeo/allfunds-mcp/internal/config"
	"github.com/Agusmazzeo/allfunds-mcp/internal/graphql"
	"github.com/Agusmazzeo/allfunds-mcp/internal/handlers"
	"github.com/Agusmazzeo/allfunds-mcp/internal/server"
	"github.com/Agusmazzeo/allfunds-mcp/internal/tools"
)

// getToolsPath returns the tools.json path from TOOLS_PATH env var or default
func getToolsPath() string {
	if path := os.Getenv("TOOLS_PATH"); path != "" {
		return path
	}
	return "tools/tools.json" // default for local development
}

func main() {
	log.SetOutput(os.Stderr)

	mode := flag.String("mode", "stdio", "Transport mode: stdio or http")
	port := flag.Int("port", 8080, "HTTP port (only used in http mode)")
	toolsPath := flag.String("tools", getToolsPath(), "Path to tools.json file")
	flag.Parse()

	// Load config
	log.Println("Loading configuration...")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("✓ Configuration loaded (GraphQL: %s)", cfg.GraphQLURL)

	// Create GraphQL client (only for stdio mode)
	var client *graphql.Client
	if *mode == "stdio" {
		if cfg.Email == "" || cfg.Password == "" {
			log.Fatalf("ALLFUNDS_EMAIL and ALLFUNDS_PASSWORD environment variables are required for stdio mode")
		}
		log.Println("Creating GraphQL client...")
		client, err = graphql.NewClient(cfg.GraphQLURL, cfg.Email, cfg.Password)
		if err != nil {
			log.Fatalf("Failed to create GraphQL client: %v", err)
		}
		log.Println("✓ GraphQL client created")
	}

	// Load tools
	log.Println("Loading tool definitions...")
	log.Printf("Tools path: %s", *toolsPath)
	toolDefs, err := tools.LoadTools(*toolsPath)
	if err != nil {
		log.Fatalf("Failed to load tools: %v", err)
	}
	log.Printf("✓ Tool definitions loaded (%d tools)", len(toolDefs))

	// Create MCP server
	log.Println("Creating MCP server...")
	mcpServer := server.NewServer(&server.ServerConfig{
		Name:    "allfunds",
		Version: "1.0.0",
	}, cfg.GraphQLURL)
	log.Println("✓ MCP server created with OAuth 2.0 support")
	log.Println("⚠ Configure client_id (email) and client_secret (password) in Claude Desktop OAuth settings")

	// Register tools based on mode
	if *mode == "stdio" {
		// For stdio mode: create handler factory and register tools with MCP server
		log.Println("Creating handler factory...")
		handlerFactory := handlers.NewHandlerFactory(client)
		mcpServer.RegisterTools(handlerFactory, toolDefs, true)
		log.Printf("✓ Tools registered (%d tools)\n", len(toolDefs))
	} else {
		// For HTTP mode: just store tool definitions (tools are executed dynamically with OAuth client)
		mcpServer.SetToolDefinitions(toolDefs)
		log.Printf("✓ Tool definitions loaded (%d tools)\n", len(toolDefs))
	}

	log.Println("========================================")
	log.Println("🚀 Allfunds MCP Server - Production Mode")
	log.Printf("Version: 1.0.0\n")
	log.Printf("Registered %d tools\n", len(toolDefs))
	log.Printf("Mode: %s\n", *mode)
	if *mode == "http" {
		log.Printf("Port: %d\n", *port)
	}
	log.Println("========================================")

	// Run server
	switch *mode {
	case "stdio":
		log.Println("Starting server in stdio mode...")
		log.Println("Ready to accept MCP requests!")
		if err := mcpServer.RunStdio(); err != nil {
			log.Fatalf("Server error: %v", err)
		}

	case "http":
		log.Printf("Starting server in HTTP mode on port %d...\n", *port)
		if err := mcpServer.RunHTTP(*port); err != nil {
			log.Fatalf("Server error: %v", err)
		}

	default:
		log.Fatalf("Invalid mode: %s (must be 'stdio' or 'http')", *mode)
	}
}
