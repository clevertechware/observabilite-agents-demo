package commands

import (
	"context"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/urfave/cli/v3"
)

const (
	defaultLimit = 100
)

type logSearcher interface {
	Search(ctx context.Context, req o2.SearchRequest) (o2.SearchResult, error)
	Streams(ctx context.Context, streamType string) ([]o2.Stream, error)
}

func o2Command(deps dependencies) *cli.Command {
	sinceFlag := &cli.StringFlag{Name: "since", Usage: "fenêtre de temps, par exemple 15m ou 1h", Value: "1h"}
	limitFlag := &cli.IntFlag{Name: "limit", Usage: "nombre maximal de résultats", Value: defaultLimit}
	return &cli.Command{
		Name:  "o2",
		Usage: "streams, logs et requêtes SQL d'OpenObserve",
		Commands: []*cli.Command{
			{
				Name:  "streams",
				Usage: "liste les streams et leurs champs",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "type", Usage: "logs, metrics ou traces (défaut : logs et traces)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return listStreams(ctx, cmd, deps)
				},
			},
			{
				Name:  "logs",
				Usage: "lit les logs du stream default",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "service", Usage: "filtre sur service_name"},
					&cli.StringFlag{Name: "grep", Usage: "filtre sur le texte du message"},
					&cli.StringFlag{Name: "level", Usage: "niveau minimal : trace, debug, info, warn, error ou fatal"},
					sinceFlag,
					limitFlag,
					&cli.BoolFlag{Name: "follow", Aliases: []string{"f"}, Usage: "relit les logs à intervalle fixe"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return showLogs(ctx, cmd, deps)
				},
			},
			{
				Name:      "query",
				Usage:     "exécute une requête SQL telle quelle",
				ArgsUsage: "<SQL>",
				Flags:     []cli.Flag{sinceFlag, limitFlag},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runQuery(ctx, cmd, deps)
				},
			},
		},
	}
}
