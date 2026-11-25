// Package cmdutil contains helpers shared by ctwctl command packages.
package cmdutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

const (
	// OutputText selects human-readable command output.
	OutputText = "text"
	// OutputJSON selects JSON command output.
	OutputJSON = "json"
)

var flagKeys = map[string]string{
	"output":    "output",
	"namespace": "kube.namespace",
	"context":   "kube.context",
}

type settingsKey struct{}

// GlobalFlags returns the flags shared by all ctwctl commands.
func GlobalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "output format: text or json"},
		&cli.StringFlag{Name: "context", Usage: "kubectl context (default: the current context)"},
		&cli.StringFlag{Name: "namespace", Usage: "Kubernetes namespace (default: demo)"},
	}
}

// LoadConfig returns the hook that loads and validates the CLI configuration from environment before a command runs.
func LoadConfig(environment config.Environment) cli.BeforeFunc {
	load := NewSettingsLoader(environment)

	return func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
		settings, err := load(cmd)
		if err != nil {
			return ctx, err
		}

		return context.WithValue(ctx, settingsKey{}, settings), nil
	}
}

// SettingsLoader resolves the settings of the invoked command.
type SettingsLoader func(cmd *cli.Command) (config.Settings, error)

// NewSettingsLoader returns a SettingsLoader that resolves the settings from environment and the flags of the command.
func NewSettingsLoader(environment config.Environment) SettingsLoader {
	return func(cmd *cli.Command) (config.Settings, error) {
		settings, err := config.Load(environment, flagsSet(cmd))
		if err != nil {
			return config.Settings{}, err
		}

		if settings.Output != OutputText && settings.Output != OutputJSON {
			return config.Settings{}, fmt.Errorf("unknown output format %q: text or json expected", settings.Output)
		}

		return settings, nil
	}
}

// Settings returns the settings resolved by Before, or their zero value if absent.
func Settings(ctx context.Context) config.Settings {
	settings, _ := ctx.Value(settingsKey{}).(config.Settings)
	return settings
}

// Writer returns the root command's output writer.
func Writer(cmd *cli.Command) io.Writer {
	return cmd.Root().Writer
}

// WriteJSON encodes v as JSON without escaping HTML characters.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func flagsSet(cmd *cli.Command) map[string]any {
	values := map[string]any{}
	for flag, key := range flagKeys {
		if cmd.IsSet(flag) {
			values[key] = cmd.String(flag)
		}
	}

	return values
}
