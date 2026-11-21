package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/urfave/cli/v3"
)

const lineWidth = 100

func listStreams(ctx context.Context, cmd *cli.Command, deps dependencies) error {
	s := settingsFrom(ctx)
	searcher, err := searcherFor(s, deps)
	if err != nil {
		return err
	}
	types := []string{"logs", "traces"}
	if t := cmd.String("type"); t != "" {
		types = []string{t}
	}
	streams := []o2.Stream{}
	for _, t := range types {
		found, err := searcher.Streams(ctx, t)
		if err != nil {
			return err
		}
		streams = append(streams, found...)
	}
	if s.Output == outputJSON {
		return writeJSON(writer(cmd), streams)
	}
	for _, stream := range streams {
		if _, err := fmt.Fprintf(writer(cmd), "%s/%s\n", stream.Type, stream.Name); err != nil {
			return err
		}
		if err := writeWrapped(writer(cmd), stream.Fields); err != nil {
			return err
		}
	}
	return nil
}

func writeWrapped(w io.Writer, fields []string) error {
	line := ""
	flush := func() error {
		if line == "" {
			return nil
		}
		_, err := fmt.Fprintf(w, "  %s\n", line)
		line = ""
		return err
	}
	for i, field := range fields {
		item := field
		if i < len(fields)-1 {
			item += ","
		}
		if line != "" && len(line)+1+len(item) > lineWidth {
			if err := flush(); err != nil {
				return err
			}
		}
		if line != "" {
			line += " "
		}
		line += item
	}
	return flush()
}
