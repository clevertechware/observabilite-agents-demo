package config

import (
	"path/filepath"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdtest"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
)

func run(t *testing.T, environment config.Environment, args ...string) (string, error) {
	t.Helper()

	return cmdtest.Run(t.Context(), environment, NewCommand(environment), args...)
}

func configFilePath(environment config.Environment) string {
	return filepath.Join(environment.Variables["XDG_CONFIG_HOME"], "ctwctl", "config.toml")
}
