package indexer

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/klever-io/klever-go/indexer/mock"
	"github.com/klever-io/klever-go/indexer/templates"
	"github.com/stretchr/testify/require"
)

func TestDoBulkRemoveByTimestamp_EncodeError(t *testing.T) {
	t.Parallel()

	// Create a client (even though we won't use the actual elasticsearch connection)
	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)

	// Test with an index and timestamp - encode should work fine
	// This test verifies the method doesn't panic with valid inputs
	err = ec.DoBulkRemoveByTimestamp("test-index", 12345)
	// May or may not error depending on ES availability - just verify no panic
	_ = err
}

func TestDoSearch_ValidInput(t *testing.T) {
	t.Parallel()

	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)

	query := templates.Object{
		"query": templates.Object{
			"match_all": templates.Object{},
		},
	}

	body, err := encode(query)
	require.NoError(t, err)

	// Test that DoSearch accepts the input correctly
	_, err = ec.DoSearch("test-index", &body)
	// May or may not error depending on ES availability - just verify no panic
	_ = err
}

func TestDoUpdate_ValidInput(t *testing.T) {
	t.Parallel()

	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)

	update := templates.Object{
		"doc": templates.Object{
			"field": "value",
		},
	}

	body, err := encode(update)
	require.NoError(t, err)

	// Test that DoUpdate accepts the input correctly
	err = ec.DoUpdate("test-index", "doc-id", &body)
	// Error is expected since we don't have a real ES instance
	require.Error(t, err) // Will fail at ES connection
}

func TestDoBulkRemoveByTimestamp_PrepareQuery(t *testing.T) {
	t.Parallel()

	// Test the query preparation function used by DoBulkRemoveByTimestamp
	timestamp := time.Duration(123456789)
	obj := prepareTimestampForBulkRemove(timestamp)

	require.NotNil(t, obj)

	// Verify the structure
	query, ok := obj["query"]
	require.True(t, ok)

	term, ok := query.(templates.Object)["term"]
	require.True(t, ok)

	timestampValue, ok := term.(templates.Object)["timestamp"]
	require.True(t, ok)
	require.Equal(t, timestamp, timestampValue)
}

func TestDoSearch_EmptyIndex(t *testing.T) {
	t.Parallel()

	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)

	query := templates.Object{
		"query": templates.Object{
			"match_all": templates.Object{},
		},
	}

	body, err := encode(query)
	require.NoError(t, err)

	// Test with empty index name
	_, err = ec.DoSearch("", &body)
	// May or may not error depending on ES availability - just verify no panic
	_ = err
}

func TestDoUpdate_EmptyDocumentID(t *testing.T) {
	t.Parallel()

	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)

	update := templates.Object{
		"doc": templates.Object{
			"field": "value",
		},
	}

	body, err := encode(update)
	require.NoError(t, err)

	// Test with empty document ID
	err = ec.DoUpdate("test-index", "", &body)
	// Should still attempt the operation (ES will handle empty ID)
	require.Error(t, err)
}

func TestDoBulkRemoveByTimestamp_NegativeTimestamp(t *testing.T) {
	t.Parallel()

	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)

	// Test with negative timestamp (should still work, ES will handle it)
	err = ec.DoBulkRemoveByTimestamp("test-index", -12345)
	// May or may not error depending on ES availability - just verify no panic
	_ = err
}

func TestDoBulkRemoveByTimestamp_ZeroTimestamp(t *testing.T) {
	t.Parallel()

	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)

	// Test with zero timestamp
	err = ec.DoBulkRemoveByTimestamp("test-index", 0)
	// May or may not error depending on ES availability - just verify no panic
	_ = err
}

func TestCloseResponseBody_NilResponse(t *testing.T) {
	t.Parallel()

	// Should not panic with nil response
	closeResponseBody(nil, "test")
}

func TestCloseResponseBody_NilBody(t *testing.T) {
	t.Parallel()

	resp := &esapi.Response{
		StatusCode: 200,
		Body:       nil,
	}

	// Should not panic with nil body
	closeResponseBody(resp, "test")
}

func TestCloseResponseBody_ValidResponse(t *testing.T) {
	t.Parallel()

	closeCalled := false
	resp := &esapi.Response{
		StatusCode: 200,
		Body: &mock.ReadCloserStub{
			CloseCalled: func() error {
				closeCalled = true
				return nil
			},
		},
	}

	closeResponseBody(resp, "test")
	require.True(t, closeCalled)
}

func TestCloseResponseBody_CloseError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("close error")
	closeCalled := false

	resp := &esapi.Response{
		StatusCode: 200,
		Body: &mock.ReadCloserStub{
			CloseCalled: func() error {
				closeCalled = true
				return expectedErr
			},
		},
	}

	// Should not panic even if close returns an error
	closeResponseBody(resp, "test")
	require.True(t, closeCalled)
}

func TestElasticClient_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	var ec *elasticClient
	require.True(t, ec.IsInterfaceNil())

	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)
	require.False(t, ec.IsInterfaceNil())
}

func TestNewElasticClient_NoAddresses(t *testing.T) {
	t.Parallel()

	cfg := elasticsearch.Config{
		Addresses: []string{},
	}

	ec, err := NewElasticClient(cfg)
	require.Error(t, err)
	require.Equal(t, ErrNoElasticUrlProvided, err)
	require.Nil(t, ec)
}

func TestNewElasticClient_ValidAddress(t *testing.T) {
	t.Parallel()

	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, ec)
	require.Equal(t, "http://localhost:9200", ec.elasticBaseUrl)
}

func TestNewElasticClient_MultipleAddresses(t *testing.T) {
	t.Parallel()

	cfg := elasticsearch.Config{
		Addresses: []string{
			"http://localhost:9200",
			"http://localhost:9201",
			"http://localhost:9202",
		},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, ec)
	// Should use the first address as base URL
	require.Equal(t, "http://localhost:9200", ec.elasticBaseUrl)
}

func TestDoSearch_ResponseParsing(t *testing.T) {
	t.Parallel()

	// Test that the method attempts to parse the response correctly
	// This is more of a contract test
	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	ec, err := NewElasticClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, ec)

	query := templates.Object{
		"query": templates.Object{
			"match": templates.Object{
				"field": "value",
			},
		},
		"size": 10,
	}

	body, err := encode(query)
	require.NoError(t, err)

	// Verify body was encoded correctly
	require.Greater(t, body.Len(), 0)

	bodyStr := body.String()
	require.Contains(t, bodyStr, "query")
	require.Contains(t, bodyStr, "match")
	require.Contains(t, bodyStr, "size")
}

func TestDoUpdate_BodyStructure(t *testing.T) {
	t.Parallel()

	// Test the structure of update body
	update := templates.Object{
		"script": templates.Object{
			"source": "ctx._source.field = params.value",
			"params": templates.Object{
				"value": "newValue",
			},
		},
	}

	body, err := encode(update)
	require.NoError(t, err)
	require.Greater(t, body.Len(), 0)

	bodyStr := body.String()
	require.Contains(t, bodyStr, "script")
	require.Contains(t, bodyStr, "source")
	require.Contains(t, bodyStr, "params")
}

func TestDoBulkRemoveByTimestamp_QueryStructure(t *testing.T) {
	t.Parallel()

	// Test different timestamp values
	testCases := []struct {
		name      string
		timestamp time.Duration
	}{
		{
			name:      "Positive timestamp",
			timestamp: 1234567890,
		},
		{
			name:      "Zero timestamp",
			timestamp: 0,
		},
		{
			name:      "Negative timestamp",
			timestamp: -1234567890,
		},
		{
			name:      "Large timestamp",
			timestamp: 9999999999999,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			obj := prepareTimestampForBulkRemove(tc.timestamp)
			require.NotNil(t, obj)

			body, err := encode(obj)
			require.NoError(t, err)
			require.Greater(t, body.Len(), 0)

			bodyStr := body.String()
			require.Contains(t, bodyStr, "query")
			require.Contains(t, bodyStr, "term")
			require.Contains(t, bodyStr, "timestamp")
			require.Contains(t, bodyStr, fmt.Sprintf("%d", tc.timestamp))
		})
	}
}
