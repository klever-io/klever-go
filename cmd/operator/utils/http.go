package utils

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
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
