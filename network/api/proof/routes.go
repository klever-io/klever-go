package proof

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/state"
	apierrors "github.com/klever-io/klever-go/network/api/errors"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/network/api/wrapper"
)

const (
	getProofPath       = "/address/:address"
	getProofByRootPath = "/root-hash/:roothash/address/:address"
	verifyProofPath    = "/verify"
)

// FacadeHandler is the subset of the node facade used by the proof routes.
type FacadeHandler interface {
	GetProof(address string) (*state.MerkleProof, error)
	GetProofForRootHash(rootHash []byte, address string) (*state.MerkleProof, error)
	VerifyProof(rootHash []byte, address string, proof [][]byte) (bool, error)
	IsInterfaceNil() bool
}

// Routes registers the proof route group.
func Routes(router *wrapper.RouterWrapper) {
	router.RegisterHandler(http.MethodGet, getProofPath, GetProof)
	router.RegisterHandler(http.MethodGet, getProofByRootPath, GetProofForRootHash)
	router.RegisterHandler(http.MethodPost, verifyProofPath, VerifyProof)
}

func getFacade(c *gin.Context) (FacadeHandler, bool) {
	facadeObj, ok := c.Get("facade")
	if !ok {
		shared.RespondWith(
			c,
			http.StatusInternalServerError,
			nil,
			apierrors.ErrNilAppContext.Error(),
			shared.ReturnCodeInternalError,
		)
		return nil, false
	}

	facade, ok := facadeObj.(FacadeHandler)
	if !ok {
		shared.RespondWith(
			c,
			http.StatusInternalServerError,
			nil,
			apierrors.ErrInvalidAppContext.Error(),
			shared.ReturnCodeInternalError,
		)
		return nil, false
	}
	return facade, true
}

// GetProof returns a Merkle proof of the account under the current state root.
//
// @Summary Merkle proof of an account at the current state root
// @Tags Proof
// @Produce json
// @Param address path string true "bech32 address"
// @Success 200 {object} shared.GenericAPIResponse{data=ProofResponse} "ok"
// @Failure 400 {object} shared.GenericAPIResponse "bad address, missing account, or unavailable root"
// @Failure 500 {object} shared.GenericAPIResponse "internal error"
// @Router /proof/address/{address} [get]
func GetProof(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	proof, err := facade.GetProof(c.Param("address"))
	if err != nil {
		respondProofErr(c, apierrors.ErrGetProof, err)
		return
	}
	shared.RespondWith(c, http.StatusOK, encodeProof(proof), "", shared.ReturnCodeSuccess)
}

// GetProofForRootHash returns a Merkle proof of the account under a historical root.
// The node returns a clear error when that root is not in storage.
//
// @Summary Merkle proof of an account at a historical state root
// @Tags Proof
// @Produce json
// @Param roothash path string true "hex state root"
// @Param address path string true "bech32 address"
// @Success 200 {object} shared.GenericAPIResponse{data=ProofResponse} "ok"
// @Failure 400 {object} shared.GenericAPIResponse "bad request, missing account, or unavailable root"
// @Failure 500 {object} shared.GenericAPIResponse "internal error"
// @Router /proof/root-hash/{roothash}/address/{address} [get]
func GetProofForRootHash(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	rootHash, err := decodeHex(c.Param("roothash"))
	if err != nil {
		respondProofErr(c, apierrors.ErrGetProof, err)
		return
	}

	proof, err := facade.GetProofForRootHash(rootHash, c.Param("address"))
	if err != nil {
		respondProofErr(c, apierrors.ErrGetProof, err)
		return
	}
	shared.RespondWith(c, http.StatusOK, encodeProof(proof), "", shared.ReturnCodeSuccess)
}

// VerifyProof checks a submitted Merkle proof and returns a boolean.
//
// @Summary Verify a Merkle proof of an account
// @Tags Proof
// @Accept json
// @Produce json
// @Param proof body VerifyRequest true "root, address, and proof nodes"
// @Success 200 {object} shared.GenericAPIResponse{data=VerifyResponse} "ok"
// @Failure 400 {object} shared.GenericAPIResponse "bad request or unavailable root"
// @Failure 500 {object} shared.GenericAPIResponse "internal error"
// @Router /proof/verify [post]
func VerifyProof(c *gin.Context) {
	facade, ok := getFacade(c)
	if !ok {
		return
	}

	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondWithValidationError(c, fmt.Sprintf("%s: %s", apierrors.ErrValidation.Error(), err.Error()))
		return
	}

	rootHash, err := decodeHex(req.RootHash)
	if err != nil {
		respondProofErr(c, apierrors.ErrVerifyProof, err)
		return
	}
	nodes, err := decodeProof(req.Proof)
	if err != nil {
		respondProofErr(c, apierrors.ErrVerifyProof, err)
		return
	}

	verified, err := facade.VerifyProof(rootHash, req.Address, nodes)
	if err != nil {
		respondProofErr(c, apierrors.ErrVerifyProof, err)
		return
	}
	shared.RespondWith(c, http.StatusOK, VerifyResponse{Ok: verified}, "", shared.ReturnCodeSuccess)
}

func respondProofErr(c *gin.Context, base error, err error) {
	status := http.StatusInternalServerError
	code := shared.ReturnCodeInternalError
	switch {
	case errors.Is(err, state.ErrInvalidProofRequest),
		errors.Is(err, common.ErrNilAddress),
		errors.Is(err, common.ErrAccNotFound),
		errors.Is(err, state.ErrStateRootUnavailable):
		status = http.StatusBadRequest
		code = shared.ReturnCodeRequestError
	}
	shared.RespondWith(c, status, nil, apierrors.APIErrorString(base, err), code)
}

func encodeProof(proof *state.MerkleProof) ProofResponse {
	nodes := make([]string, len(proof.Proof))
	for i, node := range proof.Proof {
		nodes[i] = hex.EncodeToString(node)
	}
	return ProofResponse{
		Proof:    nodes,
		Value:    hex.EncodeToString(proof.Value),
		RootHash: hex.EncodeToString(proof.RootHash),
	}
}

func decodeHex(s string) ([]byte, error) {
	if s == "" {
		return nil, fmt.Errorf("%w: empty hex", state.ErrInvalidProofRequest)
	}
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", state.ErrInvalidProofRequest, err.Error())
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("%w: empty hex", state.ErrInvalidProofRequest)
	}
	return decoded, nil
}

func decodeProof(nodes []string) ([][]byte, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("%w: empty proof", state.ErrInvalidProofRequest)
	}
	out := make([][]byte, len(nodes))
	for i, node := range nodes {
		decoded, err := decodeHex(node)
		if err != nil {
			return nil, err
		}
		out[i] = decoded
	}
	return out, nil
}
