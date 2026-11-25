package commands

import (
	"context"
	"io"
	"os"

	appscmd "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/apps"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	o2cmd "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/o2"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/shop"
	"github.com/urfave/cli/v3"
)

const (
	outputText = cmdutil.OutputText
	outputJSON = cmdutil.OutputJSON
)

type dependencies struct {
	apps           appscmd.Dependencies
	o2             o2cmd.Dependencies
	newOrderClient func(settings) order.Client
	environment    config.Environment
}

func (d dependencies) withDefaults() dependencies {
	if d.newOrderClient == nil {
		d.newOrderClient = func(s settings) order.Client { return shop.NewOrderClient(s.Shop.URL) }
	}
	return d
}

func settingsFrom(ctx context.Context) settings {
	return cmdutil.Settings(ctx)
}

func resolvedConfigFrom(ctx context.Context) *config.Resolved {
	return cmdutil.Resolved(ctx)
}

func NewRootCommand() *cli.Command {
	return newRootCommand(dependencies{environment: config.ProcessEnvironment()})
}

func newRootCommand(deps dependencies) *cli.Command {
	deps = deps.withDefaults()

	return &cli.Command{
		Name:   "ctwctl",
		Usage:  "applications, OpenObserve logs and orders of the observability demo",
		Writer: os.Stdout,
		Flags:  cmdutil.GlobalFlags(),
		Before: cmdutil.LoadConfig(deps.environment),
		Commands: []*cli.Command{
			configCommand(deps.environment),
			appscmd.NewCommand(deps.apps),
			ordersCommand(deps),
			o2cmd.NewCommand(deps.o2),
		},
	}
}

func writer(cmd *cli.Command) io.Writer {
	return cmdutil.Writer(cmd)
}
