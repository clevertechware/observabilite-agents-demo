package o2

import (
	"context"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdtest"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
)

func run(t *testing.T, environment config.Environment, deps Dependencies, args ...string) (string, error) {
	t.Helper()

	return runWithContext(t.Context(), environment, deps, args...)
}

func runWithContext(
	ctx context.Context, environment config.Environment, deps Dependencies, args ...string,
) (string, error) {
	return cmdtest.RunWithEnvironment(ctx, environment, NewCommand(deps), args...)
}
