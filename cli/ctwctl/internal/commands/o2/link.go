package o2

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/urfave/cli/v3"
)

// queryEncoder turns the standard base64 alphabet into the one of the OpenObserve interface, which reads the query
// parameter back with b64DecodeUnicode (web/src/utils/formatters.ts).
var queryEncoder = strings.NewReplacer("+", "-", "/", "_", "=", ".")

func encodeQuery(sql string) string {
	return queryEncoder.Replace(base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(sql))))
}

type linkTarget struct {
	baseURL string
	org     string
	stream  string
	window  timeWindow
}

func (t linkTarget) logsURL(streamType, sql string) string {
	return t.baseURL + "/web/logs?" + joinQuery(
		"org_identifier", t.org,
		"stream_type", streamType,
		"stream", t.stream,
		"from", microseconds(t.window.start),
		"to", microseconds(t.window.end),
		"sql_mode", "true",
		"query", encodeQuery(sql),
	)
}

func (t linkTarget) traceURL(traceID string) string {
	return t.baseURL + "/web/traces/trace-details?" + joinQuery(
		"org_identifier", t.org,
		"stream", t.stream,
		"trace_id", traceID,
		"from", microseconds(t.window.start),
		"to", microseconds(t.window.end),
	)
}

// joinQuery keeps the parameters in the given order, which url.Values would sort alphabetically.
func joinQuery(pairs ...string) string {
	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		parts = append(parts, url.QueryEscape(pairs[i])+"="+url.QueryEscape(pairs[i+1]))
	}

	return strings.Join(parts, "&")
}

func microseconds(t time.Time) string {
	return fmt.Sprint(t.UnixMicro())
}

func linkFlags() []cli.Flag {
	return append([]cli.Flag{
		&cli.StringFlag{Name: "sql", Usage: "SQL query to open in the logs page"},
		&cli.StringFlag{Name: "trace-id", Usage: "trace to open in the trace details page"},
		&cli.StringFlag{Name: "stream", Usage: "stream name", Value: logStream},
		&cli.StringFlag{Name: "type", Usage: "stream type of the SQL query: logs, traces or metrics", Value: streamTypeLogs},
	}, windowFlags()...)
}

type linkOutput struct {
	URL string `json:"url"`
}

func printLink(ctx context.Context, cmd *cli.Command, deps Dependencies) error {
	sql, traceID := cmd.String("sql"), cmd.String("trace-id")
	if (sql == "") == (traceID == "") {
		return errors.New("exactly one of --sql and --trace-id is required")
	}
	if traceID != "" && cmd.IsSet("type") {
		return errors.New("--type applies to --sql only")
	}

	streamType, err := parseStreamType(cmd.String("type"))
	if err != nil {
		return err
	}

	window, err := parseWindow(cmd, deps.Now())
	if err != nil {
		return err
	}

	s, err := deps.LoadSettings(cmd)
	if err != nil {
		return err
	}

	target := linkTarget{
		baseURL: strings.TrimRight(s.O2.URL, "/"),
		org:     s.O2.Org,
		stream:  cmd.String("stream"),
		window:  window,
	}

	link := target.traceURL(traceID)
	if sql != "" {
		link = target.logsURL(streamType, sql)
	}

	if s.Output == config.OutputJSON {
		return writeJSON(cmd.Root().Writer, linkOutput{URL: link})
	}

	_, err = fmt.Fprintln(cmd.Root().Writer, link)
	return err
}
