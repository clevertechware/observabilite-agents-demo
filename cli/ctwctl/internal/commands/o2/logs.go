package o2

import (
	"context"
	"errors"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/urfave/cli/v3"
)

func showLogs(ctx context.Context, cmd *cli.Command, deps Dependencies) error {
	s := cmdutil.Settings(ctx)
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
	printer := logPrinter{w: cmdutil.Writer(cmd), json: s.Output == cmdutil.OutputJSON}
	if cmd.Bool("follow") {
		follower := newFollower(searcher, filter, limit, deps.PollInterval, deps.Now, printer.print)
		err := follower.run(ctx, since)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	end := deps.Now()
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
