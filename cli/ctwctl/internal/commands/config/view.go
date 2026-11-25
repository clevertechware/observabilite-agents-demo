package config

import (
	"context"
	"fmt"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

func viewConfig(loadSettings cmdutil.SettingsLoader) cli.ActionFunc {
	return func(_ context.Context, cmd *cli.Command) error {
		settings, err := loadSettings(cmd)
		if err != nil {
			return err
		}

		masked := settings.Masked()
		if settings.Output == cmdutil.OutputJSON {
			return cmdutil.WriteJSON(cmdutil.Writer(cmd), masked)
		}

		byKey, err := masked.ByKey()
		if err != nil {
			return err
		}

		for _, key := range config.Keys() {
			if _, err = fmt.Fprintf(cmdutil.Writer(cmd), "%s = %v\n", key, byKey[key]); err != nil {
				return err
			}
		}

		return nil
	}
}
