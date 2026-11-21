package o2

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/cmdutil"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
)

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
		return cmdutil.WriteJSON(p.w, hit)
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
