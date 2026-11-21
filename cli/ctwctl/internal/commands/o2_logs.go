package commands

import (
	"context"
	"errors"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/urfave/cli/v3"
)

func showLogs(ctx context.Context, cmd *cli.Command, deps dependencies) error {
	s := settingsFrom(ctx)
	searcher, err := searcherFor(s, deps)
	if err != nil {
		return err
	}
	since, limit, err := parseSearchOptions(cmd)
	if err != nil {
		return err
	}
	namespace := ""
	if cmd.IsSet("namespace") {
		namespace = s.Kube.Namespace
	}
	filter, err := newLogFilter(cmd.String("service"), namespace, cmd.String("grep"), cmd.String("level"))
	if err != nil {
		return err
	}
	printer := logPrinter{w: writer(cmd), json: s.Output == outputJSON}
	if cmd.Bool("follow") {
		follower := newFollower(searcher, filter, limit, deps.pollInterval, deps.now, printer.print)
		err := follower.run(ctx, since)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	end := deps.now()
	result, err := searcher.Search(ctx, o2.SearchRequest{
		SQL:   filter.sql(orderNewestFirst),
		Start: end.Add(-since),
		End:   end,
		Size:  limit,
	})
	if err != nil {
		return err
	}
	return printer.printAll(result.Hits)
}
