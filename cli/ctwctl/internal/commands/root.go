package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/shop"
	"github.com/urfave/cli/v3"
)

const (
	outputText = "text"
	outputJSON = "json"
)

type dependencies struct {
	kubectl        kubectlRunner
	newOrderClient func(settings) orderClient
}

func (d dependencies) withDefaults() dependencies {
	if d.kubectl == nil {
		d.kubectl = execRunner{}
	}
	if d.newOrderClient == nil {
		d.newOrderClient = func(s settings) orderClient { return shop.NewOrderClient(s.Shop.URL) }
	}
	return d
}

type settingsKey struct{}

func settingsFrom(ctx context.Context) settings {
	s, _ := ctx.Value(settingsKey{}).(settings)
	return s
}

func NewRootCommand() *cli.Command {
	return newRootCommand(dependencies{})
}

func newRootCommand(deps dependencies) *cli.Command {
	deps = deps.withDefaults()
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
			appsCommand(deps),
			ordersCommand(deps),
		},
	}
}

func writer(cmd *cli.Command) io.Writer {
	return cmd.Root().Writer
}
