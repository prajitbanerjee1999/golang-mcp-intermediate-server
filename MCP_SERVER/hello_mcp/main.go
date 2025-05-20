package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// Input types for different tools
type BasicInput struct {
	Query string `json:"query" jsonschema:"required,description=Query string"`
}

type StringInput struct {
	Text string `json:"text" jsonschema:"required,description=Text to process"`
}

type CalcInput struct {
	Numbers []float64 `json:"numbers" jsonschema:"required,description=List of numbers"`
}

func main() {
	// Initialize the MCP server
	server := mcp.NewServer(stdio.NewStdioServerTransport())

	// Register tools
	tools := []struct {
		name        string
		description string
		handler     interface{}
	}{
		{"echo", "Echo the input text", echoHandler},
		{"reverse", "Reverse the input text", reverseHandler},
		{"calculate", "Perform calculations", calculateHandler},
		{"timestamp", "Get current timestamp", timestampHandler},
	}

	for _, tool := range tools {
		if err := server.RegisterTool(tool.name, tool.description, tool.handler); err != nil {
			log.Fatalf("Failed to register %s tool: %v", tool.name, err)
		}
		log.Printf("Registered tool: %s", tool.name)
	}

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the MCP server
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

func echoHandler(args StringInput) (*mcp.ToolResponse, error) {
	result := strings.TrimSpace(args.Text)
	return mcp.NewToolResponse(mcp.NewTextContent(result)), nil
}

func reverseHandler(args StringInput) (*mcp.ToolResponse, error) {
	runes := []rune(args.Text)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return mcp.NewToolResponse(mcp.NewTextContent(string(runes))), nil
}

func calculateHandler(args CalcInput) (*mcp.ToolResponse, error) {
	if len(args.Numbers) == 0 {
		return nil, fmt.Errorf("no numbers provided")
	}

	sum := 0.0
	for _, n := range args.Numbers {
		sum += n
	}
	avg := sum / float64(len(args.Numbers))

	result := fmt.Sprintf("Sum: %.2f\nAverage: %.2f", sum, avg)
	return mcp.NewToolResponse(mcp.NewTextContent(result)), nil
}

func timestampHandler(args BasicInput) (*mcp.ToolResponse, error) {
	now := time.Now()
	result := fmt.Sprintf("Current time: %s\nUnix: %d",
		now.Format(time.RFC3339),
		now.Unix())
	return mcp.NewToolResponse(mcp.NewTextContent(result)), nil
}
