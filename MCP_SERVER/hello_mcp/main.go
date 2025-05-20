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

type Input struct {
	Query string `json:"query" jsonschema:"required,description=Query to send to children"`
}

func main() {
	// Initialize the MCP server
	server := mcp.NewServer(stdio.NewStdioServerTransport())

	// Register the custom tool
	err := server.RegisterTool("relay", "Relay query to hello MCP", relayToolHandler)
	if err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}

	// Gracefully handle shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Start the MCP server
	log.Println("Starting the server...")
	go func() {
		if err := server.Serve(); err != nil {
			log.Printf("Server encountered an error: %v", err)
			close(quit) // Exit if Serve fails unexpectedly
		}
	}()

	// Block until a termination signal is received
	<-quit
	log.Println("Server shutting down gracefully.")
}

// relayToolHandler handles the communication with the child "hello" MCP process.
func relayToolHandler(args Input) (*mcp.ToolResponse, error) {
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Example subprocess execution (run the "hello-mcp" child server)
	cmd := "./hello-mcp"
	helloCmd := exec.Command(cmd)
	helloCmd.Stdin = os.Stdin
	helloCmd.Stdout = os.Stdout
	helloCmd.Stderr = os.Stderr

	log.Printf("Starting subprocess: %s", cmd)
	err := helloCmd.Start()
	if err != nil {
		return nil, fmt.Errorf("subprocess start failed: %w", err)
	}

	defer func() {
		log.Println("Cleaning up subprocess...")
		_ = helloCmd.Process.Kill()
		_ = helloCmd.Wait()
	}()

	// Simulated calls to the subprocess (replace with real logic)
	log.Printf("Simulating request relayed to hello subprocess...")
	time.Sleep(2 * time.Second)

	// Return a dummy success response
	return mcp.NewToolResponse(mcp.NewTextContent("Hello MCP relay successful!")), nil
}
