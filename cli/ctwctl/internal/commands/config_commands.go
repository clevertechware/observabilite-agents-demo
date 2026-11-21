package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

func configCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "lit et écrit ~/.config/ctwctl/config.toml",
		Commands: []*cli.Command{
			{
				Name:      "set",
				Usage:     "écrit une valeur dans le fichier de configuration",
				ArgsUsage: "<clé> <valeur>",
				Action:    setConfig,
			},
			{
				Name:   "view",
				Usage:  "affiche la configuration résolue, jeton masqué",
				Action: viewConfig,
			},
		},
	}
}

func setConfig(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 2 {
		return fmt.Errorf("une clé et une valeur sont attendues : ctwctl config set <clé> <valeur>")
	}
	key, value := cmd.Args().Get(0), cmd.Args().Get(1)
	if !config.IsKey(key) {
		return fmt.Errorf("clé inconnue %q : %s", key, strings.Join(config.Keys(), ", "))
	}
	path, err := config.SaveValue(key, value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer(cmd), "%s écrit dans %s\n", key, path)
	return err
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
		if _, err := fmt.Fprintf(writer(cmd), "%s = %v\n", key, view.Values[key]); err != nil {
			return err
		}
	}
	return nil
}
