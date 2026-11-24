package o2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxAttempts      = 3
	defaultBackoff   = 500 * time.Millisecond
	defaultPageSize  = 100
	defaultWindow    = time.Hour
	maxErrorBodySize = 512
)

type Client struct {
	baseURL string
	org     string
	user    string
	token   string
	http    *http.Client
	backoff time.Duration
}

type Option func(*Client)

func WithBackoff(d time.Duration) Option {
	return func(c *Client) { c.backoff = d }
}

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

func NewClient(baseURL, org, user, token string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		org:     org,
		user:    user,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
		backoff: defaultBackoff,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type StatusError struct {
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("openobserve a répondu %d", e.Status)
	}
	return fmt.Sprintf("openobserve a répondu %d : %s", e.Status, e.Body)
}

// SearchRequest décrit une recherche SQL. Une fenêtre vide couvre la dernière heure.
type SearchRequest struct {
	SQL        string
	Start      time.Time
	End        time.Time
	From       int
	Size       int
	StreamType string
}

type Hit map[string]any

type SearchResult struct {
	Hits  []Hit `json:"hits"`
	Total int   `json:"total"`
	From  int   `json:"from"`
	Size  int   `json:"size"`
}

type Stream struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Fields []string `json:"fields"`
}

type searchBody struct {
	Query searchQuery `json:"query"`
}

type searchQuery struct {
	SQL       string `json:"sql"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	From      int    `json:"from"`
	Size      int    `json:"size"`
}

func (c *Client) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	end := req.End
	if end.IsZero() {
		end = time.Now()
	}
	start := req.Start
	if start.IsZero() {
		start = end.Add(-defaultWindow)
	}
	size := req.Size
	if size <= 0 {
		size = defaultPageSize
	}
	streamType := req.StreamType
	if streamType == "" {
		streamType = "logs"
	}
	payload, err := json.Marshal(searchBody{Query: searchQuery{
		SQL:       req.SQL,
		StartTime: start.UnixMicro(),
		EndTime:   end.UnixMicro(),
		From:      req.From,
		Size:      size,
	}})
	if err != nil {
		return SearchResult{}, fmt.Errorf("encodage de la requête : %w", err)
	}

	path := fmt.Sprintf("/api/%s/_search?type=%s", url.PathEscape(c.org), url.QueryEscape(streamType))
	body, err := c.do(ctx, http.MethodPost, path, payload)
	if err != nil {
		return SearchResult{}, err
	}
	var result SearchResult
	if err = decodeJSON(body, &result); err != nil {
		return SearchResult{}, fmt.Errorf("décodage de la réponse de _search : %w", err)
	}
	return result, nil
}

type streamsResponse struct {
	List []struct {
		Name       string `json:"name"`
		StreamType string `json:"stream_type"`
		Schema     []struct {
			Name string `json:"name"`
		} `json:"schema"`
	} `json:"list"`
}

func (c *Client) Streams(ctx context.Context, streamType string) ([]Stream, error) {
	path := fmt.Sprintf("/api/%s/streams?type=%s&fetchSchema=true", url.PathEscape(c.org), url.QueryEscape(streamType))
	body, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response streamsResponse
	if err = decodeJSON(body, &response); err != nil {
		return nil, fmt.Errorf("décodage de la réponse de streams : %w", err)
	}
	streams := make([]Stream, 0, len(response.List))
	for _, item := range response.List {
		fields := make([]string, 0, len(item.Schema))
		for _, f := range item.Schema {
			fields = append(fields, f.Name)
		}
		streams = append(streams, Stream{Name: item.Name, Type: item.StreamType, Fields: fields})
	}
	return streams, nil
}

func (c *Client) do(ctx context.Context, method, path string, payload []byte) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if err := sleep(ctx, c.backoff<<(attempt-1)); err != nil {
				return nil, err
			}
		}
		body, retryable, err := c.once(ctx, method, path, payload)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) once(ctx context.Context, method, path string, payload []byte) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, false, fmt.Errorf("construction de la requête : %w", err)
	}
	req.SetBasicAuth(c.user, c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, ctx.Err() == nil, fmt.Errorf("appel à openobserve : %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ctx.Err() == nil, fmt.Errorf("lecture de la réponse : %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, false, nil
	}
	retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
	return nil, retryable, &StatusError{Status: resp.StatusCode, Body: truncate(string(body))}
}

func decodeJSON(data []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(target)
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxErrorBodySize {
		return s[:maxErrorBodySize] + "…"
	}
	return s
}
