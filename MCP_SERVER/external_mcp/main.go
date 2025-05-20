package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	Port            string                    `json:"Port"`
	MCPSSEServers   []MCPSSEServerConfig      `json:"MCPSSEServers"`
	MCPStdIOServers map[string]MCPStdIOConfig `json:"MCPStdIOServers"`
}

// MCPSSEServerConfig represents the configuration for an MCP SSE server
type MCPSSEServerConfig struct {
	URL            string `json:"URL"`
	MaxPayloadSize int    `json:"MaxPayloadSize"`
}

// MCPStdIOConfig represents the configuration for an MCP StdIO server
type MCPStdIOConfig struct {
	Command    string            `json:"Command"`
	Args       []string          `json:"Args"`
	Env        map[string]string `json:"Env"`
	WorkingDir string            `json:"WorkingDir"`
}

func main() {
	// Load configuration
	var Port = "7562"
	cfg := loadConfig("mcp.json")

	// Create the MCP client information
	mcpClientInfo := mcp.ClientInfo{
		Name:    "mcp-service",
		Version: "1.0.0",
	}

	// Initialize MCP clients based on mcp.json
	mcpClients, stdIOCmds := initializeMCPClients(cfg, mcpClientInfo)

	// Ensure all clients are initialized and fetch their tools
	initializeAndListTools(mcpClients)

	// Set up graceful shutdown and cleanup
	defer shutdownMCPClients(mcpClients, stdIOCmds)

	// Start a lightweight HTTP server (for demonstration purposes)
	startHTTPServer(Port, mcpClients)
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

// startHTTPServer starts the HTTP server
func startHTTPServer(port string, mcpClients []*mcp.Client) {
	// Create a router to handle different endpoints
	mux := http.NewServeMux()

	// Base endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("MCP Server running"))
	})

	// List all tools endpoint
	mux.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var allTools []map[string]interface{}
		for i, client := range mcpClients {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cursor := ""
			tools, err := client.ListTools(ctx, &cursor)
			cancel()

			if err != nil {
				log.Printf("Error fetching tools from client %d: %v", i+1, err)
				continue
			}

			allTools = append(allTools, map[string]interface{}{
				"clientIndex": i + 1,
				"tools":       tools.Tools,
			})
		}

		json.NewEncoder(w).Encode(allTools)
	})

	// Call specific tool endpoint
	mux.HandleFunc("/call-tool", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request struct {
			ClientIndex int         `json:"clientIndex"`
			ToolName    string      `json:"toolName"`
			Arguments   interface{} `json:"arguments"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if request.ClientIndex <= 0 || request.ClientIndex > len(mcpClients) {
			http.Error(w, "Invalid client index", http.StatusBadRequest)
			return
		}

		client := mcpClients[request.ClientIndex-1]
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		response, err := client.CallTool(ctx, request.ToolName, request.Arguments)
		if err != nil {
			http.Error(w, fmt.Sprintf("Tool call failed: %v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(response)
	})

	// Create the HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("HTTP server starting on port %s...", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down HTTP server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}
