package main

import (
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

type ToolRequest struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

var (
	helloClient    *mcp.Client
	externalClient *mcp.Client
)

func main() {
	// Initialize the intermediate server
	server := mcp.NewServer(stdio.NewStdioServerTransport())

	// Start and initialize clients for both MCPs
	setupClients()

	// Register the tools
	if err := server.RegisterTool("tools/call", "Tool wrapper", handleToolCall); err != nil {
		log.Fatalf("Failed to register tool wrapper: %v", err)
	}

	// Handle shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server
	go func() {
		log.Println("Starting intermediate server...")
		if err := server.Serve(); err != nil {
			log.Printf("Server error: %v", err)
			stop <- syscall.SIGTERM
		}
	}()

	<-stop
}

func setupClients() {
	// Start HelloMCP
	helloCmd := exec.Command("../hello_mcp/hellomcp")
	helloStdin, err := helloCmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to create stdin pipe for hellomcp: %v", err)
	}
	helloStdout, err := helloCmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to create stdout pipe for hellomcp: %v", err)
	}

	if err := helloCmd.Start(); err != nil {
		log.Fatalf("Failed to start hellomcp: %v", err)
	}

	// Start ExternalMCP
	externalCmd := exec.Command("../external_mcp/externalmcp")
	externalStdin, err := externalCmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to create stdin pipe for externalmcp: %v", err)
	}
	externalStdout, err := externalCmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to create stdout pipe for externalmcp: %v", err)
	}

	if err := externalCmd.Start(); err != nil {
		log.Fatalf("Failed to start externalmcp: %v", err)
	}

	// Give the servers time to start
	time.Sleep(2 * time.Second)

	// Create and initialize clients
	helloClient = mcp.NewClientWithInfo(
		stdio.NewStdioServerTransportWithIO(helloStdout, helloStdin),
		mcp.ClientInfo{Name: "hello-client", Version: "1.0.0"},
	)
	externalClient = mcp.NewClientWithInfo(
		stdio.NewStdioServerTransportWithIO(externalStdout, externalStdin),
		mcp.ClientInfo{Name: "external-client", Version: "1.0.0"},
	)

	// Initialize both clients with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := helloClient.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize hello client: %v", err)
	}

	if _, err := externalClient.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize external client: %v", err)
	}

	log.Println("Both clients initialized successfully")
}

func handleToolCall(req ToolRequest) (*mcp.ToolResponse, error) {
	log.Printf("Received tool call request: %s", req.Name)
	ctx := context.Background()

	// First, check if the tool exists in HelloMCP by listing its tools
	cursor := ""
	toolsList, err := helloClient.ListTools(ctx, &cursor)
	if err == nil {
		// Check if the requested tool is in HelloMCP
		toolExists := false
		for _, tool := range toolsList.Tools {
			if name := tool.Name; name == req.Name {
				toolExists = true
				break
			}
		}

		// If tool exists in HelloMCP, try to call it
		if toolExists {
			resp, err := helloClient.CallTool(ctx, req.Name, req.Arguments)
			if err == nil {
				log.Printf("HelloMCP successfully handled tool: %s", req.Name)
				return resp, nil
			}
			log.Printf("HelloMCP failed to handle existing tool %s: %v", req.Name, err)
		}
	}

	// List available tools from ExternalMCP
	tools, err := externalClient.ListTools(ctx, &cursor)
	if err != nil {
		log.Fatalf("Failed to list ExternalMCP tools: %v", err)
	}
	log.Printf("Available ExternalMCP tools: %+v", tools.Tools)

	// If the tool wasn't found in HelloMCP or failed, pass to ExternalMCP through its tools/call wrapper
	log.Printf("Forwarding request to ExternalMCP: %s", req.Name)
	resp, err := externalClient.CallTool(ctx, "tools/call", map[string]interface{}{
		"name":      req.Name,
		"arguments": req.Arguments,
	})
	if err == nil {
		log.Printf("ExternalMCP successfully handled tool: %s", req.Name)
		return resp, nil
	}
	log.Printf("ExternalMCP failed to handle tool %s: %v", req.Name, err)

	return nil, fmt.Errorf("no server could handle the tool %s", req.Name)
}
