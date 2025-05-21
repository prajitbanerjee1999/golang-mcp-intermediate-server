package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func TestBasicTools(t *testing.T) {
	// Start the server process
	cmd := exec.Command("./externalmcp")
	time.Sleep(5 * time.Second)

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

	// Create client with stdio transport
	client := mcp.NewClientWithInfo(
		stdio.NewStdioServerTransportWithIO(stdout, stdin),
		mcp.ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	)

	// Wait for server to initialize
	time.Sleep(2 * time.Second)

	// Initialize client
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	_, err = client.Initialize(ctx)
	cancel()
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	t.Run("FileSystem", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		resp, err := client.CallTool(ctx, "tools/call", map[string]interface{}{
			"name": "list_directory",
			"arguments": map[string]interface{}{
				"path": ".",
			},
		})
		if err != nil {
			t.Errorf("List directory failed: %v", err)
		} else {
			t.Logf("Directory listing: %s", resp.Content[0].TextContent.Text)
		}
	})

	t.Run("DirectoryTree", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		resp, err := client.CallTool(ctx, "tools/call", map[string]interface{}{
			"name": "directory_tree",
			"arguments": map[string]interface{}{
				"path": ".",
			},
		})
		if err != nil {
			t.Errorf("Directory tree failed: %v", err)
		} else {
			t.Logf("Directory tree: %s", resp.Content[0].TextContent.Text)
		}
	})

	t.Run("VisitPage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := client.CallTool(ctx, "tools/call", map[string]interface{}{
			"name": "visit_page",
			"arguments": map[string]interface{}{
				"url":            "https://www.krakend.io/docs/overview/playground/",
				"takeScreenshot": true,
			},
		})
		if err != nil {
			t.Errorf("Visit Page failed: %v", err)
		} else {
			t.Logf("Visit Page : %s", resp.Content[0].TextContent.Text)
		}
	})

	t.Run("ToolsList", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		resp, err := client.CallTool(ctx, "tools/list", map[string]interface{}{
			"name": "tools/list",
			"arguments": map[string]interface{}{
				"cursor": "",
			},
		})
		if err != nil {
			t.Errorf("List tools failed: %v", err)
		} else {
			t.Logf("Tools response: %s", resp.Content[0].TextContent.Text)
		}
	})
}
