package o2

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/commands/o2/mocks"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

var fixedNow = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

func o2Deps(searcher LogSearcher, settings config.Settings) Dependencies {
	return Dependencies{
		LoadSettings:   func(*cli.Command) (config.Settings, error) { return settings, nil },
		NewLogSearcher: func(config.Settings) LogSearcher { return searcher },
		Now:            func() time.Time { return fixedNow },
		PollInterval:   time.Millisecond,
	}
}

var (
	credentialed = config.Settings{
		Output: config.OutputText,
		O2:     config.O2Settings{User: "ctwctl@sa.internal", Token: "jeton-de-test"},
		Kube:   config.KubeSettings{Namespace: "demo"},
	}
	credentialedJSON = withOutput(credentialed, config.OutputJSON)
)

func withOutput(settings config.Settings, output string) config.Settings {
	settings.Output = output
	return settings
}

func hitAt(t time.Time, body string) o2.Hit {
	return o2.Hit{
		"_timestamp":         json.Number(strconv.FormatInt(t.UnixMicro(), 10)),
		"body":               body,
		"service_name":       "order-service",
		"k8s_namespace_name": "demo",
		"k8s_pod_name":       "order-service-7c9d5b6f4-x2k4p",
	}
}

func splitLines(out string) []string {
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

func TestO2Commands_WithoutCredentials_NameTheMissingConfigurationKeys(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	withoutCredentials := config.Settings{Output: config.OutputText}

	for _, args := range [][]string{{"o2", "streams"}, {"o2", "logs"}, {"o2", "query", "SELECT 1"}} {
		_, err := run(t, o2Deps(searcher, withoutCredentials), args...)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "CTWCTL_O2_USER")
		assert.Contains(t, err.Error(), "CTWCTL_O2_TOKEN")
	}
}

func TestO2Streams_ByDefault_ListsLogsAndTraces(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Streams(mock.Anything, "logs").Return([]o2.Stream{
		{Name: "default", Type: "logs", Fields: []string{"_timestamp", "body", "service_name"}},
		{Name: "triggers", Type: "logs", Fields: []string{}},
	}, nil)
	searcher.EXPECT().Streams(mock.Anything, "traces").Return([]o2.Stream{
		{Name: "default", Type: "traces", Fields: []string{}},
	}, nil)

	out, err := run(t, o2Deps(searcher, credentialed), "o2", "streams")

	require.NoError(t, err)
	assert.Equal(t, "logs/default\n  _timestamp, body, service_name\nlogs/triggers\ntraces/default\n", out)
}

func TestO2Streams_WithTypeFlag_QueriesOnlyThatType(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Streams(mock.Anything, "metrics").Return([]o2.Stream{
		{Name: "up", Type: "metrics", Fields: []string{"value"}},
	}, nil)

	out, err := run(t, o2Deps(searcher, credentialedJSON), "o2", "streams", "--type", "metrics")

	require.NoError(t, err)
	assert.JSONEq(t, `[{"name":"up","type":"metrics","fields":["value"]}]`, out)
}

func TestO2Streams_WithNoStreamInJSON_PrintsAnEmptyArrayRatherThanNull(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Streams(mock.Anything, "metrics").Return(nil, nil)

	out, err := run(t, o2Deps(searcher, credentialedJSON), "o2", "streams", "--type", "metrics")

	require.NoError(t, err)
	assert.Equal(t, "[]\n", out)
}

func TestO2Streams_WithManyFields_WrapsTheListUnderTheLineWidth(t *testing.T) {
	t.Parallel()

	fields := make([]string, 0, 30)
	for range 30 {
		fields = append(fields, "k8s_container_name")
	}
	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Streams(mock.Anything, "logs").Return([]o2.Stream{{Name: "default", Type: "logs", Fields: fields}}, nil)
	searcher.EXPECT().Streams(mock.Anything, "traces").Return(nil, nil)

	out, err := run(t, o2Deps(searcher, credentialed), "o2", "streams")

	require.NoError(t, err)
	for _, line := range splitLines(out) {
		assert.LessOrEqual(t, len(line), lineWidth+2)
	}
	assert.Greater(t, len(splitLines(out)), 2)
}

func TestO2Logs_ByDefault_SearchesTheLastHourNewestFirstAndPrintsOneLinePerLog(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, o2.SearchRequest{
		SQL:   `SELECT * FROM "default" WHERE service_name = 'order-service' ORDER BY _timestamp DESC`,
		Start: fixedNow.Add(-time.Hour),
		End:   fixedNow,
		Size:  2,
	}).Return(o2.SearchResult{Hits: []o2.Hit{
		hitAt(time.Date(2026, 11, 20, 9, 41, 12, 0, time.UTC), "Stock insuffisant pour SKU-LOW"),
		hitAt(time.Date(2026, 11, 20, 9, 40, 58, 0, time.UTC), "Commande créée"),
	}}, nil)

	out, err := run(t, o2Deps(searcher, credentialed), "o2", "logs", "--service", "order-service", "--since", "1h", "--limit", "2")

	require.NoError(t, err)
	assert.Equal(t, "2026-11-20T09:41:12Z demo/order-service-7c9d5b6f4-x2k4p  Stock insuffisant pour SKU-LOW\n"+
		"2026-11-20T09:40:58Z demo/order-service-7c9d5b6f4-x2k4p  Commande créée\n", out)
}

func TestO2Logs_WithJSONOutput_PrintsOneCompactObjectPerLine(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, mock.Anything).Return(o2.SearchResult{Hits: []o2.Hit{
		{"_timestamp": json.Number("1795167672000000"), "body": "a <b> & c", "severity_text": nil},
		{"_timestamp": json.Number("1795167671000000"), "body": "second"},
	}}, nil)

	out, err := run(t, o2Deps(searcher, credentialedJSON), "o2", "logs")

	require.NoError(t, err)
	//nolint:testifylint // the exact bytes are the contract: one compact JSON object per line, HTML left unescaped.
	assert.Equal(t, `{"_timestamp":1795167672000000,"body":"a <b> & c","severity_text":null}`+"\n"+
		`{"_timestamp":1795167671000000,"body":"second"}`+"\n", out)
}

func TestO2Logs_WithNoHits_PrintsNothing(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, mock.Anything).Return(o2.SearchResult{}, nil)

	out, err := run(t, o2Deps(searcher, credentialed), "o2", "logs")

	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestO2Logs_WithAllFilters_GeneratesTheCombinedSQLAndWindow(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, o2.SearchRequest{
		SQL: `SELECT * FROM "default" WHERE service_name = 'order-service' AND k8s_namespace_name = 'demo' ` +
			`AND severity_number >= 17 AND body LIKE '%timeout%' ORDER BY _timestamp DESC`,
		Start: fixedNow.Add(-15 * time.Minute),
		End:   fixedNow,
		Size:  100,
	}).Return(o2.SearchResult{}, nil)

	_, err := run(t, o2Deps(searcher, credentialed), "--namespace", "demo", "o2", "logs",
		"--service", "order-service", "--level", "error", "--grep", "timeout", "--since", "15m")

	require.NoError(t, err)
}

func TestO2Logs_WithoutNamespaceFlag_DoesNotFilterOnTheDefaultKubeNamespace(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, mock.MatchedBy(func(r o2.SearchRequest) bool {
		return r.SQL == `SELECT * FROM "default" ORDER BY _timestamp DESC`
	})).Return(o2.SearchResult{}, nil)

	_, err := run(t, o2Deps(searcher, credentialed), "o2", "logs")

	require.NoError(t, err)
}

func TestO2Logs_WithInvalidArguments_FailsBeforeAnySearch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"an unparsable window", []string{"--since", "hier"}, "--since"},
		{"a negative window", []string{"--since", "-5m"}, "--since"},
		{"a zero limit", []string{"--limit", "0"}, "--limit"},
		{"a negative limit", []string{"--limit", "-1"}, "--limit"},
		{"an unknown level", []string{"--level", "loud"}, "loud"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(st *testing.T) {
			st.Parallel()

			searcher := mocks.NewLogSearcher(st)

			_, err := run(st, o2Deps(searcher, credentialed), append([]string{"o2", "logs"}, tt.args...)...)

			require.Error(st, err)
			assert.Contains(st, err.Error(), tt.want)
		})
	}
}

func TestO2Query_WithNonPositiveLimit_FailsBeforeSearch(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"0", "-1"} {
		t.Run(value, func(st *testing.T) {
			st.Parallel()

			searcher := mocks.NewLogSearcher(st)

			_, err := run(st, o2Deps(searcher, credentialed), "o2", "query", "SELECT 1", "--limit", value)

			require.Error(st, err)
			assert.Contains(st, err.Error(), "--limit")
		})
	}
}

func TestO2Logs_WhenSearchFails_ReturnsTheClientError(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, mock.Anything).Return(o2.SearchResult{}, &o2.StatusError{Status: 401})

	_, err := run(t, o2Deps(searcher, credentialed), "o2", "logs")

	var statusErr *o2.StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, 401, statusErr.Status)
}

func TestO2Logs_WithFollow_PrintsOldestFirstThenOnlyLinesNotAlreadyShown(t *testing.T) {
	t.Parallel()

	base := fixedNow.Add(-30 * time.Second)
	first := hitAt(base, "premier")
	second := hitAt(base.Add(time.Second), "deuxième")
	third := hitAt(base.Add(2*time.Second), "troisième")
	fourth := hitAt(base.Add(3*time.Second), "quatrième")

	ctx, cancel := context.WithCancel(t.Context())
	var requests []o2.SearchRequest
	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, req o2.SearchRequest) (o2.SearchResult, error) {
			requests = append(requests, req)
			if len(requests) == 1 {
				return o2.SearchResult{Hits: []o2.Hit{third, second, first}}, nil
			}
			cancel()
			return o2.SearchResult{Hits: []o2.Hit{third, fourth}}, nil
		})

	out, err := runWithContext(ctx, o2Deps(searcher, credentialed), "o2", "logs", "--follow", "--limit", "3")

	require.NoError(t, err)
	assert.Equal(t, "2026-11-20T09:59:30Z demo/order-service-7c9d5b6f4-x2k4p  premier\n"+
		"2026-11-20T09:59:31Z demo/order-service-7c9d5b6f4-x2k4p  deuxième\n"+
		"2026-11-20T09:59:32Z demo/order-service-7c9d5b6f4-x2k4p  troisième\n"+
		"2026-11-20T09:59:33Z demo/order-service-7c9d5b6f4-x2k4p  quatrième\n", out)
	require.GreaterOrEqual(t, len(requests), 2)
	assert.Contains(t, requests[0].SQL, "ORDER BY _timestamp DESC")
	assert.Contains(t, requests[1].SQL, "ORDER BY _timestamp ASC")
	assert.Equal(t, base.UnixMicro(), requests[1].Start.UnixMicro())
}

func TestO2Query_SendsTheSQLUnchangedOverTheRequestedWindow(t *testing.T) {
	t.Parallel()

	sql := `SELECT service_name, count(*) AS erreurs FROM "default" WHERE severity_number >= 17 GROUP BY service_name`
	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, o2.SearchRequest{
		SQL:        sql,
		Start:      fixedNow.Add(-24 * time.Hour),
		End:        fixedNow,
		Size:       100,
		StreamType: "logs",
	}).Return(o2.SearchResult{Hits: []o2.Hit{
		{"service_name": "order-service", "erreurs": json.Number("3")},
	}}, nil)

	out, err := run(t, o2Deps(searcher, credentialedJSON), "o2", "query", sql, "--since", "24h")

	require.NoError(t, err)
	assert.JSONEq(t, `{"erreurs":3,"service_name":"order-service"}`, out)
}

func TestO2Query_WithTypeFlag_SearchesThatStreamType(t *testing.T) {
	t.Parallel()

	for _, streamType := range []string{"logs", "traces", "metrics"} {
		t.Run(streamType, func(st *testing.T) {
			st.Parallel()

			searcher := mocks.NewLogSearcher(st)
			searcher.EXPECT().Search(mock.Anything, mock.MatchedBy(func(req o2.SearchRequest) bool {
				return req.StreamType == streamType
			})).Return(o2.SearchResult{}, nil)

			_, err := run(st, o2Deps(searcher, credentialed), "o2", "query", "SELECT 1", "--type", streamType)

			require.NoError(st, err)
		})
	}
}

func TestO2Query_WithUnknownType_FailsBeforeSearch(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)

	_, err := run(t, o2Deps(searcher, credentialed), "o2", "query", "SELECT 1", "--type", "spans")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--type")
	assert.Contains(t, err.Error(), "spans")
}

func TestO2Query_WithFromAndTo_SendsTheAbsoluteWindowInMicroseconds(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, 11, 26, 13, 5, 0, 0, time.UTC)
	to := time.Date(2026, 11, 26, 13, 20, 0, 0, time.UTC)
	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, mock.MatchedBy(func(req o2.SearchRequest) bool {
		return req.Start.UnixMicro() == from.UnixMicro() && req.End.UnixMicro() == to.UnixMicro()
	})).Return(o2.SearchResult{}, nil)

	_, err := run(t, o2Deps(searcher, credentialed), "o2", "query", "SELECT 1",
		"--from", "2026-11-26T14:05:00+01:00", "--to", "2026-11-26T14:20:00+01:00")

	require.NoError(t, err)
}

func TestO2Query_WithFromOnly_EndsTheWindowNow(t *testing.T) {
	t.Parallel()

	from := fixedNow.Add(-3 * time.Hour)
	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, mock.MatchedBy(func(req o2.SearchRequest) bool {
		return req.Start.UnixMicro() == from.UnixMicro() && req.End.UnixMicro() == fixedNow.UnixMicro()
	})).Return(o2.SearchResult{}, nil)

	_, err := run(t, o2Deps(searcher, credentialed), "o2", "query", "SELECT 1", "--from", from.Format(time.RFC3339))

	require.NoError(t, err)
}

func TestO2Query_WithInvalidWindow_FailsBeforeSearch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"since combined with from", []string{"--since", "1h", "--from", "2026-11-26T14:05:00+01:00"}, "--since"},
		{"since combined with to", []string{"--since", "1h", "--to", "2026-11-26T14:05:00+01:00"}, "--since"},
		{"to without from", []string{"--to", "2026-11-26T14:05:00+01:00"}, "--to requires --from"},
		{"from after to", []string{"--from", "2026-11-26T15:00:00+01:00", "--to", "2026-11-26T14:00:00+01:00"}, "--from"},
		{"from equal to to", []string{"--from", "2026-11-26T14:00:00+01:00", "--to", "2026-11-26T14:00:00+01:00"}, "--from"},
		{"from in the future", []string{"--from", "2026-11-27T10:00:00Z"}, "--from"},
		{"from without time zone", []string{"--from", "2026-11-26T14:05:00"}, "RFC 3339"},
		{"to not a date", []string{"--from", "2026-11-26T14:05:00Z", "--to", "hier"}, "invalid --to"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(st *testing.T) {
			st.Parallel()

			searcher := mocks.NewLogSearcher(st)

			_, err := run(st, o2Deps(searcher, credentialed), append([]string{"o2", "query", "SELECT 1"}, tt.args...)...)

			require.Error(st, err)
			assert.Contains(st, err.Error(), tt.want)
		})
	}
}

func TestO2Query_WithTextOutput_PrintsAnAlignedTableWithSortedColumns(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, mock.Anything).Return(o2.SearchResult{Hits: []o2.Hit{
		{"service_name": "order-service", "erreurs": json.Number("3")},
		{"service_name": "inventory-service", "erreurs": json.Number("12")},
	}}, nil)

	out, err := run(t, o2Deps(searcher, credentialed), "o2", "query", "SELECT 1")

	require.NoError(t, err)
	assert.Equal(t, "erreurs  service_name\n3        order-service\n12       inventory-service\n", out)
}

func TestO2Query_WithoutSQL_ReturnsUsageError(t *testing.T) {
	t.Parallel()

	searcher := mocks.NewLogSearcher(t)

	_, err := run(t, o2Deps(searcher, credentialed), "o2", "query")

	require.Error(t, err)
}

type followFixture struct {
	follower *follower
	out      *strings.Builder
}

func newFollowFixture(searcher LogSearcher, limit, pageSize int) followFixture {
	out := &strings.Builder{}
	printer := logPrinter{w: out}
	f := newFollower(searcher, logFilter{}, limit, time.Millisecond, func() time.Time { return fixedNow },
		printer.print)
	f.pageSize = pageSize
	return followFixture{follower: f, out: out}
}

func TestFollowerEmit_PropagatesOutputError(t *testing.T) {
	t.Parallel()

	hit := hitAt(fixedNow, "message")
	writeErr := errors.New("output unavailable")
	flw := newFollower(nil, logFilter{}, 1, time.Millisecond, func() time.Time { return fixedNow },
		func(got o2.Hit) error {
			assert.Equal(t, hit, got)
			return writeErr
		})

	err := flw.emit([]o2.Hit{hit})

	require.ErrorIs(t, err, writeErr)
}

func lines(out string) []string {
	if out == "" {
		return nil
	}
	return splitLines(out)
}

func TestFollowerPoll_WhenOverlapIsFullOfAlreadySeenLines_ReadsNextPagesUntilTheNewLine(t *testing.T) {
	t.Parallel()

	base := fixedNow.Add(-30 * time.Second)
	seen := []o2.Hit{hitAt(base, "a"), hitAt(base.Add(time.Second), "b"), hitAt(base.Add(2*time.Second), "c")}
	fresh := hitAt(base.Add(3*time.Second), "nouvelle")
	ascending := (logFilter{}).sql(orderOldestFirst)
	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, mock.MatchedBy(func(r o2.SearchRequest) bool { return r.SQL != ascending })).
		Return(o2.SearchResult{Hits: []o2.Hit{seen[2], seen[1], seen[0]}}, nil).Once()
	searcher.EXPECT().Search(mock.Anything, mock.MatchedBy(func(r o2.SearchRequest) bool {
		return r.SQL == ascending && r.From == 0 && r.Size == 3
	})).Return(o2.SearchResult{Hits: seen}, nil).Once()
	searcher.EXPECT().Search(mock.Anything, mock.MatchedBy(func(r o2.SearchRequest) bool {
		return r.SQL == ascending && r.From == 3 && r.Size == 3
	})).Return(o2.SearchResult{Hits: []o2.Hit{fresh}}, nil).Once()
	fixture := newFollowFixture(searcher, 3, 3)
	require.NoError(t, fixture.follower.initialBatch(t.Context(), base, fixedNow))

	require.NoError(t, fixture.follower.poll(t.Context()))

	assert.Equal(t, []string{"a", "b", "c", "nouvelle"}, bodiesOf(fixture.out.String()))
}

func TestFollowerPoll_WhenLimitDroppedOlderLinesOfTheInitialWindow_NeverShowsThemAfterwards(t *testing.T) {
	t.Parallel()

	base := fixedNow.Add(-30 * time.Second)
	all := []o2.Hit{
		hitAt(base, "ancienne-1"), hitAt(base.Add(time.Second), "ancienne-2"),
		hitAt(base.Add(2*time.Second), "initiale-1"), hitAt(base.Add(3*time.Second), "initiale-2"),
	}
	fresh := hitAt(base.Add(4*time.Second), "nouvelle")

	var pollStart time.Time
	searcher := mocks.NewLogSearcher(t)
	searcher.EXPECT().Search(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, req o2.SearchRequest) (o2.SearchResult, error) {
			if req.SQL != (logFilter{}).sql(orderOldestFirst) {
				return o2.SearchResult{Hits: []o2.Hit{all[3], all[2]}}, nil
			}
			pollStart = req.Start
			var hits []o2.Hit
			for _, h := range append(all, fresh) {
				if ts, _ := timestampOf(h); ts >= req.Start.UnixMicro() {
					hits = append(hits, h)
				}
			}
			return o2.SearchResult{Hits: hits}, nil

		})
	fixture := newFollowFixture(searcher, 2, 1000)
	require.NoError(t, fixture.follower.initialBatch(t.Context(), base, fixedNow))

	require.NoError(t, fixture.follower.poll(t.Context()))

	assert.Equal(t, []string{"initiale-1", "initiale-2", "nouvelle"}, bodiesOf(fixture.out.String()))
	assert.Equal(t, base.Add(2*time.Second).UnixMicro(), pollStart.UnixMicro())
}

func bodiesOf(out string) []string {
	var bodies []string
	for _, line := range lines(out) {
		_, body, _ := strings.Cut(line, "  ")
		bodies = append(bodies, body)
	}
	return bodies
}
