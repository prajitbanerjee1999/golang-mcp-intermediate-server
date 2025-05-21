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

func TestIntermediateServer(t *testing.T) {
	// Start the server process
	cmd := exec.Command("go", "build -o intermediatemcp ")
	if cmd == nil {
		return
	}
	cmd = exec.Command("./intermediatemcp")
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

	// Wait for server to initialize
	time.Sleep(2 * time.Second)

	// Create client with stdio transport
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

	// Test HelloMCP tools
	t.Run("Echo", func(t *testing.T) {
		resp, err := client.CallTool(context.Background(), "tools/call", map[string]interface{}{
			"name": "echo",
			"arguments": map[string]interface{}{
				"text": "Hello, World!",
			},
		})
		if err != nil {
			t.Errorf("Echo failed: %v", err)
		} else {
			fmt.Printf("Echo response: %v\n", resp.Content[0].TextContent.Text)
		}
	})

	t.Run("Calculate", func(t *testing.T) {
		resp, err := client.CallTool(context.Background(), "tools/call", map[string]interface{}{
			"name": "calculate",
			"arguments": map[string]interface{}{
				"numbers": []float64{1, 2, 3, 4, 5},
			},
		})
		if err != nil {
			t.Errorf("Calculate failed: %v", err)
		} else {
			fmt.Printf("Calculate response: %v\n", resp.Content[0].TextContent.Text)
		}
	})

	// Test ExternalMCP tools
	t.Run("ListDirectory", func(t *testing.T) {
		resp, err := client.CallTool(context.Background(), "tools/call", map[string]interface{}{
			"name": "list_directory",
			"arguments": map[string]interface{}{
				"path": ".",
			},
		})
		if err != nil {
			t.Errorf("List directory failed: %v", err)
		} else {
			fmt.Printf("Directory listing: %v\n", resp.Content[0].TextContent.Text)
		}
	})

	t.Run("DirectoryTree", func(t *testing.T) {
		resp, err := client.CallTool(context.Background(), "tools/call", map[string]interface{}{
			"name": "directory_tree",
			"arguments": map[string]interface{}{
				"path": ".",
			},
		})
		if err != nil {
			t.Errorf("Directory tree failed: %v", err)
		} else {
			fmt.Printf("Directory tree: %v\n", resp.Content[0].TextContent.Text)
		}
	})
}
