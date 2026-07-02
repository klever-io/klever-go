package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxErrorBodySnippet = 512

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

	return json.NewDecoder(r.Body).Decode(target)
}

// PostURL provides a post using a json string
func PostURL(url, body string, headers []string, target interface{}) error {
	reqBody := strings.NewReader(body)
	req, errNewReq := http.NewRequest(http.MethodPost, url, reqBody)
	if errNewReq != nil {
		return errNewReq
	}
	req.Header.Add("Content-type", "application/json; charset=UTF-8")
	for i := 0; i < len(headers); i += 2 {
		req.Header.Add(headers[i], headers[i+1])
	}

	r, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = r.Body.Close() }()

	if err := checkStatus(http.MethodPost, url, r); err != nil {
		return err
	}

	if target != nil {
		data, errRead := io.ReadAll(r.Body)
		if errRead != nil {
			return errRead
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

	snippet, _ := io.ReadAll(io.LimitReader(r.Body, maxErrorBodySnippet))
	body := strings.TrimSpace(string(snippet))
	if body == "" {
		return fmt.Errorf("%s %s: unexpected HTTP status %s", method, url, r.Status)
	}

	return fmt.Errorf("%s %s: unexpected HTTP status %s: %s", method, url, r.Status, body)
}
