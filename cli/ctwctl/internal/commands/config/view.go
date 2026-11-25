package config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

func viewConfig(loadSettings func(*cli.Command) (config.Settings, error)) cli.ActionFunc {
	return func(_ context.Context, cmd *cli.Command) error {
		settings, err := loadSettings(cmd)
		if err != nil {
			return err
		}

		masked := settings.Masked()
		if settings.Output == config.OutputJSON {
			return json.NewEncoder(cmd.Root().Writer).Encode(masked)
		}

		byKey, err := masked.ByKey()
		if err != nil {
			return err
		}

		for _, key := range config.Keys() {
			if _, err = fmt.Fprintf(cmd.Root().Writer, "%s = %v\n", key, byKey[key]); err != nil {
				return err
			}
		}

		return nil
	}
}
