// Package cmdutil contains helpers shared by ctwctl command packages.
package cmdutil

import (
	"encoding/json"
	"io"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

var flagKeys = map[string]string{
	"output":    "output",
	"namespace": "kube.namespace",
	"context":   "kube.context",
}

// GlobalFlags returns the flags shared by all ctwctl commands.
func GlobalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "output format: text or json"},
		&cli.StringFlag{Name: "context", Usage: "kubectl context (default: the current context)"},
		&cli.StringFlag{Name: "namespace", Usage: "Kubernetes namespace (default: demo)"},
	}
}

// SettingsLoader resolves the settings of the invoked command.
type SettingsLoader func(cmd *cli.Command) (config.Settings, error)

// NewSettingsLoader returns a SettingsLoader that resolves the settings from environment and the flags of the command.
func NewSettingsLoader(environment config.Environment) SettingsLoader {
	return func(cmd *cli.Command) (config.Settings, error) {
		return config.Load(environment, flagsSet(cmd))
	}
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
