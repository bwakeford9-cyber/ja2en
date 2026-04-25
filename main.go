// Command ja2en translates Japanese text to English via OpenRouter.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/GigiTiti-Kai/ja2en/internal/clipboard"
	"github.com/GigiTiti-Kai/ja2en/internal/config"
	"github.com/GigiTiti-Kai/ja2en/internal/initcmd"
	"github.com/GigiTiti-Kai/ja2en/internal/input"
	"github.com/GigiTiti-Kai/ja2en/internal/translator"
)

// translatorClient is the minimal contract main.go needs. Both
// translator.Client (OpenAI-compatible) and translator.DeepLClient satisfy it.
//
// reasoningEffort is consumed by OpenAI-compatible providers (GPT-5.x:
// none/low/medium/high/xhigh; Gemini 2.5: none disables thinking). DeepL
// ignores it.
type translatorClient interface {
	Translate(ctx context.Context, model, systemPrompt, userText, reasoningEffort string) (string, error)
}

// version is overridden at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var (
		profileFlag     string
		modelFlag       string
		promptFileFlag  string
		clipFlag        bool
		pasteFlag       bool
		editorFlag      bool
		interactiveFlag bool
	)

	root := &cobra.Command{
		Use:           "ja2en [text]",
		Short:         "Translate Japanese to English via OpenAI / Gemini / DeepL",
		Long:          "ja2en is a tiny CLI that translates Japanese to English.\nFirst run: ja2en init",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTranslate(cmd.Context(), args, runOpts{
				Profile:     profileFlag,
				Model:       modelFlag,
				PromptFile:  promptFileFlag,
				Clip:        clipFlag,
				Paste:       pasteFlag,
				Editor:      editorFlag,
				Interactive: interactiveFlag,
			})
		},
	}

	root.Flags().StringVarP(&profileFlag, "profile", "p", "", "config profile name")
	root.Flags().StringVar(&modelFlag, "model", "", "model slug override (e.g. google/gemma-3-27b-it:free)")
	root.Flags().StringVar(&promptFileFlag, "prompt-file", "", "ad-hoc prompt file path")
	root.Flags().BoolVar(&clipFlag, "clip", false, "read input from clipboard")
	root.Flags().BoolVar(&pasteFlag, "paste", false, "write translation to clipboard")
	root.Flags().BoolVarP(&editorFlag, "editor", "e", false, "compose input in $EDITOR (vim/nano)")
	root.Flags().BoolVarP(&interactiveFlag, "interactive", "i", false, "read multi-line stdin until Ctrl-D")

	root.AddCommand(newInitCmd())
	return root
}

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create ~/.config/ja2en/config.toml from the default template",
		RunE: func(_ *cobra.Command, _ []string) error {
			return initcmd.Run(force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config")
	return cmd
}

type runOpts struct {
	Profile     string
	Model       string
	PromptFile  string
	Clip        bool
	Paste       bool
	Editor      bool
	Interactive bool
}

func runTranslate(ctx context.Context, args []string, opts runOpts) error {
	cfgPath, err := config.Path()
	if err != nil {
		return err
	}
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return fmt.Errorf("config not found at %s. Run: ja2en init", cfgPath)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	resolved, err := cfg.Resolve(opts.Profile, opts.Model, opts.PromptFile)
	if err != nil {
		return err
	}

	text, err := input.Resolve(input.Source{
		Args:           args,
		UseClip:        opts.Clip,
		UseEditor:      opts.Editor,
		UseInteractive: opts.Interactive,
	})
	if err != nil {
		return err
	}

	timeout := time.Duration(resolved.TimeoutSeconds) * time.Second
	var client translatorClient
	switch resolved.Provider {
	case "deepl":
		client = translator.NewDeepLClient(resolved.APIBase, resolved.APIKey, timeout)
	default:
		client = translator.NewClient(resolved.APIBase, resolved.APIKey, timeout)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := client.Translate(ctx, resolved.Model, resolved.Prompt, text, resolved.ReasoningEffort)
	if err != nil {
		return err
	}

	fmt.Println(out)

	if opts.Paste {
		if err := clipboard.Write(out); err != nil {
			fmt.Fprintf(os.Stderr, "warning: clipboard write failed: %v\n", err)
		}
	}
	return nil
}
