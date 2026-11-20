package commands

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ctwctl-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	for _, entry := range os.Environ() {
		if name, _, _ := strings.Cut(entry, "="); strings.HasPrefix(name, envPrefix) {
			os.Unsetenv(name)
		}
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	os.Exit(m.Run())
}

func run(t *testing.T, deps dependencies, args ...string) (string, error) {
	t.Helper()
	return runWithContext(t.Context(), deps, args...)
}

func runWithContext(ctx context.Context, deps dependencies, args ...string) (string, error) {
	var out bytes.Buffer
	cmd := newRootCommand(deps)
	cmd.Writer = &out
	cmd.ErrWriter = io.Discard
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
	err := cmd.Run(ctx, append([]string{"ctwctl"}, args...))
	return out.String(), err
}

func isolatedConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}
