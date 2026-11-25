package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

func setConfig(environment config.Environment) cli.ActionFunc {
	return func(_ context.Context, cmd *cli.Command) error {
		if cmd.Args().Len() != 2 {
			return errors.New("a key and a value are expected: ctwctl config set <key> <value>")
		}

		key, value := cmd.Args().Get(0), cmd.Args().Get(1)
		if !config.IsKey(key) {
			return fmt.Errorf("unknown key %q: %s", key, strings.Join(config.Keys(), ", "))
		}

		path, err := config.SaveValue(environment, key, value)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(cmdutil.Writer(cmd), "%s written to %s\n", key, path)
		return err
	}
}
