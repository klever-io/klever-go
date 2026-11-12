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
	"unicode/utf8"
)

const (
	// maxErrorBody bounds how much of a non-2xx body goes into the error message. The
	// original 512-byte snippet cut the node's `{"error":"..."}` reason mid-string, which
	// is the one thing an operator needs from a failed call. 64 KiB covers any node error
	// and a 500 with a stack trace, while still keeping a hostile server from flooding
	// the terminal; anything past it is dropped and the message says so.
	maxErrorBody    = 64 << 10
	maxResponseBody = 32 << 20
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

// HTTPStatusError is the error for a non-2xx response. The operator prints Error(); a
// caller that logs it (the node's websocket mirror) can take the fields apart and bound
// each on its own, so a body the server wrote cannot imitate the attribution or push it
// past the log's bound.
type HTTPStatusError struct {
	Method     string
	URL        string
	StatusCode int
	Status     string // from the code alone, e.g. "502 Bad Gateway"
	Body       string // sanitised, bounded at maxErrorBody, with a marker when cut
	ReadErr    string // sanitised read failure; empty when the body was read whole
}

// Error renders the message the operator reads: method and URL first, so the attribution
// is never something the body wrote, then the status, the body and any read failure.
func (e *HTTPStatusError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s: unexpected HTTP status %s", e.Method, e.URL, e.Status)
	if e.Body != "" {
		b.WriteString(": ")
		b.WriteString(e.Body)
	}
	if e.ReadErr != "" {
		fmt.Fprintf(&b, " (failed to read response body: %s)", e.ReadErr)
	}
	return b.String()
}

// checkStatus returns nil for a 2xx response. For anything else it returns an
// *HTTPStatusError with the status and the response body (sanitised, bounded at
// maxErrorBody, with a marker when cut), so the node's reason is not lost.
func checkStatus(method, url string, r *http.Response) error {
	if r.StatusCode >= http.StatusOK && r.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	// The status comes from the code alone: r.Status carries the server's own reason
	// phrase, unbounded and chosen by the server. TrimSpace covers a code net/http has no
	// phrase for.
	statusErr := &HTTPStatusError{
		Method:     method,
		URL:        url,
		StatusCode: r.StatusCode,
		Status:     strings.TrimSpace(fmt.Sprintf("%d %s", r.StatusCode, http.StatusText(r.StatusCode))),
	}
	// io.ReadAll returns the bytes it did manage to read alongside a non-nil error on a
	// truncated/hijacked body (e.g. a declared Content-Length the server never delivered) —
	// keep that partial snippet instead of discarding it. readErr's own message can embed
	// server-supplied text (net/http's trailer parsing may fold a server header into it), so
	// sanitize it the same as status/body rather than let it skip the same treatment.
	// Read one byte past the cap so a body that is exactly at it is not reported as cut.
	snippet, readErr := io.ReadAll(io.LimitReader(r.Body, maxErrorBody+1))
	truncated := len(snippet) > maxErrorBody
	if truncated {
		// A multi-byte rune the cut splits is invalid UTF-8, and sanitizeServerText drops
		// invalid bytes, so no U+FFFD precedes the marker.
		snippet = snippet[:maxErrorBody]
	}
	statusErr.Body = sanitizeServerText(string(snippet))
	if truncated {
		marker := fmt.Sprintf("(response body truncated at %d bytes)", maxErrorBody)
		if statusErr.Body == "" {
			statusErr.Body = marker
		} else {
			statusErr.Body += " ... " + marker
		}
	}
	if readErr != nil {
		statusErr.ReadErr = sanitizeServerText(readErr.Error())
	}

	return statusErr
}

// sanitizeServerText makes server-supplied text safe to print as one terminal line, and
// keeps it bounded. Every whitespace run — newlines of a stack trace, tabs, the Unicode
// line and paragraph separators — becomes one space, so the lines stay readable and
// padding cannot stretch the text. The other control characters (escape sequences) and
// the format characters (bidi overrides, zero-width joiners, which reorder or hide what
// follows — a reason that reads backwards could disguise a fee or an address) are dropped.
// So is every invalid UTF-8 byte: rendering each one as the 3-byte U+FFFD would triple a
// body that was capped before sanitising. A U+FFFD the server itself sent is kept.
// One pass, one allocation: this runs on every failed mirror post on the node, so no
// intermediate copy or per-word slice of a 64 KiB body.
func sanitizeServerText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false
	for i := 0; i < len(s); {
		// An invalid byte decodes as utf8.RuneError with width 1; a genuine U+FFFD in the
		// body (the node itself writes one when it echoes bad input bytes through JSON) has
		// width 3 and is kept, since it is the one hint that bad bytes were in the input.
		r, width := utf8.DecodeRuneInString(s[i:])
		i += width
		switch {
		case unicode.IsSpace(r):
			pendingSpace = b.Len() > 0 // leading whitespace is dropped, trailing never flushed
		case r == utf8.RuneError && width == 1, unicode.IsControl(r), unicode.Is(unicode.Cf, r):
		default:
			if pendingSpace {
				b.WriteByte(' ')
				pendingSpace = false
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
