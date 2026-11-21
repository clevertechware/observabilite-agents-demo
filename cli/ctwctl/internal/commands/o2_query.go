package commands

import (
	"context"
	"errors"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/urfave/cli/v3"
)

func runQuery(ctx context.Context, cmd *cli.Command, deps dependencies) error {
	if cmd.Args().Len() != 1 {
		return errors.New("une requête SQL est attendue : ctwctl o2 query \"<SQL>\"")
	}
	s := settingsFrom(ctx)
	searcher, err := searcherFor(s, deps)
	if err != nil {
		return err
	}
	since, limit, err := parseSearchOptions(cmd)
	if err != nil {
		return err
	}
	end := deps.now()
	result, err := searcher.Search(ctx, o2.SearchRequest{
		SQL:   cmd.Args().First(),
		Start: end.Add(-since),
		End:   end,
		Size:  limit,
	})
	if err != nil {
		return err
	}
	if s.Output == outputJSON {
		return logPrinter{w: writer(cmd), json: true}.printAll(result.Hits)
	}
	return writeTable(writer(cmd), result.Hits)
}
