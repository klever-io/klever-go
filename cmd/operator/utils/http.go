package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const (
	maxErrorBodySnippet = 512
	maxResponseBody     = 32 << 20
)

var httpClient *http.Client

func init() {
	httpClient = &http.Client{Timeout: 10 * time.Second}
}

// GetURL provides json result decode to struct
func GetURL(url string, target interface{}) error {
	r, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = r.Body.Close() }()

	if err := checkStatus(http.MethodGet, url, r); err != nil {
		return err
	}

	return json.NewDecoder(io.LimitReader(r.Body, maxResponseBody)).Decode(target)
}

// PostURL provides a post using a json string
func PostURL(url, body string, headers []string, target interface{}) error {
	return PostURLWithContext(context.Background(), url, body, headers, target)
}

// PostURLWithContext behaves like PostURL but ties the request to ctx, so a caller can
// abort an in-flight POST (e.g. on shutdown) instead of waiting out httpClient's fixed
// timeout.
func PostURLWithContext(ctx context.Context, url, body string, headers []string, target interface{}) error {
	reqBody := strings.NewReader(body)
	req, errNewReq := http.NewRequestWithContext(ctx, http.MethodPost, url, reqBody)
	if errNewReq != nil {
		return fmt.Errorf("create POST request: %w", errNewReq)
	}
	req.Header.Add("Content-type", "application/json; charset=UTF-8")
	for i := 0; i < len(headers); i += 2 {
		req.Header.Add(headers[i], headers[i+1])
	}

	r, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send POST request: %w", err)
	}
	defer func() { _ = r.Body.Close() }()

	if err := checkStatus(http.MethodPost, url, r); err != nil {
		return err
	}

	if target != nil {
		data, errRead := io.ReadAll(io.LimitReader(r.Body, maxResponseBody+1))
		if errRead != nil {
			return fmt.Errorf("read POST response body: %w", errRead)
		}
		if int64(len(data)) > maxResponseBody {
			return fmt.Errorf("%s %s: response body exceeds %d bytes", http.MethodPost, url, maxResponseBody)
		}

		if err := json.Unmarshal(data, &target); err != nil {
			return err
		}
	}
	return nil
}

func checkStatus(method, url string, r *http.Response) error {
	if r.StatusCode >= http.StatusOK && r.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	status := sanitizeServerText(r.Status)
	snippet, readErr := io.ReadAll(io.LimitReader(r.Body, maxErrorBodySnippet))
	if readErr != nil {
		return fmt.Errorf("%s %s: unexpected HTTP status %s (failed to read response body: %w)", method, url, status, readErr)
	}
	body := strings.TrimSpace(sanitizeServerText(string(snippet)))
	if body == "" {
		return fmt.Errorf("%s %s: unexpected HTTP status %s", method, url, status)
	}

	return fmt.Errorf("%s %s: unexpected HTTP status %s: %s", method, url, status, body)
}

func sanitizeServerText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
