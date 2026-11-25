// Package cmdtest contains test helpers shared by ctwctl command packages.
package cmdtest

import (
	"bytes"
	"context"
	"io"
	"maps"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

// IsolatedEnvironment returns an environment with its own empty configuration directory and the given variables, so
// that the user's configuration and variables never leak into a test.
func IsolatedEnvironment(t *testing.T, variables map[string]string) config.Environment {
	t.Helper()

	all := map[string]string{"XDG_CONFIG_HOME": t.TempDir()}
	maps.Copy(all, variables)

	return config.Environment{Variables: all, UserHomeDir: func() (string, error) {
		t.Fatal("user home should not be requested when XDG_CONFIG_HOME is set")
		return "", nil
	}}
}

// Run executes command under a ctwctl root that only declares the global flags, and returns the output.
func Run(ctx context.Context, command *cli.Command, args ...string) (string, error) {
	root := &cli.Command{Name: "ctwctl", Flags: cmdutil.GlobalFlags(), Commands: []*cli.Command{command}}

	return RunRoot(ctx, root, args...)
}

// RunRoot executes the root command with args, and returns what it writes to its output.
func RunRoot(ctx context.Context, root *cli.Command, args ...string) (string, error) {
	var out bytes.Buffer
	root.Writer = &out
	root.ErrWriter = io.Discard
	root.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	err := root.Run(ctx, append([]string{root.Name}, args...))

	return out.String(), err
}
