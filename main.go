package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hra42/ai/internal/config"
	aiexec "github.com/hra42/ai/internal/exec"
	"github.com/hra42/ai/internal/runner"
	"github.com/hra42/ai/internal/ui"
)

const defaultModel = "anthropic/claude-sonnet-4-5"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		modelFlag string
		chatFlag  bool
		printOnly bool
		yes       bool
	)

	cmd := &cobra.Command{
		Use:     "ai [request...]",
		Short:   "Turn natural language into shell commands via OpenRouter",
		Version: fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if printOnly && yes {
				return fmt.Errorf("--print and --yes are mutually exclusive")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			apiKey := os.Getenv("OPENROUTER_API_KEY")
			if apiKey == "" && cfg.OpRef != "" {
				key, err := config.ResolveOpRef(ctx, cfg.OpRef)
				if err != nil {
					return err
				}
				apiKey = key
			}
			if apiKey == "" {
				apiKey = cfg.APIKey
			}

			model := modelFlag
			if model == "" {
				model = cfg.Model
			}

			needKey := apiKey == ""
			needModel := model == "" && modelFlag == ""
			if (needKey || needModel) && config.IsTTY(os.Stdin) {
				res, err := config.RunSetup(ctx, cfg, needKey, needModel)
				if err != nil {
					return err
				}
				cfg = res.Cfg
				if err := config.Save(cfg); err != nil {
					return fmt.Errorf("save config: %w", err)
				}
				if needKey {
					apiKey = res.APIKey
				}
				if needModel {
					model = res.Model
				}
			}

			if apiKey == "" {
				return fmt.Errorf("no API key — set OPENROUTER_API_KEY or run interactively")
			}
			if model == "" {
				model = defaultModel
			}

			userPrompt := strings.Join(args, " ")

			stdinContext, err := readPipedStdin()
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}

			isTTY := config.IsTTY(os.Stdin) && config.IsTTY(os.Stderr)

			opts := runner.Options{
				APIKey:        apiKey,
				Model:         model,
				StdinContext:  stdinContext,
				RedactSecrets: cfg.RedactSecretsEnabled(),
			}
			if chatFlag {
				return handleChat(ctx, opts, userPrompt, isTTY, printOnly, yes)
			}
			return handleCommand(ctx, opts, userPrompt, isTTY, printOnly, yes)
		},
	}

	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.Flags().StringVar(&modelFlag, "model", "", "OpenRouter model id (overrides config)")
	cmd.Flags().BoolVarP(&chatFlag, "chat", "c", false, "chat mode: answer the question, don't run anything")
	cmd.Flags().BoolVarP(&printOnly, "print", "p", false, "print the command and exit, don't execute")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation and execute the command")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error.Render("error: "+err.Error()))
		os.Exit(1)
	}
}

func handleCommand(ctx context.Context, opts runner.Options, userPrompt string, isTTY, printOnly, yes bool) error {
	// Non-TTY paths skip the TUI.
	if !isTTY || printOnly || yes {
		res, err := runner.Generate(ctx, userPrompt, opts)
		if err != nil {
			return err
		}
		if res.Command == "" {
			return fmt.Errorf("model returned empty command")
		}
		fmt.Fprintln(os.Stdout, res.Command)
		if printOnly {
			return nil
		}
		if !yes && !isTTY {
			return fmt.Errorf("interactive terminal required, or pass --yes / --print")
		}
		return aiexec.Run(ctx, res.Command)
	}

	outcome, res, err := ui.RunReview(ctx, func(c context.Context) (ui.Result, error) {
		out, err := runner.Generate(c, userPrompt, opts)
		if err != nil {
			return ui.Result{}, err
		}
		return ui.Result{Command: out.Command, Explanation: out.Explanation}, nil
	})
	if err != nil {
		return err
	}
	if outcome == ui.OutcomeRun {
		return aiexec.Run(ctx, res.Command)
	}
	return nil
}

func handleChat(ctx context.Context, opts runner.Options, userPrompt string, isTTY, printOnly, yes bool) error {
	if !isTTY || printOnly || yes {
		res, err := runner.Chat(ctx, userPrompt, opts)
		if err != nil {
			return err
		}
		if res.Answer != "" {
			fmt.Fprintln(os.Stdout, res.Answer)
		}
		if res.SuggestedCommand == "" {
			return nil
		}
		fmt.Fprintln(os.Stdout, res.SuggestedCommand)
		if printOnly {
			return nil
		}
		if !yes && !isTTY {
			return nil // suggested command surfaced, but no consent path available
		}
		return aiexec.Run(ctx, res.SuggestedCommand)
	}

	outcome, res, err := ui.RunReview(ctx, func(c context.Context) (ui.Result, error) {
		out, err := runner.Chat(c, userPrompt, opts)
		if err != nil {
			return ui.Result{}, err
		}
		return ui.Result{Answer: out.Answer, SuggestedCommand: out.SuggestedCommand}, nil
	})
	if err != nil {
		return err
	}
	// Pure chat answer (no suggestion) — TUI already exited; print on stdout.
	if res.Command == "" && res.SuggestedCommand == "" && res.Answer != "" {
		fmt.Fprintln(os.Stdout, res.Answer)
		return nil
	}
	if outcome == ui.OutcomeRun && res.SuggestedCommand != "" {
		return aiexec.Run(ctx, res.SuggestedCommand)
	}
	return nil
}

// readPipedStdin returns the contents of stdin if it's been piped in,
// or an empty string if stdin is a TTY.
func readPipedStdin() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
