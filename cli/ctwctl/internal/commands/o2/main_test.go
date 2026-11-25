package o2

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/urfave/cli/v3"
)

func run(t *testing.T, deps Dependencies, args ...string) (string, error) {
	t.Helper()

	return runWithContext(t.Context(), deps, args...)
}

// runWithContext executes the o2 commands under a root that declares --namespace, the global flag that o2 logs reads.
func runWithContext(ctx context.Context, deps Dependencies, args ...string) (string, error) {
	var out bytes.Buffer
	root := &cli.Command{
		Name:           "ctwctl",
		Flags:          []cli.Flag{&cli.StringFlag{Name: "namespace"}},
		Commands:       []*cli.Command{NewCommand(deps)},
		Writer:         &out,
		ErrWriter:      io.Discard,
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
	}

	err := root.Run(ctx, append([]string{root.Name}, args...))

	return out.String(), err
}
