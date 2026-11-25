package config

import (
	"context"
	"fmt"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

func viewConfig(ctx context.Context, cmd *cli.Command) error {
	view, err := cmdutil.Resolved(ctx).MaskedView()
	if err != nil {
		return err
	}

	if cmdutil.Settings(ctx).Output == cmdutil.OutputJSON {
		return cmdutil.WriteJSON(cmdutil.Writer(cmd), view.Raw)
	}

	for _, key := range config.Keys() {
		if _, err = fmt.Fprintf(cmdutil.Writer(cmd), "%s = %v\n", key, view.Values[key]); err != nil {
			return err
		}
	}

	return nil
}
