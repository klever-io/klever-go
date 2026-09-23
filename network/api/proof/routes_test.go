package proof

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type stubFacade struct {
	getProof  func(address string) (*state.MerkleProof, error)
	getAtRoot func(rootHash []byte, address string) (*state.MerkleProof, error)
	verify    func(rootHash []byte, address string, proof [][]byte) (bool, error)
}

func (s *stubFacade) GetProof(address string) (*state.MerkleProof, error) {
	return s.getProof(address)
}

func (s *stubFacade) GetProofForRootHash(rootHash []byte, address string) (*state.MerkleProof, error) {
	return s.getAtRoot(rootHash, address)
}

func (s *stubFacade) VerifyProof(rootHash []byte, address string, proof [][]byte) (bool, error) {
	return s.verify(rootHash, address, proof)
}

func (s *stubFacade) IsInterfaceNil() bool { return s == nil }

func proofRoutesConfig(open, secured bool) config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			"proof": {
				Routes: []config.RouteConfig{
					{Name: getProofPath, Open: open, Secured: secured},
					{Name: getProofByRootPath, Open: open, Secured: secured},
					{Name: verifyProofPath, Open: open, Secured: secured},
				},
			},
		},
		Credentials: []config.Credential{{Username: "u", Password: "p"}},
		Hasher:      config.TypeConfig{Type: "sha256"},
	}
}

func newProofEngine(t *testing.T, cfg config.APIRoutesConfig, facade FacadeHandler) *gin.Engine {
	t.Helper()

	ws := gin.New()
	if facade != nil {
		ws.Use(middleware.WithFacade(facade))
	}
	group := ws.Group("/proof")
	router, err := wrapper.NewRouterWrapper("proof", group, cfg, middleware.NewAuthenticationFunc(cfg))
	require.NoError(t, err)
	Routes(router)
	return ws
}

func decodeBody(t *testing.T, resp *httptest.ResponseRecorder) shared.GenericAPIResponse {
	t.Helper()
	var body shared.GenericAPIResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	return body
}

func TestGetProof_ReturnsHexProof(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(true, false), &stubFacade{
		getProof: func(address string) (*state.MerkleProof, error) {
			require.Equal(t, "klv1addr", address)
			return &state.MerkleProof{
				RootHash: []byte{0xab},
				Value:    []byte{0x01, 0x02},
				Proof:    [][]byte{{0x0a}, {0x0b, 0x0c}},
			}, nil
		},
	})

	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/proof/address/klv1addr", nil))
	require.Equal(t, http.StatusOK, resp.Code)

	body := decodeBody(t, resp)
	raw, err := json.Marshal(body.Data)
	require.NoError(t, err)
	var proof ProofResponse
	require.NoError(t, json.Unmarshal(raw, &proof))
	require.Equal(t, "ab", proof.RootHash)
	require.Equal(t, "0102", proof.Value)
	require.Equal(t, []string{"0a", "0b0c"}, proof.Proof)
}

func TestGetProofForRootHash_UnavailableRootIsAClearError(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(true, false), &stubFacade{
		getAtRoot: func(rootHash []byte, address string) (*state.MerkleProof, error) {
			require.Equal(t, []byte{0xaa, 0xbb}, rootHash)
			require.Equal(t, "klv1addr", address)
			return nil, state.ErrStateRootUnavailable
		},
	})

	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/proof/root-hash/aabb/address/klv1addr", nil))
	require.Equal(t, http.StatusBadRequest, resp.Code)
	body := decodeBody(t, resp)
	require.Contains(t, body.Error, state.ErrStateRootUnavailable.Error())
	require.Equal(t, shared.ReturnCodeRequestError, body.Code)
}

func TestGetProof_MissingAccount(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(true, false), &stubFacade{
		getProof: func(string) (*state.MerkleProof, error) {
			return nil, common.ErrAccNotFound
		},
	})

	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/proof/address/klv1missing", nil))
	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Contains(t, decodeBody(t, resp).Error, common.ErrAccNotFound.Error())
}

func TestGetProofForRootHash_BadHex(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(true, false), &stubFacade{})
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/proof/root-hash/zz/address/klv1addr", nil))
	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Contains(t, decodeBody(t, resp).Error, state.ErrInvalidProofRequest.Error())
}

func TestVerifyProof_BooleanResult(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(true, false), &stubFacade{
		verify: func(rootHash []byte, address string, proof [][]byte) (bool, error) {
			require.Equal(t, []byte{0x11}, rootHash)
			require.Equal(t, "klv1addr", address)
			require.Equal(t, [][]byte{{0x22}}, proof)
			return true, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/proof/verify", bytes.NewBufferString(
		`{"rootHash":"11","address":"klv1addr","proof":["22"]}`,
	))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	body := decodeBody(t, resp)
	raw, err := json.Marshal(body.Data)
	require.NoError(t, err)
	var verified VerifyResponse
	require.NoError(t, json.Unmarshal(raw, &verified))
	require.True(t, verified.Ok)
}

func TestVerifyProof_BadJSON(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(true, false), &stubFacade{})
	req := httptest.NewRequest(http.MethodPost, "/proof/verify", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestProofRoutes_ClosedRouteIsNotRegistered(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(false, true), &stubFacade{})
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/proof/address/klv1addr", nil))
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestProofRoutes_SecuredRejectsUnauthenticated(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(true, true), &stubFacade{
		getProof: func(string) (*state.MerkleProof, error) {
			t.Fatal("auth must reject the request before the handler")
			return nil, nil
		},
	})
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/proof/address/klv1addr", nil))
	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestProofRoutes_MissingOrWrongFacade(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(true, false), nil)
	for _, path := range []string{"/proof/address/klv1addr", "/proof/root-hash/aa/address/klv1addr"} {
		resp := httptest.NewRecorder()
		ws.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusInternalServerError, resp.Code)
	}
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/proof/verify", nil))
	require.Equal(t, http.StatusInternalServerError, resp.Code)

	wrong := gin.New()
	wrong.Use(func(c *gin.Context) { c.Set("facade", "not a facade") })
	cfg := proofRoutesConfig(true, false)
	router, err := wrapper.NewRouterWrapper("proof", wrong.Group("/proof"), cfg, middleware.NewAuthenticationFunc(cfg))
	require.NoError(t, err)
	Routes(router)
	resp = httptest.NewRecorder()
	wrong.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/proof/address/klv1addr", nil))
	require.Equal(t, http.StatusInternalServerError, resp.Code)
	require.Contains(t, decodeBody(t, resp).Error, "invalid app context")
}

func TestGetProofForRootHash_Success(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(true, false), &stubFacade{
		getAtRoot: func([]byte, string) (*state.MerkleProof, error) {
			return &state.MerkleProof{RootHash: []byte{0xaa}, Value: []byte{0x01}, Proof: [][]byte{{0x02}}}, nil
		},
	})
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/proof/root-hash/aa/address/klv1addr", nil))
	require.Equal(t, http.StatusOK, resp.Code)
}

func TestVerifyProof_RejectsBadInput(t *testing.T) {
	t.Parallel()

	ws := newProofEngine(t, proofRoutesConfig(true, false), &stubFacade{
		verify: func([]byte, string, [][]byte) (bool, error) {
			return false, errors.New("boom")
		},
	})

	cases := map[string]struct {
		body   string
		status int
	}{
		"bad root hex":   {`{"rootHash":"zz","address":"a","proof":["22"]}`, http.StatusBadRequest},
		"empty root":     {`{"rootHash":"","address":"a","proof":["22"]}`, http.StatusBadRequest},
		"empty proof":    {`{"rootHash":"11","address":"a","proof":[]}`, http.StatusBadRequest},
		"bad proof node": {`{"rootHash":"11","address":"a","proof":["zz"]}`, http.StatusBadRequest},
		"facade failure": {`{"rootHash":"11","address":"a","proof":["22"]}`, http.StatusInternalServerError},
	}
	for name, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/proof/verify", bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		ws.ServeHTTP(resp, req)
		require.Equal(t, tc.status, resp.Code, name)
	}
}
