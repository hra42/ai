package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	openrouter "github.com/hra42/openrouter-go"
)

// CommandResult is what command-mode returns: a runnable command plus a one-line
// rationale shown to the user before the y/n prompt.
type CommandResult struct {
	Command     string `json:"command"`
	Explanation string `json:"explanation"`
}

// ChatResult is what chat-mode returns: a free-form answer and, when the
// question implies an action, a concrete shell command the user can opt into.
type ChatResult struct {
	Answer           string `json:"answer"`
	SuggestedCommand string `json:"suggested_command,omitempty"`
}

var commandSchema = &openrouter.JSONSchema{
	Name:   "shell_command",
	Strict: true,
	Schema: map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"command", "explanation"},
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The raw shell command on a single line. No markdown, no code fences.",
			},
			"explanation": map[string]any{
				"type":        "string",
				"description": "One short sentence explaining what the command does. Plain text.",
			},
		},
	},
}

var chatSchema = &openrouter.JSONSchema{
	Name:   "chat_answer",
	Strict: true,
	Schema: map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"answer", "suggested_command"},
		"properties": map[string]any{
			"answer": map[string]any{
				"type":        "string",
				"description": "Concise plain-text answer to the user's question. No markdown headings.",
			},
			"suggested_command": map[string]any{
				"type":        "string",
				"description": "If the answer naturally calls for running a command, the raw single-line command. Empty string when no command applies.",
			},
		},
	},
}

// Options bundles per-call settings that aren't part of the user prompt.
type Options struct {
	APIKey        string
	Model         string
	StdinContext  string
	RedactSecrets bool
}

// Generate calls OpenRouter for command mode and returns a structured command
// + explanation.
func Generate(ctx context.Context, userPrompt string, opts Options) (CommandResult, error) {
	var out CommandResult
	if err := complete(ctx, userPrompt, opts, modeCommand, commandSchema, &out); err != nil {
		return CommandResult{}, err
	}
	out.Command = strings.TrimSpace(out.Command)
	out.Explanation = strings.TrimSpace(out.Explanation)
	return out, nil
}

// Chat calls OpenRouter for chat mode and returns a structured answer plus an
// optional suggested command the caller can offer to execute.
func Chat(ctx context.Context, userPrompt string, opts Options) (ChatResult, error) {
	var out ChatResult
	if err := complete(ctx, userPrompt, opts, modeChat, chatSchema, &out); err != nil {
		return ChatResult{}, err
	}
	out.Answer = strings.TrimSpace(out.Answer)
	out.SuggestedCommand = strings.TrimSpace(out.SuggestedCommand)
	return out, nil
}

func complete(ctx context.Context, userPrompt string, opts Options, mode promptMode, schema *openrouter.JSONSchema, out any) error {
	prompt := userPrompt
	if work := gatherWorkContext(ctx, opts.RedactSecrets); work != "" {
		prompt = prompt + "\n\n" + work
	}
	if opts.StdinContext != "" {
		prompt = prompt + "\n\n--- stdin ---\n" + opts.StdinContext
	}

	client := openrouter.NewClient(openrouter.WithAPIKey(opts.APIKey))
	messages := []openrouter.Message{
		openrouter.CreateSystemMessage(buildSystemPrompt(mode)),
		openrouter.CreateUserMessage(prompt),
	}

	resp, err := client.ChatComplete(ctx, messages,
		openrouter.WithModel(opts.Model),
		openrouter.WithResponseFormat(openrouter.ResponseFormat{
			Type:       "json_schema",
			JSONSchema: schema,
		}),
	)
	if err != nil {
		return err
	}
	if len(resp.Choices) == 0 {
		return errors.New("no choices in response")
	}
	raw, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return errors.New("unexpected response content type")
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return errors.New("model returned invalid JSON: " + err.Error())
	}
	return nil
}
