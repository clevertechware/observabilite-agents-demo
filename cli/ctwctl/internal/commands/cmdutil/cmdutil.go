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

type resolvedKey struct{}

// GlobalFlags returns the flags shared by all ctwctl commands.
func GlobalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "format de sortie : text ou json"},
		&cli.StringFlag{Name: "context", Usage: "contexte kubectl (défaut : le contexte courant)"},
		&cli.StringFlag{Name: "namespace", Usage: "namespace Kubernetes (défaut : demo)"},
	}
}

// Before loads and validates the CLI configuration before a command runs.
func Before(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	resolved, err := config.Load(flagsSet(cmd))
	if err != nil {
		return ctx, err
	}

	if resolved.Settings.Output != OutputText && resolved.Settings.Output != OutputJSON {
		return ctx, fmt.Errorf("format de sortie %q inconnu : text ou json attendu", resolved.Settings.Output)
	}

	return context.WithValue(ctx, resolvedKey{}, resolved), nil
}

// Resolved returns the configuration resolved by Before, if present in ctx.
func Resolved(ctx context.Context) *config.Resolved {
	resolved, _ := ctx.Value(resolvedKey{}).(*config.Resolved)
	return resolved
}

// Settings returns the settings resolved by Before, or their zero value if absent.
func Settings(ctx context.Context) config.Settings {
	resolved := Resolved(ctx)
	if resolved == nil {
		return config.Settings{}
	}

	return resolved.Settings
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
