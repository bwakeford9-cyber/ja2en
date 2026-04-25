// Package initcmd implements the `ja2en init` subcommand.
package initcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/GigiTiti-Kai/ja2en/internal/config"
)

// Run creates ~/.config/ja2en/config.toml from the embedded template.
// If the file already exists, returns an error unless force is true.
func Run(force bool) error {
	cfgPath, err := config.Path()
	if err != nil {
		return err
	}
	cfgDir := filepath.Dir(cfgPath)

	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", cfgDir, err)
	}

	if !force {
		if _, err := os.Stat(cfgPath); err == nil {
			return fmt.Errorf("config already exists at %s. use --force to overwrite", cfgPath)
		}
	}

	if err := os.WriteFile(cfgPath, []byte(ConfigTemplate), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", cfgPath, err)
	}

	fmt.Printf("Created %s\n", cfgPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Make sure the API key referenced by api_key_env in the")
	fmt.Println("     config (default: GEMINI_API_KEY) is set in your shell:")
	fmt.Println("       echo $GEMINI_API_KEY")
	fmt.Println("     (get a Google AI Studio key: https://aistudio.google.com/apikey)")
	fmt.Println("  2. Try it:")
	fmt.Println("       ja2en \"明日出社する\"")
	return nil
}
