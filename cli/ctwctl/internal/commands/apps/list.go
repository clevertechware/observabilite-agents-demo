package apps

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/urfave/cli/v3"
)

func listApps(ctx context.Context, cmd *cli.Command, apps *app.Apps) error {
	statuses, err := apps.List(ctx)
	if err != nil {
		return err
	}

	if cmdutil.Settings(ctx).Output == cmdutil.OutputJSON {
		return cmdutil.WriteJSON(cmdutil.Writer(cmd), statuses)
	}

	tw := tabwriter.NewWriter(cmdutil.Writer(cmd), 0, 0, 2, ' ', 0)
	if _, err = fmt.Fprintln(tw, "NAME\tREADY\tAVAILABLE"); err != nil {
		return err
	}

	for _, status := range statuses {
		available := "no"
		if status.Available {
			available = "yes"
		}
		if _, err = fmt.Fprintf(tw, "%s\t%d/%d\t%s\n", status.Name, status.Ready, status.Replicas, available); err != nil {
			return err
		}
	}

	return tw.Flush()
}
