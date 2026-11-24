package o2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogFilterSQL_BuildsOneConditionPerFilterInAnOrderedQuery(t *testing.T) {
	tests := []struct {
		name   string
		filter logFilter
		order  string
		want   string
	}{
		{
			"no filter reads the whole stream", logFilter{}, orderNewestFirst,
			`SELECT * FROM "default" ORDER BY _timestamp DESC`,
		},
		{
			"a service restricts on service_name", logFilter{service: "order-service"}, orderNewestFirst,
			`SELECT * FROM "default" WHERE service_name = 'order-service' ORDER BY _timestamp DESC`,
		},
		{
			"all filters are combined with AND", logFilter{
				service: "order-service", namespace: "demo", minSeverity: 17, grep: "Stock",
			}, orderOldestFirst,
			`SELECT * FROM "default" WHERE service_name = 'order-service' AND k8s_namespace_name = 'demo' ` +
				`AND severity_number >= 17 AND body LIKE '%Stock%' ORDER BY _timestamp ASC`,
		},
		{
			"single quotes are doubled so a value cannot close the string", logFilter{service: "a' OR '1'='1"},
			orderNewestFirst,
			`SELECT * FROM "default" WHERE service_name = 'a'' OR ''1''=''1' ORDER BY _timestamp DESC`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(st *testing.T) {
			assert.Equal(st, tt.want, tt.filter.sql(tt.order))
		})
	}
}

func TestNewLogFilter_MapsLevelNamesToSeverityNumbers(t *testing.T) {
	tests := []struct {
		level string
		want  int
	}{
		{"", 0}, {"debug", 5}, {"info", 9}, {"warn", 13}, {"error", 17}, {"ERROR", 17}, {"fatal", 21},
	}

	for _, tt := range tests {
		t.Run("level "+tt.level, func(st *testing.T) {
			filter, err := newLogFilter("", "", "", tt.level)

			require.NoError(st, err)
			assert.Equal(st, tt.want, filter.minSeverity)
		})
	}
}

func TestNewLogFilter_WithUnknownLevel_ListsTheValidOnes(t *testing.T) {
	_, err := newLogFilter("", "", "", "loud")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error")
}
