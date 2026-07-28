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
	// Draining before Close lets the transport reuse the underlying HTTP/1.x connection;
	// otherwise it's forced to redial (and re-handshake, for TLS) on the next request. This
	// matters most when target == nil (the caller doesn't want the body at all, e.g. the
	// websocket mirror at websocket.go — every 2xx response with a non-empty body would
	// otherwise pay for a fresh connection).
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, maxResponseBody))
		_ = r.Body.Close()
	}()

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
	// io.ReadAll returns the bytes it did manage to read alongside a non-nil error on a
	// truncated/hijacked body (e.g. a declared Content-Length the server never delivered) —
	// keep that partial snippet instead of discarding it. readErr's own message can embed
	// server-supplied text (net/http's trailer parsing may fold a server header into it), so
	// sanitize it the same as status/body rather than let it skip the same treatment.
	snippet, readErr := io.ReadAll(io.LimitReader(r.Body, maxErrorBodySnippet))
	body := strings.TrimSpace(sanitizeServerText(string(snippet)))
	if readErr != nil {
		sanitizedReadErr := sanitizeServerText(readErr.Error())
		if body == "" {
			return fmt.Errorf("%s %s: unexpected HTTP status %s (failed to read response body: %s)", method, url, status, sanitizedReadErr)
		}
		return fmt.Errorf("%s %s: unexpected HTTP status %s: %s (failed to read response body: %s)", method, url, status, body, sanitizedReadErr)
	}
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
