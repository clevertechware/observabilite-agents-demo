package orders

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/urfave/cli/v3"
)

func run(t *testing.T, deps Dependencies, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	command := NewCommand(deps)
	command.Writer = &out
	command.ErrWriter = io.Discard
	command.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	err := command.Run(t.Context(), args)

	return out.String(), err
}
