package o2

import (
	"fmt"
	"slices"
	"strings"
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
