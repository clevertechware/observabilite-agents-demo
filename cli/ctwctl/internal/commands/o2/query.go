package o2

import (
	"context"
	"errors"
	"fmt"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/urfave/cli/v3"
)

func runQuery(ctx context.Context, cmd *cli.Command, deps Dependencies) error {
	if cmd.Args().Len() != 1 {
		return errors.New("an SQL query is expected: ctwctl o2 query \"<SQL>\"")
	}

	streamType, err := parseStreamType(cmd.String("type"))
	if err != nil {
		return err
	}

	s, err := deps.LoadSettings(cmd)
	if err != nil {
		return err
	}

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
		SQL:        cmd.Args().First(),
		Start:      end.Add(-since),
		End:        end,
		Size:       limit,
		StreamType: streamType,
	})
	if err != nil {
		return fmt.Errorf("run query: %w", err)
	}

	if s.Output == config.OutputJSON {
		return logPrinter{w: cmd.Root().Writer, isJSON: true}.printAll(result.Hits)
	}

	return writeTable(cmd.Root().Writer, result.Hits)
}
