package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// Config represents the configuration for the MCP clients and servers
type Config struct {
	MCPStdIOServers map[string]MCPStdIOConfig `json:"MCPStdIOServers"`
}

// MCPStdIOConfig represents the configuration for an MCP StdIO server
type MCPStdIOConfig struct {
	Command    string            `json:"Command"`
	Args       []string          `json:"Args"`
	Env        map[string]string `json:"Env"`
	WorkingDir string            `json:"WorkingDir"`
}

func main() {
	// Initialize the MCP server with stdio transport
	server := mcp.NewServer(stdio.NewStdioServerTransport())

	// Load configuration
	cfg := loadConfig("mcp.json")

	// Create the MCP client information
	mcpClientInfo := mcp.ClientInfo{
		Name:    "mcp-service",
		Version: "1.0.0",
	}

	// Initialize MCP clients
	mcpClients, stdIOCmds := initializeMCPClients(cfg, mcpClientInfo)
	defer shutdownMCPClients(mcpClients, stdIOCmds)

	// Initialize all clients and fetch their tools
	initializeAndListTools(mcpClients)

	// Register tools with the server
	registerTools(server, mcpClients)

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the server
	go func() {
		log.Println("Starting MCP server...")
		if err := server.Serve(); err != nil {
			log.Printf("Server error: %v", err)
			stop <- syscall.SIGTERM
		}
	}()

	<-stop
	log.Println("Server shutting down gracefully...")
}

// registerTools registers all the tools with the MCP server
func registerTools(server *mcp.Server, mcpClients []*mcp.Client) {
	tools := []struct {
		name        string
		description string
		handler     interface{}
	}{
		{"tools/list", "List all available tools", handleListTools(mcpClients)},
		{"tools/call", "Call a specific tool", handleCallTool(mcpClients)},
	}

	for _, tool := range tools {
		if err := server.RegisterTool(tool.name, tool.description, tool.handler); err != nil {
			log.Fatalf("Failed to register %s tool: %v", tool.name, err)
		}
		log.Printf("Registered tool: %s", tool.name)
	}
}

// Tool handlers
type ListToolsRequest struct {
	Cursor string `json:"cursor"`
}

type CallToolRequest struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

func handleListTools(mcpClients []*mcp.Client) interface{} {
	return func(args ListToolsRequest) (*mcp.ToolResponse, error) {
		var allTools []interface{}
		for _, client := range mcpClients {
			tools, err := client.ListTools(context.Background(), &args.Cursor)
			if err != nil {
				continue
			}
			for _, tool := range tools.Tools {
				allTools = append(allTools, tool)
			}
		}

		// Convert tools to JSON string
		toolsJSON, err := json.Marshal(map[string]interface{}{
			"tools": allTools,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tools: %v", err)
		}

		return &mcp.ToolResponse{
			Content: []*mcp.Content{
				{
					Type: "text",
					TextContent: &mcp.TextContent{
						Text: string(toolsJSON),
					},
				},
			},
		}, nil
	}
}

func handleCallTool(mcpClients []*mcp.Client) interface{} {
	return func(args CallToolRequest) (*mcp.ToolResponse, error) {
		for _, client := range mcpClients {
			resp, err := client.CallTool(context.Background(), args.Name, args.Arguments)
			if err == nil {
				return resp, nil
			}
		}
		return &mcp.ToolResponse{
			Content: []*mcp.Content{
				{
					Type: "text",
					TextContent: &mcp.TextContent{
						Text: "method not found",
					},
				},
			},
		}, nil
	}
}

// loadConfig reads and parses the configuration from the given file path
func loadConfig(filePath string) Config {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			return
		}
	}(file)

	var cfg Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	// Resolve any environment variable placeholders in the configuration
	resolveEnvVariables(&cfg)
	return cfg
}

// resolveEnvVariables replaces ${ENV_VAR} placeholders in the configuration with actual environment variables
func resolveEnvVariables(cfg *Config) {
	for name, server := range cfg.MCPStdIOServers {
		for key, value := range server.Env {
			if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
				envVar := strings.Trim(value, "${}")
				if resolvedValue, found := os.LookupEnv(envVar); found {
					server.Env[key] = resolvedValue
				} else {
					log.Fatalf("Environment variable '%s' is not set", envVar)
				}
			}
		}
		cfg.MCPStdIOServers[name] = server
	}
}

// initializeMCPClients sets up both SSE and StdIO clients based on the configuration
func initializeMCPClients(cfg Config, clientInfo mcp.ClientInfo) ([]*mcp.Client, []*exec.Cmd) {
	var mcpClients []*mcp.Client
	var stdIOCmds []*exec.Cmd

	// Set up StdIO clients
	for name, config := range cfg.MCPStdIOServers {
		log.Printf("Initializing StdIO client '%s' with command: %s", name, config.Command)

		// Start the external process
		cmd := exec.Command(config.Command, config.Args...)
		for key, value := range config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
		stdIOCmds = append(stdIOCmds, cmd)

		// Set up pipes for communication
		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Fatalf("Failed to create stdin pipe for '%s': %v", name, err)
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatalf("Failed to create stdout pipe for '%s': %v", name, err)
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Fatalf("Failed to create stderr pipe for '%s': %v", name, err)
		}

		// Start the external command
		if err := cmd.Start(); err != nil {
			log.Fatalf("Failed to start command '%s': %v", name, err)
		}

		// Log any error output from the command
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				log.Printf("StdIO client '%s' stderr: %s", name, scanner.Text())
			}
		}()

		// Create an StdIO MCP client
		stdIOClient := mcp.NewClientWithInfo(stdio.NewStdioServerTransportWithIO(stdout, stdin), clientInfo)
		mcpClients = append(mcpClients, stdIOClient)
	}

	return mcpClients, stdIOCmds
}

// initializeAndListTools initializes all clients and fetches available tools
func initializeAndListTools(mcpClients []*mcp.Client) {
	for i, client := range mcpClients {
		log.Printf("Initializing MCP client %d...", i+1)

		// Initialize the client
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, err := client.Initialize(ctx)
		cancel()

		if err != nil {
			log.Printf("Failed to initialize client %d: %v", i+1, err)
			continue
		}

		// Fetch tools with empty string cursor instead of nil
		log.Printf("Fetching tools for client %d...", i+1)
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		cursor := "" // Use empty string instead of nil
		toolsResponse, err := client.ListTools(ctx, &cursor)
		cancel()

		if err != nil {
			log.Printf("Failed to fetch tools for client %d: %v", i+1, err)
			continue
		}

		// Print tools
		log.Printf("Client %d Tools:", i+1)
		for _, tool := range toolsResponse.Tools {
			log.Printf("- %v", tool)
		}
	}
}

// shutdownMCPClients gracefully shuts down all MCP clients and StdIO commands
func shutdownMCPClients(mcpClients []*mcp.Client, stdIOCmds []*exec.Cmd) {
	log.Println("Shutting down MCP clients...")
	for _, client := range mcpClients {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := client.Ping(ctx) // Only as an example of cleanup logic
		cancel()
		if err != nil {
			log.Printf("Failed to ping MCP client: %v", err)
		}
	}

	log.Println("Killing StdIO commands...")
	for _, cmd := range stdIOCmds {
		if err := cmd.Process.Kill(); err != nil {
			log.Printf("Failed to kill StdIO command: %v", err)
		}
		err := cmd.Wait()
		if err != nil {
			return
		}
	}
}
