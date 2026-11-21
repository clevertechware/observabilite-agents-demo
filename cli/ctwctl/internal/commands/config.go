package commands

import (
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

var flagKeys = map[string]string{
	"output":    "output",
	"namespace": "kube.namespace",
	"context":   "kube.context",
}

func flagsSet(cmd *cli.Command) map[string]any {
	values := map[string]any{}
	for flag, key := range flagKeys {
		if cmd.IsSet(flag) {
			values[key] = cmd.String(flag)
		}
	}
	return values
}

type settings = config.Settings
