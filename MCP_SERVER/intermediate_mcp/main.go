package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// Config represents the configuration for the MCP clients
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

type Input struct {
	Query string `json:"query" jsonschema:"required,description=Query to send to MCPs"`
}

func main() {
	// Initialize the MCP server
	server := mcp.NewServer(stdio.NewStdioServerTransport())

	// Create configuration for both MCPs
	cfg := Config{
		MCPStdIOServers: map[string]MCPStdIOConfig{
			"hello-mcp": {
				Command:    "../hello_mcp/hellomcp",
				WorkingDir: ".",
			},
			"external-mcp": {
				Command:    "../external_mcp/externalmcp",
				WorkingDir: ".",
			},
		},
	}

	// Initialize MCP clients
	mcpClientInfo := mcp.ClientInfo{
		Name:    "gateway",
		Version: "1.0.0",
	}

	mcpClients, stdIOCmds := initializeMCPClients(cfg, mcpClientInfo)
	defer shutdownMCPClients(mcpClients, stdIOCmds)

	// Initialize all clients and fetch their tools
	initializeAndListTools(mcpClients)

	// Register the gateway tool
	err := server.RegisterTool("gateway", "Gateway between hello_mcp and external MCPs", func(args Input) (*mcp.ToolResponse, error) {
		return handleGatewayRequest(args, mcpClients)
	})
	if err != nil {
		log.Fatalf("Failed to register gateway tool: %v", err)
	}

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Start the server
	log.Println("Starting the gateway server...")
	go func() {
		if err := server.Serve(); err != nil {
			log.Printf("Server encountered an error: %v", err)
			quit <- syscall.SIGTERM
		}
	}()

	<-quit
	log.Println("Gateway server shutting down gracefully...")
}

func initializeMCPClients(cfg Config, clientInfo mcp.ClientInfo) ([]*mcp.Client, []*exec.Cmd) {
	var mcpClients []*mcp.Client
	var stdIOCmds []*exec.Cmd

	for name, config := range cfg.MCPStdIOServers {
		log.Printf("Initializing StdIO client '%s' with command: %s", name, config.Command)

		cmd := exec.Command(config.Command)
		if config.WorkingDir != "" {
			cmd.Dir = config.WorkingDir
		}
		stdIOCmds = append(stdIOCmds, cmd)

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

		if err := cmd.Start(); err != nil {
			log.Fatalf("Failed to start command '%s': %v", name, err)
		}

		go func(name string) {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				log.Printf("%s stderr: %s", name, scanner.Text())
			}
		}(name)

		client := mcp.NewClientWithInfo(stdio.NewStdioServerTransportWithIO(stdout, stdin), clientInfo)
		mcpClients = append(mcpClients, client)
	}

	return mcpClients, stdIOCmds
}

func initializeAndListTools(mcpClients []*mcp.Client) {
	for i, client := range mcpClients {
		log.Printf("Initializing MCP client %d...", i+1)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, err := client.Initialize(ctx)
		cancel()

		if err != nil {
			log.Printf("Failed to initialize client %d: %v", i+1, err)
			continue
		}

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		cursor := ""
		toolsResponse, err := client.ListTools(ctx, &cursor)
		cancel()

		if err != nil {
			log.Printf("Failed to fetch tools for client %d: %v", i+1, err)
			continue
		}

		log.Printf("Client %d Tools:", i+1)
		for _, tool := range toolsResponse.Tools {
			log.Printf("- %v", tool)
		}
	}
}

func handleGatewayRequest(args Input, mcpClients []*mcp.Client) (*mcp.ToolResponse, error) {
	var responses []string

	for i, client := range mcpClients {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var resp *mcp.ToolResponse
		var err error

		if i == 0 { // hello-mcp
			resp, err = client.CallTool(ctx, "hello", map[string]interface{}{
				"submitter": "gateway",
				"content": map[string]interface{}{
					"title": args.Query,
				},
			})
		} else { // external-mcp
			resp, err = client.CallTool(ctx, "search", map[string]interface{}{
				"query": args.Query,
			})
		}
		cancel()

		if err != nil {
			responses = append(responses, fmt.Sprintf("MCP %d error: %v", i+1, err))
		} else {
			responses = append(responses, fmt.Sprintf("MCP %d: %v", i+1, resp.Content))
		}
	}

	combined := fmt.Sprintf("%s\n%s", responses[0], responses[1])
	return mcp.NewToolResponse(mcp.NewTextContent(combined)), nil
}

func shutdownMCPClients(mcpClients []*mcp.Client, stdIOCmds []*exec.Cmd) {
	log.Println("Shutting down MCP clients...")
	for _, client := range mcpClients {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = client.Ping(ctx)
		cancel()
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
