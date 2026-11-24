package o2

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/urfave/cli/v3"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ctwctl-test-")
	if err != nil {
		panic(err)
	}

	for _, entry := range os.Environ() {
		if name, _, _ := strings.Cut(entry, "="); strings.HasPrefix(name, "CTWCTL_") {
			os.Unsetenv(name)
		}
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	if err = os.RemoveAll(dir); err != nil {
		code = 1
	}
	os.Exit(code)
}

func run(t *testing.T, deps Dependencies, args ...string) (string, error) {
	t.Helper()
	return runWithContext(t.Context(), deps, args...)
}

func runWithContext(ctx context.Context, deps Dependencies, args ...string) (string, error) {
	var out bytes.Buffer
	cmd := &cli.Command{
		Name:           "ctwctl",
		Flags:          cmdutil.GlobalFlags(),
		Before:         cmdutil.Before,
		Commands:       []*cli.Command{NewCommand(deps)},
		Writer:         &out,
		ErrWriter:      io.Discard,
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
	}
	err := cmd.Run(ctx, append([]string{"ctwctl"}, args...))
	return out.String(), err
}

func isolatedConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}
