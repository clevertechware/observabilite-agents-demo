package o2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchSendsBasicAuthAndMicrosecondWindowToOrgEndpoint(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, token, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "ctwctl@sa.internal", user)
		assert.Equal(t, "jeton-de-test", token)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/default/_search", r.URL.Path)
		assert.Equal(t, "logs", r.URL.Query().Get("type"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body searchBody
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, start.UnixMicro(), body.Query.StartTime)
		assert.Equal(t, end.UnixMicro(), body.Query.EndTime)
		assert.Equal(t, `SELECT * FROM "default"`, body.Query.SQL)
		assert.Equal(t, 100, body.Query.Size)
		_, _ = w.Write([]byte(`{"hits":[],"total":0}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "default", "ctwctl@sa.internal", "jeton-de-test")
	_, err := client.Search(t.Context(), SearchRequest{SQL: `SELECT * FROM "default"`, Start: start, End: end})

	require.NoError(t, err)
}

func TestSearchDecodesHitsAndTotalAtTheRootOfTheResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"took":3,"hits":[{"_timestamp":1795167672000000,"body":"ok"}],"total":1,"from":0,"size":100}`))
	}))
	defer srv.Close()

	result, err := NewClient(srv.URL, "default", "u", "t").Search(t.Context(), SearchRequest{SQL: "SELECT 1"})

	require.NoError(t, err)
	require.Len(t, result.Hits, 1)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "ok", result.Hits[0]["body"])
	assert.Equal(t, json.Number("1795167672000000"), result.Hits[0]["_timestamp"])
}

func TestSearchDefaultsToTheLastHourWhenNoWindowIsGiven(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body searchBody
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, time.Hour.Microseconds(), body.Query.EndTime-body.Query.StartTime)
		_, _ = w.Write([]byte(`{"hits":[],"total":0}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "default", "u", "t").Search(t.Context(), SearchRequest{SQL: "SELECT 1"})

	require.NoError(t, err)
}

func TestSearchRetriesOn429And503ThenSucceeds(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	statuses := []int{http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusOK}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statuses[calls.Add(1)-1])
		if calls.Load() == 3 {
			_, _ = w.Write([]byte(`{"hits":[{"body":"ok"}],"total":1}`))
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "default", "ctwctl@sa.internal", "jeton-de-test", WithBackoff(0))
	result, err := client.Search(context.Background(), SearchRequest{SQL: `SELECT * FROM "default"`})

	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.EqualValues(t, 3, calls.Load())
}

func TestSearchGivesUpAfterThreeServerErrorsAndReportsTheStatus(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "default", "u", "t", WithBackoff(0)).Search(t.Context(), SearchRequest{SQL: "SELECT 1"})

	var statusErr *StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, http.StatusInternalServerError, statusErr.Status)
	assert.EqualValues(t, 3, calls.Load())
}

func TestSearchDoesNotRetryOnBadRequest(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"sql invalide"}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "default", "u", "t", WithBackoff(0)).Search(t.Context(), SearchRequest{SQL: "SELEC"})

	var statusErr *StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, http.StatusBadRequest, statusErr.Status)
	assert.Contains(t, statusErr.Body, "sql invalide")
	assert.EqualValues(t, 1, calls.Load())
}

func TestSearchStopsRetryingWhenTheContextIsCancelled(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	ctx, cancel := context.WithCancel(t.Context())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		cancel()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "default", "u", "t", WithBackoff(time.Hour)).Search(ctx, SearchRequest{SQL: "SELECT 1"})

	require.ErrorIs(t, err, context.Canceled)
	assert.EqualValues(t, 1, calls.Load())
}

func TestStreamsListsFieldNamesFromTheSchemaOfTheRequestedType(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/default/streams", r.URL.Path)
		assert.Equal(t, "logs", r.URL.Query().Get("type"))
		assert.Equal(t, "true", r.URL.Query().Get("fetchSchema"))
		_, _ = w.Write([]byte(`{"list":[
			{"name":"default","stream_type":"logs","schema":[{"name":"_timestamp","type":"Int64"},{"name":"body","type":"Utf8"}]},
			{"name":"triggers","stream_type":"logs"}]}`))
	}))
	defer srv.Close()

	streams, err := NewClient(srv.URL, "default", "u", "t").Streams(t.Context(), "logs")

	require.NoError(t, err)
	assert.Equal(t, []Stream{
		{Name: "default", Type: "logs", Fields: []string{"_timestamp", "body"}},
		{Name: "triggers", Type: "logs", Fields: []string{}},
	}, streams)
}
