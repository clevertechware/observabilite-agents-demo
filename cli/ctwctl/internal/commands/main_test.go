package commands

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdtest"
	"github.com/urfave/cli/v3"
)

// run executes ctwctl with args and returns its output. Without an environment in deps, it runs in an isolated one so
// that the user's configuration and variables never leak into a test.
func run(t *testing.T, deps dependencies, args ...string) (string, error) {
	t.Helper()
	if deps.environment.Variables == nil {
		deps.environment = cmdtest.IsolatedEnvironment(t, nil)
	}

	var out bytes.Buffer
	cmd := newRootCommand(deps)
	cmd.Writer = &out
	cmd.ErrWriter = io.Discard
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
	err := cmd.Run(t.Context(), append([]string{"ctwctl"}, args...))
	return out.String(), err
}
