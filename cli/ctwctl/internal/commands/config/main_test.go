package config

import (
	"bytes"
	"context"
	"io"
	"maps"
	"path/filepath"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

func isolatedEnvironment(t *testing.T, variables map[string]string) config.Environment {
	t.Helper()

	all := map[string]string{"XDG_CONFIG_HOME": t.TempDir()}
	maps.Copy(all, variables)

	return config.Environment{Variables: all, UserHomeDir: func() (string, error) {
		t.Fatal("user home should not be requested when XDG_CONFIG_HOME is set")
		return "", nil
	}}
}

func runSet(t *testing.T, environment config.Environment, args ...string) (string, error) {
	t.Helper()

	loadSettings := func(*cli.Command) (config.Settings, error) {
		t.Fatal("config set should not load the settings")
		return config.Settings{}, nil
	}

	return run(t, NewCommand(environment, loadSettings), append([]string{"config", "set"}, args...)...)
}

func runView(t *testing.T, settings config.Settings) (string, error) {
	t.Helper()

	loadSettings := func(*cli.Command) (config.Settings, error) { return settings, nil }

	return run(t, NewCommand(config.Environment{}, loadSettings), "config", "view")
}

func run(t *testing.T, command *cli.Command, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	command.Writer = &out
	command.ErrWriter = io.Discard
	command.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	err := command.Run(t.Context(), args)

	return out.String(), err
}

func configFilePath(environment config.Environment) string {
	return filepath.Join(environment.Variables["XDG_CONFIG_HOME"], "ctwctl", "config.toml")
}
