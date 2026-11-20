package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/urfave/cli/v3"
)

const (
	defaultLimit = 100
	lineWidth    = 100
)

type logSearcher interface {
	Search(ctx context.Context, req o2.SearchRequest) (o2.SearchResult, error)
	Streams(ctx context.Context, streamType string) ([]o2.Stream, error)
}

func o2Command(deps dependencies) *cli.Command {
	sinceFlag := &cli.StringFlag{Name: "since", Usage: "fenêtre de temps, par exemple 15m ou 1h", Value: "1h"}
	limitFlag := &cli.IntFlag{Name: "limit", Usage: "nombre maximal de résultats", Value: defaultLimit}
	return &cli.Command{
		Name:  "o2",
		Usage: "streams, logs et requêtes SQL d'OpenObserve",
		Commands: []*cli.Command{
			{
				Name:  "streams",
				Usage: "liste les streams et leurs champs",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "type", Usage: "logs, metrics ou traces (défaut : logs et traces)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return listStreams(ctx, cmd, deps)
				},
			},
			{
				Name:  "logs",
				Usage: "lit les logs du stream default",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "service", Usage: "filtre sur service_name"},
					&cli.StringFlag{Name: "grep", Usage: "filtre sur le texte du message"},
					&cli.StringFlag{Name: "level", Usage: "niveau minimal : trace, debug, info, warn, error ou fatal"},
					sinceFlag,
					limitFlag,
					&cli.BoolFlag{Name: "follow", Aliases: []string{"f"}, Usage: "relit les logs à intervalle fixe"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return showLogs(ctx, cmd, deps)
				},
			},
			{
				Name:      "query",
				Usage:     "exécute une requête SQL telle quelle",
				ArgsUsage: "<SQL>",
				Flags:     []cli.Flag{sinceFlag, limitFlag},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runQuery(ctx, cmd, deps)
				},
			},
		},
	}
}

func searcherFor(s settings, deps dependencies) (logSearcher, error) {
	var missing []string
	if s.O2.User == "" {
		missing = append(missing, "o2.user (CTWCTL_O2_USER)")
	}
	if s.O2.Token == "" {
		missing = append(missing, "o2.token (CTWCTL_O2_TOKEN)")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("identifiants OpenObserve absents : %s", strings.Join(missing, ", "))
	}
	return deps.newLogSearcher(s), nil
}

func parseSince(value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("--since %q invalide : une durée positive est attendue, par exemple 15m ou 1h", value)
	}
	return d, nil
}

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

func showLogs(ctx context.Context, cmd *cli.Command, deps dependencies) error {
	s := settingsFrom(ctx)
	searcher, err := searcherFor(s, deps)
	if err != nil {
		return err
	}
	since, err := parseSince(cmd.String("since"))
	if err != nil {
		return err
	}
	limit := cmd.Int("limit")
	if limit <= 0 {
		return fmt.Errorf("--limit doit être strictement positif, reçu %d", limit)
	}
	namespace := ""
	if cmd.IsSet("namespace") {
		namespace = s.Kube.Namespace
	}
	filter, err := newLogFilter(cmd.String("service"), namespace, cmd.String("grep"), cmd.String("level"))
	if err != nil {
		return err
	}
	printer := logPrinter{w: writer(cmd), json: s.Output == outputJSON}
	if cmd.Bool("follow") {
		follower := newFollower(searcher, filter, limit, deps.pollInterval, deps.now, printer)
		err := follower.run(ctx, since)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	end := deps.now()
	result, err := searcher.Search(ctx, o2.SearchRequest{
		SQL:   filter.sql(orderNewestFirst),
		Start: end.Add(-since),
		End:   end,
		Size:  limit,
	})
	if err != nil {
		return err
	}
	return printer.printAll(result.Hits)
}

func runQuery(ctx context.Context, cmd *cli.Command, deps dependencies) error {
	if cmd.Args().Len() != 1 {
		return errors.New("une requête SQL est attendue : ctwctl o2 query \"<SQL>\"")
	}
	s := settingsFrom(ctx)
	searcher, err := searcherFor(s, deps)
	if err != nil {
		return err
	}
	since, err := parseSince(cmd.String("since"))
	if err != nil {
		return err
	}
	end := deps.now()
	result, err := searcher.Search(ctx, o2.SearchRequest{
		SQL:   cmd.Args().First(),
		Start: end.Add(-since),
		End:   end,
		Size:  cmd.Int("limit"),
	})
	if err != nil {
		return err
	}
	if s.Output == outputJSON {
		return logPrinter{w: writer(cmd), json: true}.printAll(result.Hits)
	}
	return writeTable(writer(cmd), result.Hits)
}

func writeTable(w io.Writer, hits []o2.Hit) error {
	columns := columnNames(hits)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(columns, "\t"))
	for _, hit := range hits {
		cells := make([]string, len(columns))
		for i, column := range columns {
			cells[i] = fmt.Sprint(hit[column])
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

func columnNames(hits []o2.Hit) []string {
	set := map[string]struct{}{}
	for _, hit := range hits {
		for column := range hit {
			set[column] = struct{}{}
		}
	}
	columns := make([]string, 0, len(set))
	for column := range set {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	return columns
}

type logPrinter struct {
	w    io.Writer
	json bool
}

func (p logPrinter) printAll(hits []o2.Hit) error {
	for _, hit := range hits {
		if err := p.print(hit); err != nil {
			return err
		}
	}
	return nil
}

func (p logPrinter) print(hit o2.Hit) error {
	if p.json {
		return writeJSON(p.w, hit)
	}
	_, err := fmt.Fprintf(p.w, "%s %s  %s\n", formatTimestamp(hit), origin(hit), fmt.Sprint(hit["body"]))
	return err
}

func formatTimestamp(hit o2.Hit) string {
	micros, ok := timestampOf(hit)
	if !ok {
		return "-"
	}
	return time.UnixMicro(micros).UTC().Format(time.RFC3339)
}

func origin(hit o2.Hit) string {
	namespace, _ := hit["k8s_namespace_name"].(string)
	pod, _ := hit["k8s_pod_name"].(string)
	if namespace == "" && pod == "" {
		service, _ := hit["service_name"].(string)
		if service == "" {
			return "-"
		}
		return service
	}
	return namespace + "/" + pod
}

func timestampOf(hit o2.Hit) (int64, bool) {
	switch v := hit["_timestamp"].(type) {
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	default:
		return 0, false
	}
}
