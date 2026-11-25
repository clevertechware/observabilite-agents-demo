package commands

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

// run executes ctwctl with args and returns its output. Without an environment in deps, it runs in an isolated one so
// that the user's configuration and variables never leak into a test.
func run(t *testing.T, deps dependencies, args ...string) (string, error) {
	t.Helper()
	if deps.environment.Variables == nil {
		deps.environment = isolatedEnvironment(t, nil)
	}

	var out bytes.Buffer
	cmd := newRootCommand(deps)
	cmd.Writer = &out
	cmd.ErrWriter = io.Discard
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
	err := cmd.Run(t.Context(), append([]string{"ctwctl"}, args...))
	return out.String(), err
}

func isolatedEnvironment(t *testing.T, variables map[string]string) config.Environment {
	t.Helper()
	all := map[string]string{"XDG_CONFIG_HOME": t.TempDir()}
	maps.Copy(all, variables)

	return config.Environment{Variables: all, UserHomeDir: func() (string, error) {
		t.Fatal("user home should not be requested when XDG_CONFIG_HOME is set")
		return "", nil
	}}
}

func configFilePath(environment config.Environment) string {
	return filepath.Join(environment.Variables["XDG_CONFIG_HOME"], "ctwctl", "config.toml")
}
