package apps

import (
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdtest"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
)

func run(t *testing.T, deps Dependencies, args ...string) (string, error) {
	t.Helper()

	deps.LoadSettings = cmdutil.NewSettingsLoader(cmdtest.IsolatedEnvironment(t, nil))

	return cmdtest.Run(t.Context(), NewCommand(deps), args...)
}
