package o2

import (
	"context"
	"errors"
	"fmt"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/urfave/cli/v3"
)

func runQuery(ctx context.Context, cmd *cli.Command, deps Dependencies) error {
	if cmd.Args().Len() != 1 {
		return errors.New("une requête SQL est attendue : ctwctl o2 query \"<SQL>\"")
	}

	s := cmdutil.Settings(ctx)
	searcher, err := searcherFor(s, deps)
	if err != nil {
		return err
	}

	since, limit, err := parseSearchOptions(cmd)
	if err != nil {
		return err
	}

	end := deps.Now()
	result, err := searcher.Search(ctx, o2.SearchRequest{
		SQL:   cmd.Args().First(),
		Start: end.Add(-since),
		End:   end,
		Size:  limit,
	})
	if err != nil {
		return fmt.Errorf("exécution de la requête : %w", err)
	}

	if s.Output == cmdutil.OutputJSON {
		return logPrinter{w: cmdutil.Writer(cmd), isJSON: true}.printAll(result.Hits)
	}

	return writeTable(cmdutil.Writer(cmd), result.Hits)
}
