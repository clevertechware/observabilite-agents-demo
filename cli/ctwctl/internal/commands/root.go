package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/shop"
	"github.com/urfave/cli/v3"
)

const (
	outputText = "text"
	outputJSON = "json"
)

type dependencies struct {
	kubectl        kubectlRunner
	newLogSearcher func(settings) logSearcher
	newOrderClient func(settings) orderClient
	pollInterval   time.Duration
	now            func() time.Time
}

func (d dependencies) withDefaults() dependencies {
	if d.kubectl == nil {
		d.kubectl = execRunner{}
	}
	if d.newLogSearcher == nil {
		d.newLogSearcher = func(s settings) logSearcher {
			return o2.NewClient(s.O2.URL, s.O2.Org, s.O2.User, s.O2.Token)
		}
	}
	if d.newOrderClient == nil {
		d.newOrderClient = func(s settings) orderClient { return shop.NewOrderClient(s.Shop.URL) }
	}
	if d.pollInterval == 0 {
		d.pollInterval = 2 * time.Second
	}
	if d.now == nil {
		d.now = time.Now
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
			o2Command(deps),
		},
	}
}

func writer(cmd *cli.Command) io.Writer {
	return cmd.Root().Writer
}
