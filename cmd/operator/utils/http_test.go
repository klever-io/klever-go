package utils_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sampleResult struct {
	Name string `json:"name"`
}

func TestGetURLDecodesSuccessBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"klever"}`))
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	assert.NoError(t, err)
	assert.Equal(t, "klever", got.Name)
}

func TestGetURLReturnsErrorOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"name":"boom"}`))
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "boom")
	assert.Empty(t, got.Name)
}

func TestGetURLReturnsErrorOnNotFoundStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestGetURLReturnsErrorOnMultipleChoicesStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultipleChoices)
		_, _ = w.Write([]byte("choices"))
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "300")
}

func TestGetURLSurfacesEOFOnEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	assert.True(t, errors.Is(err, io.EOF), "empty 2xx body must still surface io.EOF for not-found callers")
}

func TestGetURLTruncatesBodyInError(t *testing.T) {
	longBody := strings.Repeat("Z", 2000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(longBody))
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
	assert.Equal(t, 512, strings.Count(err.Error(), "Z"), "body snippet must be capped at 512 bytes")
}

func TestPostURLDecodesSuccessBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"klever"}`))
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.PostURL(srv.URL, `{}`, nil, &got)
	assert.NoError(t, err)
	assert.Equal(t, "klever", got.Name)
}

func TestPostURLReturnsErrorOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"name":"boom"}`))
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.PostURL(srv.URL, `{}`, nil, &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "boom")
	assert.Empty(t, got.Name)
}

func TestPostURLReturnsErrorOnEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.PostURL(srv.URL, `{}`, nil, &got)
	require.Error(t, err)
}

func TestPostURLNilTargetReturnsNilOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"ignored"}`))
	}))
	defer srv.Close()

	err := utils.PostURL(srv.URL, `{}`, nil, nil)
	assert.NoError(t, err)
}
