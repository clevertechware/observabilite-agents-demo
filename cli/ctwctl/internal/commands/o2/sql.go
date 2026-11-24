package o2

import (
	"fmt"
	"strings"
)

const logStream = "default"

var severityNumbers = map[string]int{
	"trace": 1,
	"debug": 5,
	"info":  9,
	"warn":  13,
	"error": 17,
	"fatal": 21,
}

type logFilter struct {
	service     string
	namespace   string
	grep        string
	minSeverity int
}

func newLogFilter(service, namespace, grep, level string) (logFilter, error) {
	filter := logFilter{service: service, namespace: namespace, grep: grep}
	if level == "" {
		return filter, nil
	}

	severity, ok := severityNumbers[strings.ToLower(level)]
	if !ok {
		return logFilter{}, fmt.Errorf("niveau %q inconnu : trace, debug, info, warn, error ou fatal", level)
	}

	filter.minSeverity = severity
	return filter, nil
}

const (
	orderNewestFirst = "DESC"
	orderOldestFirst = "ASC"
)

func (f logFilter) sql(order string) string {
	var conditions []string
	if f.service != "" {
		conditions = append(conditions, "service_name = "+sqlString(f.service))
	}
	if f.namespace != "" {
		conditions = append(conditions, "k8s_namespace_name = "+sqlString(f.namespace))
	}
	if f.minSeverity > 0 {
		conditions = append(conditions, fmt.Sprintf("severity_number >= %d", f.minSeverity))
	}
	if f.grep != "" {
		conditions = append(conditions, "body LIKE "+sqlString("%"+f.grep+"%"))
	}

	query := fmt.Sprintf("SELECT * FROM %q", logStream)
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	return query + " ORDER BY _timestamp " + order
}

func sqlString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
