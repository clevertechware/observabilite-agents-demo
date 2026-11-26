package o2

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/o2/mocks"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	incidentFrom = "2026-11-26T14:05:00+01:00"
	incidentTo   = "2026-11-26T14:20:00+01:00"
	fromMicros   = "1795698300000000"
	toMicros     = "1795699200000000"
	lastHourFrom = "1795165200000000"
	lastHourTo   = "1795168800000000"
)

var linkSettings = config.Settings{
	Output: config.OutputText,
	O2:     config.O2Settings{URL: "https://o2.obs.scw.clevertechware.fr/", Org: "default"},
}

func runLink(t *testing.T, settings config.Settings, args ...string) (string, error) {
	t.Helper()

	return run(t, o2Deps(mocks.NewLogSearcher(t), settings), append([]string{"o2", "link"}, args...)...)
}

func decodeQuery(t *testing.T, encoded string) string {
	t.Helper()

	standard := strings.NewReplacer("-", "+", "_", "/", ".", "=").Replace(encoded)
	decoded, err := base64.StdEncoding.DecodeString(standard)
	require.NoError(t, err)

	return string(decoded)
}

func TestEncodeQuery_ProducesTheAlphabetOfTheOpenObserveInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"a string whose standard base64 contains plus and slash", "<<???>>>", "PDw_Pz8-Pj4."},
		{"a padded string", "SELECT 1", "U0VMRUNUIDE."},
		{
			"quotes, equals, greater-than and accents",
			`SELECT * FROM "default" WHERE body LIKE '%café%' AND code='500' AND n > 3`,
			"U0VMRUNUICogRlJPTSAiZGVmYXVsdCIgV0hFUkUgYm9keSBMSUtFICclY2Fmw6klJyBBTkQgY29kZT0nNTAwJyBBTkQgbiA-IDM.",
		},
		{"surrounding whitespace, trimmed as the interface does", "  SELECT 1\n", "U0VMRUNUIDE."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(st *testing.T) {
			st.Parallel()

			encoded := encodeQuery(tt.sql)

			assert.Equal(st, tt.want, encoded)
			assert.NotContains(st, encoded, "+")
			assert.NotContains(st, encoded, "/")
			assert.NotContains(st, encoded, "=")
			assert.Equal(st, strings.TrimSpace(tt.sql), decodeQuery(st, encoded))
		})
	}
}

func TestO2Link_WithSQL_PrintsTheLogsPageURLOverTheAbsoluteWindow(t *testing.T) {
	t.Parallel()

	out, err := runLink(t, linkSettings, "--sql", "SELECT 1", "--from", incidentFrom, "--to", incidentTo)

	require.NoError(t, err)
	assert.Equal(t, "https://o2.obs.scw.clevertechware.fr/web/logs?org_identifier=default&stream_type=logs&stream=default"+
		"&from="+fromMicros+"&to="+toMicros+"&sql_mode=true&query=U0VMRUNUIDE.\n", out)
}

func TestO2Link_WithSQLAndSince_EndsTheWindowNow(t *testing.T) {
	t.Parallel()

	out, err := runLink(t, linkSettings, "--sql", "SELECT 1", "--since", "1h")

	require.NoError(t, err)
	assert.Contains(t, out, "&from="+lastHourFrom+"&to="+lastHourTo+"&")
}

func TestO2Link_WithStreamAndType_TargetsThatStream(t *testing.T) {
	t.Parallel()

	out, err := runLink(t, linkSettings, "--sql", "SELECT 1", "--stream", "my stream", "--type", "traces")

	require.NoError(t, err)
	assert.Contains(t, out, "&stream_type=traces&stream=my+stream&")
}

func TestO2Link_WithQueryOfSpecialCharacters_RoundTripsThroughTheURL(t *testing.T) {
	t.Parallel()

	sql := `SELECT * FROM "default" WHERE body LIKE '%café%' AND code='500' AND n > 3`

	out, err := runLink(t, linkSettings, "--sql", sql)

	require.NoError(t, err)
	parsed, err := url.Parse(strings.TrimSpace(out))
	require.NoError(t, err)
	assert.Equal(t, sql, decodeQuery(t, parsed.Query().Get("query")))
}

func TestO2Link_WithTraceID_PrintsTheTraceDetailsURL(t *testing.T) {
	t.Parallel()

	out, err := runLink(t, linkSettings, "--trace-id", "4bf92f3577b34da6a3ce929d0e0e4736", "--from", incidentFrom, "--to", incidentTo)

	require.NoError(t, err)
	assert.Equal(t, "https://o2.obs.scw.clevertechware.fr/web/traces/trace-details?org_identifier=default&stream=default"+
		"&trace_id=4bf92f3577b34da6a3ce929d0e0e4736&from="+fromMicros+"&to="+toMicros+"\n", out)
}

func TestO2Link_WithJSONOutput_PrintsTheURLInAnObject(t *testing.T) {
	t.Parallel()

	out, err := runLink(t, withOutput(linkSettings, config.OutputJSON), "--sql", "SELECT 1", "--from", incidentFrom, "--to", incidentTo)

	require.NoError(t, err)
	assert.JSONEq(t, `{"url":"https://o2.obs.scw.clevertechware.fr/web/logs?org_identifier=default&stream_type=logs`+
		`&stream=default&from=`+fromMicros+`&to=`+toMicros+`&sql_mode=true&query=U0VMRUNUIDE."}`, out)
}

func TestO2Link_WithoutCredentials_StillPrintsTheURL(t *testing.T) {
	t.Parallel()

	out, err := runLink(t, linkSettings, "--sql", "SELECT 1")

	require.NoError(t, err)
	assert.Contains(t, out, "https://o2.obs.scw.clevertechware.fr/web/logs?")
}

func TestO2Link_WithInvalidArguments_FailsWithoutPrinting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"neither sql nor trace id", nil, "exactly one"},
		{"both sql and trace id", []string{"--sql", "SELECT 1", "--trace-id", "abc"}, "exactly one"},
		{"a type with a trace id", []string{"--trace-id", "abc", "--type", "traces"}, "--type"},
		{"an unknown type", []string{"--sql", "SELECT 1", "--type", "spans"}, "spans"},
		{"since with from", []string{"--sql", "SELECT 1", "--since", "1h", "--from", incidentFrom}, "--since"},
		{"to without from", []string{"--sql", "SELECT 1", "--to", incidentTo}, "--to requires --from"},
		{"from after to", []string{"--sql", "SELECT 1", "--from", incidentTo, "--to", incidentFrom}, "--from"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(st *testing.T) {
			st.Parallel()

			out, err := runLink(st, linkSettings, tt.args...)

			require.Error(st, err)
			assert.Contains(st, err.Error(), tt.want)
			assert.Empty(st, out)
		})
	}
}
