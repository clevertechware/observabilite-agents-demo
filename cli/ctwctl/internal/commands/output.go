package commands

import (
	"io"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
)

func writeJSON(w io.Writer, v any) error {
	return cmdutil.WriteJSON(w, v)
}
