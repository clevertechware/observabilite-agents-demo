package orders

import (
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdtest"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
)

func run(t *testing.T, deps Dependencies, args ...string) (string, error) {
	t.Helper()

	return runIn(t, cmdtest.IsolatedEnvironment(t, nil), deps, args...)
}

func runIn(t *testing.T, environment config.Environment, deps Dependencies, args ...string) (string, error) {
	t.Helper()

	return cmdtest.Run(t.Context(), environment, NewCommand(deps), args...)
}
