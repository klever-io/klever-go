package main

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

const validHash = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

func makeMSServer(t *testing.T, method, path string, status int, body interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, method, r.Method)
		assert.Equal(t, path, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

// --- getMSApiTransaction ---

func TestGetMSApiTransaction_InvalidHashLength(t *testing.T) {
	_, err := getMSApiTransaction("0xdeadbeef")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid TX hash length")
}

func TestGetMSApiTransaction_InvalidHex(t *testing.T) {
	_, err := getMSApiTransaction("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid TX hash")
}

func TestGetMSApiTransaction_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// EOF — close without writing body
	}))
	defer srv.Close()
	multisignAPI = srv.URL

	_, err := getMSApiTransaction(validHash)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in multisign API")
}

func TestGetMSApiTransaction_Success(t *testing.T) {
	want := MSApiTransaction{Hash: validHash}
	srv := makeMSServer(t, http.MethodGet, "/transaction/"+validHash, http.StatusOK, want)
	defer srv.Close()
	multisignAPI = srv.URL

	got, err := getMSApiTransaction(validHash)
	require.NoError(t, err)
	assert.Equal(t, validHash, got.Hash)
}

func TestGetMSApiTransaction_StripPrefix(t *testing.T) {
	want := MSApiTransaction{Hash: validHash}
	srv := makeMSServer(t, http.MethodGet, "/transaction/"+validHash, http.StatusOK, want)
	defer srv.Close()
	multisignAPI = srv.URL

	got, err := getMSApiTransaction("0x" + validHash)
	require.NoError(t, err)
	assert.Equal(t, validHash, got.Hash)
}

// --- doSignAndPost ---

func TestDoSignAndPost_NotAuthorized(t *testing.T) {
	signerAddress = "addr_A"
	tx := &MSApiTransaction{
		Hash:    validHash,
		Signers: []MSApiSigner{{Address: "addr_B", Signed: false}},
		Raw:     &transaction.Transaction{},
	}
	err := doSignAndPost(tx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not required to sign")
}

func TestDoSignAndPost_AlreadySigned(t *testing.T) {
	signerAddress = "addr_A"

	tx := &MSApiTransaction{
		Hash:    validHash,
		Signers: []MSApiSigner{{Address: "addr_A", Signed: true}},
		Raw:     &transaction.Transaction{},
	}
	// Should return nil (already signed = no-op)
	err := doSignAndPost(tx)
	assert.NoError(t, err)
}

func TestDoSignAndPost_AutoSign_Success(t *testing.T) {
	autoSign = true
	signerAddress = "addr_A"
	_, rawPriv, _ := ed25519.GenerateKey(nil)

	privateKey = &mock.PrivateKeyMock{
		ScalarMock: func() crypto.Scalar {
			return &mock.ScalarMock{
				GetUnderlyingObjStub: func() interface{} {
					return rawPriv
				},
			}
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "error": ""})
	}))
	defer srv.Close()
	multisignAPI = srv.URL
	tx := &MSApiTransaction{
		Hash:    validHash,
		Signers: []MSApiSigner{{Address: "addr_A", Signed: false}},
		Raw: &transaction.Transaction{
			Signature: make([][]byte, 0),
			RawData:   &transaction.Transaction_Raw{},
		},
	}
	err := doSignAndPost(tx)
	assert.NoError(t, err)
}

// --- doPostMSTransactionSignature ---

func TestDoPostMSTransactionSignature_Success(t *testing.T) {
	srv := makeMSServer(t, http.MethodPost, "/transaction", http.StatusOK,
		map[string]string{"status": "ok", "error": ""})
	defer srv.Close()
	multisignAPI = srv.URL

	encoded := &MSApiEncoded{Hash: validHash, Address: "addr_A"}
	err := doPostMSTransactionSignature(encoded)
	assert.NoError(t, err)
}

func TestDoPostMSTransactionSignature_APIError(t *testing.T) {
	srv := makeMSServer(t, http.MethodPost, "/transaction", http.StatusOK,
		map[string]string{"status": "error", "error": "invalid signature"})
	defer srv.Close()
	multisignAPI = srv.URL

	encoded := &MSApiEncoded{Hash: validHash, Address: "addr_A"}
	err := doPostMSTransactionSignature(encoded)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestDoPostMSTransactionSignature_NetworkError(t *testing.T) {
	multisignAPI = "http://127.0.0.1:1" // nothing listening

	encoded := &MSApiEncoded{Hash: validHash, Address: "addr_A"}
	err := doPostMSTransactionSignature(encoded)
	assert.Error(t, err)
}
