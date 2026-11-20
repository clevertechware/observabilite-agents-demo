package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v3"
)

const (
	outputText = "text"
	outputJSON = "json"
)

type dependencies struct{}

type settingsKey struct{}

func settingsFrom(ctx context.Context) settings {
	s, _ := ctx.Value(settingsKey{}).(settings)
	return s
}

func NewRootCommand() *cli.Command {
	return newRootCommand(dependencies{})
}

func newRootCommand(_ dependencies) *cli.Command {
	return &cli.Command{
		Name:   "ctwctl",
		Usage:  "applications, logs OpenObserve et commandes de la démo d'observabilité",
		Writer: os.Stdout,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "format de sortie : text ou json"},
			&cli.StringFlag{Name: "context", Usage: "contexte kubectl (défaut : le contexte courant)"},
			&cli.StringFlag{Name: "namespace", Usage: "namespace Kubernetes (défaut : demo)"},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			k, err := loadConfig(cmd)
			if err != nil {
				return ctx, err
			}
			s, err := decodeSettings(k)
			if err != nil {
				return ctx, err
			}
			if s.Output != outputText && s.Output != outputJSON {
				return ctx, fmt.Errorf("format de sortie %q inconnu : text ou json attendu", s.Output)
			}
			return context.WithValue(ctx, settingsKey{}, s), nil
		},
		Commands: []*cli.Command{
			configCommand(),
		},
	}
}

func writer(cmd *cli.Command) io.Writer {
	return cmd.Root().Writer
}
