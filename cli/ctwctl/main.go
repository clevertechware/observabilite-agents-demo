package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
	appscmd "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/apps"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	configcmd "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/config"
	o2cmd "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/o2"
	orderscmd "github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/orders"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/kube"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/order"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/shop"
	"github.com/urfave/cli/v3"
)

const logPollInterval = 2 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := newRootCommand(config.ProcessEnvironment()).Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "ctwctl: %v\n", err)
		stop()
		os.Exit(1)
	}
}

// newRootCommand assembles the command groups with the clients they use, all configured from environment.
func newRootCommand(environment config.Environment) *cli.Command {
	loadSettings := cmdutil.NewSettingsLoader(environment)

	return &cli.Command{
		Name:   "ctwctl",
		Usage:  "applications, OpenObserve logs and orders of the observability demo",
		Writer: os.Stdout,
		Flags:  cmdutil.GlobalFlags(),
		Commands: []*cli.Command{
			configcmd.NewCommand(environment, loadSettings),
			appscmd.NewCommand(appscmd.Dependencies{
				LoadSettings: loadSettings,
				NewAppClient: newAppClient,
			}),
			orderscmd.NewCommand(orderscmd.Dependencies{
				LoadSettings:   loadSettings,
				NewOrderClient: newOrderClient,
			}),
			o2cmd.NewCommand(o2cmd.Dependencies{
				LoadSettings:   loadSettings,
				NewLogSearcher: newLogSearcher,
				PollInterval:   logPollInterval,
				Now:            time.Now,
			}),
		},
	}
}

func newAppClient(s config.Settings) app.Client {
	return kube.NewClient(s.Kube.Namespace, s.Kube.Context)
}

func newOrderClient(s config.Settings) order.Client {
	return shop.NewOrderClient(s.Shop.URL)
}

func newLogSearcher(s config.Settings) o2cmd.LogSearcher {
	return o2.NewClient(s.O2.URL, s.O2.Org, s.O2.User, s.O2.Token)
}
