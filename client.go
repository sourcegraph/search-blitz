package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	envToken    = "SOURCEGRAPH_TOKEN"
	envEndpoint = "SOURCEGRAPH_ENDPOINT"
)

type client struct {
	token    string
	endpoint string
	client   *http.Client
}

func newClient() (*client, error) {
	tkn := os.Getenv(envToken)
	if tkn == "" {
		return nil, fmt.Errorf("%s not set", envToken)
	}
	endpoint := os.Getenv(envEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("%s not set", envEndpoint)
	}

	return &client{
		token:    tkn,
		endpoint: endpoint,
		client:   http.DefaultClient,
	}, nil
}

func (s *client) searchBatch(ctx context.Context, qc QueryConfig) (*result, *metrics, error) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(map[string]interface{}{
		"query":     graphQLQuery,
		"variables": map[string]string{"query": qc.Query},
	}); err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.batchURL(), io.NopCloser(&body))
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Authorization", "token "+s.token)
	req.Header.Set("X-Sourcegraph-Should-Trace", "true")
	req.Header.Set("User-Agent", fmt.Sprintf("SearchBlitz (%s)", qc.Name))

	start := time.Now()
	resp, err := s.client.Do(req)
	m := &metrics{}
	m.took = time.Since(start).Milliseconds()

	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		break
	default:
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	m.trace = resp.Header.Get("x-trace")

	// Decode the response.
	respDec := rawResult{Data: result{}}
	if err := json.NewDecoder(resp.Body).Decode(&respDec); err != nil {
		return nil, nil, err
	}
	return &respDec.Data, m, nil
}

func (s *client) batchURL() string {
	return s.endpoint + "/.api/graphql?SearchBlitz"
}

type streamResults []interface{}

func (s *client) searchStream(ctx context.Context, qc QueryConfig) ([]streamResults, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.streamURL(qc.Query), nil)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "token "+s.token)
	req.Header.Set("X-Sourcegraph-Should-Trace", "true")
	req.Header.Set("User-Agent", fmt.Sprintf("SearchBlitz (%s)", qc.Name))

	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		break
	default:
		return nil, 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var matchesEvents []streamResults
	scanner := NewSSEScanner(resp.Body)
	for scanner.Scan() {
		event := scanner.Event()
		switch event.Type {
		case "matches":
			var s streamResults
			if err := json.Unmarshal([]byte(event.Data), &s); err != nil {
				return nil, 0, err
			}

			matchesEvents = append(matchesEvents, s)
		}
	}

	return matchesEvents, time.Since(start), scanner.Err()
}

func (s *client) streamURL(query string) string {
	return s.endpoint + "/.api/search/stream?SearchBlitz&q=" + url.QueryEscape(query)
}
