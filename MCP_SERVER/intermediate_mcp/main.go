package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

type Input struct {
	Query string `json:"query" jsonschema:"required,description=Query to forward to both hello and weather MCPs"`
}

func main() {
	done := make(chan struct{})
	server := mcp.NewServer(stdio.NewStdioServerTransport())

	err := server.RegisterTool("relay", "Relay query to hello and weather MCPs", func(args Input) (*mcp.ToolResponse, error) {
		ctx := context.Background()

		// === HELLO MCP ===
		helloCmd := exec.Command("/Users/prajit/Desktop/mcp_server/hello_mcp/hello-mcp")
		helloIn, _ := helloCmd.StdinPipe()
		helloOut, _ := helloCmd.StdoutPipe()
		if err := helloCmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start hello-mcp: %w", err)
		}

		helloTransport := stdio.NewTransport(helloOut, helloIn)
		helloClient := mcp.NewClient(helloTransport)
		if _, err := helloClient.Initialize(ctx); err != nil {
			return nil, fmt.Errorf("hello client init error: %w", err)
		}

		helloArgs := map[string]interface{}{
			"submitter": "intermediate",
			"content": map[string]interface{}{
				"title": args.Query,
			},
		}
		helloResp, err := helloClient.CallTool(ctx, "hello", helloArgs)
		if err != nil {
			switch {
			case errors.Is(err, mcp.ErrClientNotInitialized):
				return nil, fmt.Errorf("hello client not initialized: %w", err)
			default:
				return nil, fmt.Errorf("hello call error: %w", err)
			}
		}

		// === WEATHER MCP ===
		weatherCmd := exec.Command("/Users/prajit/Desktop/mcp_server/weather_mcp/weather-mcp")
		weatherIn, _ := weatherCmd.StdinPipe()
		weatherOut, _ := weatherCmd.StdoutPipe()
		if err := weatherCmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start weather-mcp: %w", err)
		}

		weatherTransport := stdio.NewTransportWithStreams(weatherIn, weatherOut)
		weatherClient := mcp.NewClient(weatherTransport)
		if _, err := weatherClient.Initialize(ctx); err != nil {
			return nil, fmt.Errorf("weather client init error: %w", err)
		}

		weatherArgs := map[string]interface{}{
			"city": args.Query,
		}
		weatherResp, err := weatherClient.CallTool(ctx, "weather", weatherArgs)
		if err != nil {
			switch {
			case errors.Is(err, mcp.ErrClientNotInitialized):
				return nil, fmt.Errorf("weather client not initialized: %w", err)
			default:
				return nil, fmt.Errorf("weather call error: %w", err)
			}
		}

		combined := fmt.Sprintf("Hello response: %v\nWeather response: %v", helloResp.Content, weatherResp.Content)
		return mcp.NewToolResponse(mcp.NewTextContent(combined)), nil
	})

	if err != nil {
		panic(err)
	}

	if err := server.Serve(); err != nil {
		panic(err)
	}

	<-done
}
