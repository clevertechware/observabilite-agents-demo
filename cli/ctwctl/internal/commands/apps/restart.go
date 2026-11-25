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

func restartApp(ctx context.Context, cmd *cli.Command, deps Dependencies) error {
	if cmd.Args().Len() != 1 {
		return errors.New("a Deployment name is expected: ctwctl apps restart <name>")
	}

	s, err := deps.LoadSettings(cmd)
	if err != nil {
		return err
	}

	name := cmd.Args().First()
	out, err := app.NewApps(deps.NewAppClient(s)).Restart(ctx, name)
	if errors.Is(err, app.ErrInvalidName) {
		return fmt.Errorf("invalid Deployment name: %q", name)
	}
	if err != nil {
		return err
	}

	if s.Output == cmdutil.OutputJSON {
		return cmdutil.WriteJSON(cmdutil.Writer(cmd), map[string]any{"name": name, "restarted": true})
	}

	_, err = fmt.Fprintln(cmdutil.Writer(cmd), strings.TrimSpace(out))
	return err
}
