package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

func configCommand(environment config.Environment) *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "read and write ~/.config/ctwctl/config.toml",
		Commands: []*cli.Command{
			{
				Name:      "set",
				Usage:     "write a value to the configuration file",
				ArgsUsage: "<key> <value>",
				Action:    setConfig(environment),
			},
			{
				Name:   "view",
				Usage:  "show the resolved configuration, token masked",
				Action: viewConfig,
			},
		},
	}
}

func setConfig(environment config.Environment) cli.ActionFunc {
	return func(_ context.Context, cmd *cli.Command) error {
		if cmd.Args().Len() != 2 {
			return fmt.Errorf("a key and a value are expected: ctwctl config set <key> <value>")
		}

		key, value := cmd.Args().Get(0), cmd.Args().Get(1)
		if !config.IsKey(key) {
			return fmt.Errorf("unknown key %q: %s", key, strings.Join(config.Keys(), ", "))
		}

		path, err := config.SaveValue(environment, key, value)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(writer(cmd), "%s written to %s\n", key, path)
		return err
	}
}

func viewConfig(ctx context.Context, cmd *cli.Command) error {
	view, err := resolvedConfigFrom(ctx).MaskedView()
	if err != nil {
		return err
	}

	if settingsFrom(ctx).Output == outputJSON {
		return writeJSON(writer(cmd), view.Raw)
	}

	for _, key := range config.Keys() {
		if _, err = fmt.Fprintf(writer(cmd), "%s = %v\n", key, view.Values[key]); err != nil {
			return err
		}
	}

	return nil
}
