// Package config provides the commands that read and write the ctwctl configuration.
package config

import (
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

// NewCommand creates the configuration command tree. config set writes to the configuration file of environment and
// config view shows the settings resolved by loadSettings.
func NewCommand(environment config.Environment, loadSettings func(*cli.Command) (config.Settings, error)) *cli.Command {
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
				Action: viewConfig(loadSettings),
			},
		},
	}
}
