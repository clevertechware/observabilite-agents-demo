// Package o2 provides OpenObserve commands for ctwctl.
package o2

import (
	"context"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/urfave/cli/v3"
)

const defaultLimit = 100

// LogSearcher provides the OpenObserve search and stream operations used by the commands.
type LogSearcher interface {
	// Search executes a log search request.
	Search(ctx context.Context, req o2.SearchRequest) (o2.SearchResult, error)
	// Streams lists streams of the requested type.
	Streams(ctx context.Context, streamType string) ([]o2.Stream, error)
}

// Dependencies contains the configuration, the services and the clock used by the OpenObserve commands.
type Dependencies struct {
	// LoadSettings resolves the settings of the invoked command.
	LoadSettings func(*cli.Command) (config.Settings, error)
	// NewLogSearcher creates a search client from the resolved configuration.
	NewLogSearcher func(config.Settings) LogSearcher
	// PollInterval is the delay between successive log-follow searches.
	PollInterval time.Duration
	// Now returns the current time used to build search windows.
	Now func() time.Time
}

// NewCommand creates the OpenObserve command tree. Every field of deps is required.
func NewCommand(deps Dependencies) *cli.Command {
	sinceFlag := &cli.StringFlag{Name: "since", Usage: "time window, for example 15m or 1h", Value: "1h"}
	limitFlag := &cli.IntFlag{Name: "limit", Usage: "maximum number of results", Value: defaultLimit}

	return &cli.Command{
		Name:  "o2",
		Usage: "OpenObserve streams, logs and SQL queries",
		Commands: []*cli.Command{
			{
				Name:  "streams",
				Usage: "list streams and their fields",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "type", Usage: "logs, metrics or traces (default: logs and traces)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return listStreams(ctx, cmd, deps)
				},
			},
			{
				Name:  "logs",
				Usage: "read the logs of the default stream",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "service", Usage: "filter on service_name"},
					&cli.StringFlag{Name: "grep", Usage: "filter on the message text"},
					&cli.StringFlag{Name: "level", Usage: "minimum level: trace, debug, info, warn, error or fatal"},
					sinceFlag,
					limitFlag,
					&cli.BoolFlag{Name: "follow", Aliases: []string{"f"}, Usage: "poll the logs at a fixed interval"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return showLogs(ctx, cmd, deps)
				},
			},
			{
				Name:      "query",
				Usage:     "run an SQL query as is",
				ArgsUsage: "<SQL>",
				Flags: append([]cli.Flag{
					&cli.StringFlag{Name: "type", Usage: "stream type: logs, traces or metrics", Value: streamTypeLogs},
					limitFlag,
				}, windowFlags()...),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runQuery(ctx, cmd, deps)
				},
			},
			{
				Name:  "link",
				Usage: "print the OpenObserve interface URL of an SQL query or a trace",
				Flags: linkFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return printLink(ctx, cmd, deps)
				},
			},
		},
	}
}
