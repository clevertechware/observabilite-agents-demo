package o2

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

func run(t *testing.T, environment config.Environment, deps Dependencies, args ...string) (string, error) {
	t.Helper()
	return runWithContext(t.Context(), environment, deps, args...)
}

func runWithContext(
	ctx context.Context, environment config.Environment, deps Dependencies, args ...string,
) (string, error) {
	var out bytes.Buffer
	cmd := &cli.Command{
		Name:           "ctwctl",
		Flags:          cmdutil.GlobalFlags(),
		Before:         cmdutil.LoadConfig(environment),
		Commands:       []*cli.Command{NewCommand(deps)},
		Writer:         &out,
		ErrWriter:      io.Discard,
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
	}
	err := cmd.Run(ctx, append([]string{"ctwctl"}, args...))
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
