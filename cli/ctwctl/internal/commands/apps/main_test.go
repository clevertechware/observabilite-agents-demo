package apps

import (
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdtest"
)

func run(t *testing.T, deps Dependencies, args ...string) (string, error) {
	t.Helper()

	return cmdtest.RunWithEnvironment(t.Context(), cmdtest.IsolatedEnvironment(t, nil), NewCommand(deps), args...)
}
