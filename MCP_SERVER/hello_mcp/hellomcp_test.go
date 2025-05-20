package main

import (
	"testing"
)

func TestRelayToolHandler(t *testing.T) {
	// Mock input
	input := Input{
		Query: "Hello World!",
	}

	// Expected Output
	expected := "Hello MCP relay successful!" // This matches the dummy hello-mcp output

	// Simulate the tool handler
	response, err := relayToolHandler(input)
	if err != nil {
		t.Fatalf("Tool handler failed: %v", err)
	}

	if len(response.Content) == 0 || response.Content[0].TextContent.Text != expected {
		t.Errorf("Expected: %s, but got: %s", expected, response.Content[0].TextContent.Text)
	}
}
