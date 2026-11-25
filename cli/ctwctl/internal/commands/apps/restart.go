package apps

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/app"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/urfave/cli/v3"
)

func restartApp(ctx context.Context, cmd *cli.Command, apps *app.Apps) error {
	if cmd.Args().Len() != 1 {
		return errors.New("a Deployment name is expected: ctwctl apps restart <name>")
	}

	name := cmd.Args().First()
	out, err := apps.Restart(ctx, name)
	if errors.Is(err, app.ErrInvalidName) {
		return fmt.Errorf("invalid Deployment name: %q", name)
	}
	if err != nil {
		return err
	}

	if cmdutil.Settings(ctx).Output == cmdutil.OutputJSON {
		return cmdutil.WriteJSON(cmdutil.Writer(cmd), map[string]any{"name": name, "restarted": true})
	}

	_, err = fmt.Fprintln(cmdutil.Writer(cmd), strings.TrimSpace(out))
	return err
}
