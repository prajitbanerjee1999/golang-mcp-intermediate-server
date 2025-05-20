package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func TestCallTools(t *testing.T) {
	// Start the server process
	cmd := exec.Command("./hellomcp") // assuming the binary is in the same directory
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func(Process *os.Process) {
		err := Process.Kill()
		if err != nil {
			return
		}
	}(cmd.Process)

	// Create client with stdio transport connected to the server
	client := mcp.NewClientWithInfo(
		stdio.NewStdioServerTransportWithIO(stdout, stdin),
		mcp.ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	)

	// Initialize client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err = client.Initialize(ctx)
	cancel()
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Call each tool
	ctx = context.Background()

	// Echo tool
	echoResp, err := client.CallTool(ctx, "echo", map[string]interface{}{
		"text": "Hello, World!",
	})
	if err != nil {
		t.Logf("Echo tool error: %v", err)
	} else {
		fmt.Printf("Echo response: %v\n", echoResp.Content[0].TextContent.Text)
	}

	// Reverse tool
	reverseResp, err := client.CallTool(ctx, "reverse", map[string]interface{}{
		"text": "Hello, World!",
	})
	if err != nil {
		t.Logf("Reverse tool error: %v", err)
	} else {
		fmt.Printf("Reverse response: %v\n", reverseResp.Content[0].TextContent.Text)
	}

	// Calculate tool
	calcResp, err := client.CallTool(ctx, "calculate", map[string]interface{}{
		"numbers": []float64{1, 2, 3, 4, 5},
	})
	if err != nil {
		t.Logf("Calculate tool error: %v", err)
	} else {
		fmt.Printf("Calculate response: %v\n", calcResp.Content[0].TextContent.Text)
	}

	// Timestamp tool
	timeResp, err := client.CallTool(ctx, "timestamp", map[string]interface{}{
		"query": "",
	})
	if err != nil {
		t.Logf("Timestamp tool error: %v", err)
	} else {
		fmt.Printf("Timestamp response: %v\n", timeResp.Content[0].TextContent.Text)
	}
}
