package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	openrouter "github.com/hra42/openrouter-go"
)

const (
	defaultModel = "anthropic/claude-sonnet-4-5"
	systemPrompt = "You are a shell command generator. Output only the shell command that fulfills the user's request. No markdown, no code fences, no explanation — just the raw command on a single line."
)

func main() {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: OPENROUTER_API_KEY is not set")
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: ai <natural language request>")
		os.Exit(1)
	}

	userPrompt := strings.Join(os.Args[1:], " ")

	client := openrouter.NewClient(openrouter.WithAPIKey(apiKey))
	messages := []openrouter.Message{
		openrouter.CreateSystemMessage(systemPrompt),
		openrouter.CreateUserMessage(userPrompt),
	}

	resp, err := client.ChatComplete(context.Background(), messages, openrouter.WithModel(defaultModel))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if len(resp.Choices) == 0 {
		fmt.Fprintln(os.Stderr, "error: no choices in response")
		os.Exit(1)
	}

	out, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		fmt.Fprintln(os.Stderr, "error: unexpected response content type")
		os.Exit(1)
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	fmt.Print(out)
}
