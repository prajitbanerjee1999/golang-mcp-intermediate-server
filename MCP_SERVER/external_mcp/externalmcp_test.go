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

func String(t mcp.ToolRetType) string {
	return t.Name // Assuming "Name" is a string field of ToolRetType
}

// TestInitializeAndMockTools ensures that MCP clients initialize correctly and fetch tools (mocked).
func TestInitializeAndMockTools(t *testing.T) {
	// Mock MCP client configuration
	config := Config{
		MCPStdIOServers: map[string]MCPStdIOConfig{
			"test-client": {
				Command: "npx",
				Args:    []string{"-y", "@mock/command"},
				Env: map[string]string{
					"MOCK_ENV": "mock_value",
				},
				WorkingDir: ".",
			},
		},
	}

	// Initialize a mock client and process
	var client *mcp.Client
	var process *os.Process
	for name, cfg := range config.MCPStdIOServers {
		fmt.Printf("Setting up mock client '%s'...\n", name)
		mockClient, mockProcess := initializeMockClient(t, cfg)
		client = mockClient
		process = mockProcess
		defer func(process *os.Process) {
			err := process.Kill()
			if err != nil {

			}
		}(process) // Ensure the process is killed after the test
	}

	// Simulate initialization of the client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Failed to initialize mock client: %v", err)
	}

	// Mock the ListTools API result
	mockedTools := []string{"MockTool1", "MockTool2", "MockTool3"}
	mockListTools(client, mockedTools)

	// Fetch the tools and print them in the terminal
	tools, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to fetch tools: %v", err)
	}

	// Print tools in terminal
	fmt.Println("Fetched Tools:")
	for _, tool := range tools.Tools {
		fmt.Println("-", tool)
	}

	// Validate that the tools are as expected
	if len(tools.Tools) != len(mockedTools) {
		t.Fatalf("Expected %d tools, got %d", len(mockedTools), len(tools.Tools))
	}

	for i, tool := range tools.Tools {
		if tool.Name != mockedTools[i] { // Compare the relevant field (e.g., Name)
			t.Errorf("Expected tool %d to be '%s', got '%s'", i, mockedTools[i], tool.Name)
		}
	}

}

// initializeMockClient initializes a mock MCP client.
func initializeMockClient(t *testing.T, cfg MCPStdIOConfig) (*mcp.Client, *os.Process) {
	t.Helper()

	// Create a mocked process
	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = append(cmd.Env, formatEnv(cfg.Env)...)

	// Create pipes for mock communication
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failedvvvv to create stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}

	// Start the mocked process
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Initialize the MCP client with mock I/O
	client := mcp.NewClientWithInfo(stdio.NewStdioServerTransportWithIO(stdout, stdin), mcp.ClientInfo{
		Name:    "mock-client",
		Version: "1.0.0",
	})

	return client, cmd.Process
}

// mockListTools mocks the ListTools method by setting up a fake response.
func mockListTools(client *mcp.Client, tools []string) func(ctx context.Context, cursor *string) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, cursor *string) (*mcp.ToolResponse, error) {
		return &mcp.ToolResponse{}, nil
	}
}

// formatEnv converts a map of environment variables into a slice of strings.
func formatEnv(env map[string]string) []string {
	var formatted []string
	for key, value := range env {
		formatted = append(formatted, fmt.Sprintf("%s=%s", key, value))
	}
	return formatted
}
