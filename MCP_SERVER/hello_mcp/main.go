package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

type Input struct {
	Query string `json:"query" jsonschema:"required,description=Query to send to children"`
}

func main() {
	server := mcp.NewServer(stdio.NewStdioServerTransport())
	err := server.RegisterTool("relay", "Relay query to hello and weather MCPs", func(args Input) (*mcp.ToolResponse, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Start hello-mcp subprocess
		helloCmd := exec.Command("/Users/prajit/Desktop/mcp_server/hello_mcp/hello-mcp")
		helloIn, _ := helloCmd.StdinPipe()
		helloOut, _ := helloCmd.StdoutPipe()
		helloCmd.Start()
		helloTransport := stdio.NewStdioClientTransport(helloOut, helloIn)
		helloClient := mcp.NewClient(helloTransport)
		helloClient.Initialize(ctx)

		helloArgs := map[string]any{
			"submitter": "intermediate",
			"content": map[string]any{
				"title": args.Query,
			},
		}
		helloResp, err := helloClient.CallTool(ctx, "hello", helloArgs)
		if err != nil {
			return nil, fmt.Errorf("hello call failed: %w", err)
		}

		// Start weather-mcp subprocess
		weatherCmd := exec.Command("/Users/prajit/Desktop/mcp_server/weather_mcp/weather-mcp")
		weatherIn, _ := weatherCmd.StdinPipe()
		weatherOut, _ := weatherCmd.StdoutPipe()
		weatherCmd.Start()
		weatherTransport := stdio.NewStdioClientTransport(weatherOut, weatherIn)
		weatherClient := mcp.NewClient(weatherTransport)
		weatherClient.Initialize(ctx)

		weatherArgs := map[string]any{
			"city": args.Query,
		}
		weatherResp, err := weatherClient.CallTool(ctx, "weather", weatherArgs)
		if err != nil {
			return nil, fmt.Errorf("weather call failed: %w", err)
		}

		combined := fmt.Sprintf(
			"Hello Tool: %s\nWeather Tool: %s",
			helloResp.Content[0].TextContent.Text,
			weatherResp.Content[0].TextContent.Text,
		)
		return mcp.NewToolResponse(mcp.NewTextContent(combined)), nil
	})

	if err != nil {
		panic(err)
	}

	if err := server.Serve(); err != nil {
		panic(err)
	}
}
