package main

import (
	"flag"
	"log"
	"os"

	"github.com/Agusmazzeo/MCP-Servers/services/allfunds/internal/config"
	"github.com/Agusmazzeo/MCP-Servers/services/allfunds/internal/graphql"
	"github.com/Agusmazzeo/MCP-Servers/services/allfunds/internal/handlers"
	"github.com/Agusmazzeo/MCP-Servers/services/allfunds/internal/transport"
	"github.com/Agusmazzeo/MCP-Servers/shared/pkg/server"
	"github.com/Agusmazzeo/MCP-Servers/shared/pkg/tools"
)

func main() {
	log.SetOutput(os.Stderr)

	mode := flag.String("mode", "stdio", "Transport mode: stdio or http")
	port := flag.Int("port", 8080, "HTTP port (only used in http mode)")
	flag.Parse()

	// Load config
	log.Println("Loading configuration...")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("✓ Configuration loaded (GraphQL: %s)", cfg.GraphQLURL)

	// Create GraphQL client
	log.Println("Creating GraphQL client...")
	client, err := graphql.NewClient(cfg.GraphQLURL, cfg.Email, cfg.Password)
	if err != nil {
		log.Fatalf("Failed to create GraphQL client: %v", err)
	}
	log.Println("✓ GraphQL client created")

	// Load tools
	log.Println("Loading tool definitions...")
	toolDefs, err := tools.LoadTools("tools/tools.json")
	if err != nil {
		log.Fatalf("Failed to load tools: %v", err)
	}
	log.Printf("✓ Tool definitions loaded (%d tools)", len(toolDefs))

	// Create MCP server
	log.Println("Creating MCP server...")
	mcpServer := server.NewServer(&server.ServerConfig{
		Name:    "allfunds",
		Version: "1.0.0",
	})
	log.Println("✓ MCP server created")

	// Create handler factory and register tools
	log.Println("Creating handler factory...")
	handlerFactory := handlers.NewHandlerFactory(client)
	mcpServer.RegisterTools(handlerFactory, toolDefs, true)
	log.Printf("✓ Tools registered (%d tools)\n", len(toolDefs))

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
		log.Println("OAuth flow enabled - configure client_id (email) and client_secret (password) in Claude Desktop")
		httpTransport := transport.NewHTTPTransport(*port, toolDefs, cfg.GraphQLURL)
		if err := httpTransport.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}

	default:
		log.Fatalf("Invalid mode: %s (must be 'stdio' or 'http')", *mode)
	}
}
