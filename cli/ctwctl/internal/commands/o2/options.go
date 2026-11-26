package o2

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

const (
	streamTypeLogs    = "logs"
	streamTypeTraces  = "traces"
	streamTypeMetrics = "metrics"
)

var streamTypes = []string{streamTypeLogs, streamTypeTraces, streamTypeMetrics}

func parseStreamType(value string) (string, error) {
	if !slices.Contains(streamTypes, value) {
		return "", fmt.Errorf("invalid --type %q: %s expected", value, strings.Join(streamTypes, ", "))
	}

	return value, nil
}

type timeWindow struct {
	start time.Time
	end   time.Time
}

func windowFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "since", Usage: "time window ending now, for example 15m or 1h", Value: "1h"},
		&cli.StringFlag{Name: "from", Usage: "window start in RFC 3339, for example 2026-11-26T14:05:00+01:00"},
		&cli.StringFlag{Name: "to", Usage: "window end in RFC 3339 (default: now when --from is set)"},
	}
}

// parseWindow resolves --since, or the absolute --from and --to, into a window. The absolute form anchors a search
// on a past incident: a relative window always ends now, so a descending sort returns the most recent entries.
func parseWindow(cmd *cli.Command, now time.Time) (timeWindow, error) {
	if !cmd.IsSet("from") && !cmd.IsSet("to") {
		since, err := parseSince(cmd)
		if err != nil {
			return timeWindow{}, err
		}

		return timeWindow{start: now.Add(-since), end: now}, nil
	}

	if cmd.IsSet("since") {
		return timeWindow{}, errors.New("--since cannot be combined with --from or --to")
	}
	if !cmd.IsSet("from") {
		return timeWindow{}, errors.New("--to requires --from")
	}

	start, err := parseInstant("--from", cmd.String("from"))
	if err != nil {
		return timeWindow{}, err
	}

	end := now
	if cmd.IsSet("to") {
		if end, err = parseInstant("--to", cmd.String("to")); err != nil {
			return timeWindow{}, err
		}
	}

	if !start.Before(end) {
		return timeWindow{}, fmt.Errorf("--from %s must be before the end of the window %s",
			start.Format(time.RFC3339), end.Format(time.RFC3339))
	}

	return timeWindow{start: start, end: end}, nil
}

func parseInstant(flag, value string) (time.Time, error) {
	instant, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s %q: RFC 3339 expected, for example 2026-11-26T14:05:00+01:00", flag, value)
	}

	return instant, nil
}
