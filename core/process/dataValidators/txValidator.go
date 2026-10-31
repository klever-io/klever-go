package dataValidators

import (
	"encoding/hex"
	"errors"
	"fmt"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto"
	disabledSig "github.com/klever-io/klever-go/crypto/signing/disabled/singlesig"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
)

var _ process.TxValidator = (*txValidator)(nil)

var log = logger.GetOrCreate("process/dataValidator")

// txValidator represents a tx handler validator that doesn't check the validity of provided txHandler
type txValidator struct {
	accounts             state.AccountsAdapter
	txStorer             storage.Storer
	dataPool             retriever.PoolsHolder
	whiteListHandler     process.WhiteListHandler
	pubkeyConverter      core.PubkeyConverter
	singleSigner         crypto.SingleSigner
	keyGen               crypto.KeyGenerator
	kAppController       kapp.KAppController
	maxNonceDeltaAllowed int
}

// NewTxValidator creates a new nil tx handler validator instance
func NewTxValidator(
	accounts state.AccountsAdapter,
	txStorer storage.Storer,
	dataPool retriever.PoolsHolder,
	whiteListHandler process.WhiteListHandler,
	pubkeyConverter core.PubkeyConverter,
	singleSigner crypto.SingleSigner,
	keyGen crypto.KeyGenerator,
	kAppController kapp.KAppController,
	maxNonceDeltaAllowed int,
) (*txValidator, error) {
	if check.IfNil(accounts) {
		return nil, common.ErrNilAccountsAdapter
	}
	if check.IfNil(whiteListHandler) {
		return nil, process.ErrNilWhiteListHandler
	}
	if check.IfNil(txStorer) {
		return nil, process.ErrNilStorage
	}
	if check.IfNil(dataPool) {
		return nil, common.ErrNilDataPoolHolder
	}
	if check.IfNil(pubkeyConverter) {
		return nil, fmt.Errorf("%w in NewTxValidator", common.ErrNilPubkeyConverter)
	}
	if check.IfNil(singleSigner) {
		return nil, common.ErrNilSingleSigner
	}
	if check.IfNil(keyGen) {
		return nil, common.ErrNilKeyGen
	}
	if check.IfNil(kAppController) {
		return nil, common.ErrNilKAppController
	}

	return &txValidator{
		accounts:             accounts,
		txStorer:             txStorer,
		dataPool:             dataPool,
		whiteListHandler:     whiteListHandler,
		pubkeyConverter:      pubkeyConverter,
		maxNonceDeltaAllowed: maxNonceDeltaAllowed,
		singleSigner:         singleSigner,
		kAppController:       kAppController,
		keyGen:               keyGen,
	}, nil
}

// CheckTxValidity will filter transactions that needs to be added in pools
func (txv *txValidator) CheckTxValidity(interceptedTx process.TxValidatorHandler) error {
	var txHash []byte
	interceptedData, ok := interceptedTx.(process.InterceptedData)
	if ok {
		txHash = interceptedData.Hash()
	}
	isWhiteListed := ok && txv.whiteListHandler.IsWhiteListed(interceptedData)

	senderAddress := interceptedTx.SenderAddress()

	// Consensus account freeze: keep a frozen sender's tx out of this node's pool, so
	// this node never proposes it. Gossip has already relayed the message by the time
	// this runs, so it does not stop propagation. Local ingestion policy, applied from
	// binary rollout like the API check in node.SendTransaction; skipped for a
	// whitelisted tx, so a tx requested to process a block is still fetched. Returns a
	// generic rejection (not a freeze-specific error) because this validator also backs
	// the create / estimate endpoints, which must not reveal the frozen set. The
	// epoch-start bootstrap wires a disabled kApp controller with no fork controller;
	// skip it there.
	forkController := txv.kAppController.GetForkController()
	if !isWhiteListed && !check.IfNil(forkController) && common.IsAccountFrozenAtAPI(senderAddress, forkController) {
		log.Debug("rejected tx from frozen account", "sender", hex.EncodeToString(senderAddress))
		return process.ErrWrongTransaction
	}

	accountHandler, err := txv.accounts.GetExistingAccount(senderAddress)
	if err != nil {
		// during sync a requested tx may come from an account created in a block the
		// node hasn't processed yet; a new account starts with the default owner permission
		if isWhiteListed && errors.Is(err, common.ErrAccNotFound) {
			return txv.checkSignatures(interceptedTx, txHash, state.NewUserAccountNoCheck(senderAddress))
		}
		return fmt.Errorf("%w for address %s, err: %s",
			process.ErrAccountNotFound,
			txv.pubkeyConverter.Encode(senderAddress),
			err.Error(),
		)
	}

	if accountHandler == nil {
		log.Debug("CheckTxValidity disabled account")
		return nil
	}

	account, ok := accountHandler.(state.UserAccountHandler)
	if !ok {
		return fmt.Errorf("%w, account is not of type *state.Account, address: %s",
			process.ErrWrongTypeAssertion,
			txv.pubkeyConverter.Encode(senderAddress),
		)
	}

	// a requested tx skips the nonce and balance checks, which earlier txs
	// of its block may change, but never the signature check (KLR-74)
	if !isWhiteListed {
		err = txv.checkNonceAndBalance(interceptedTx, account)
		if err != nil {
			return err
		}
	}

	return txv.checkSignatures(interceptedTx, txHash, account)
}

func (txv *txValidator) checkNonceAndBalance(
	interceptedTx process.TxValidatorHandler,
	account state.UserAccountHandler,
) error {
	senderAddress := interceptedTx.SenderAddress()
	accountNonce := account.GetNonce()
	txNonce := interceptedTx.Nonce()
	lowerNonceInTx := txNonce < accountNonce
	veryHighNonceInTx := txNonce > accountNonce+uint64(txv.maxNonceDeltaAllowed) // #nosec G115
	isTxRejected := lowerNonceInTx || veryHighNonceInTx
	if isTxRejected {
		return fmt.Errorf("%w lowerNonceInTx: %v, veryHighNonceInTx: %v",
			process.ErrWrongTransaction,
			lowerNonceInTx,
			veryHighNonceInTx,
		)
	}

	// if TX paying fees with KDA, check KDA balance
	txFee := interceptedTx.Fee()
	assetFee := []byte(nil)
	kdaFee := interceptedTx.KDAFee()
	if !check.IfNil(kdaFee) {
		// validate if asset +fee is valid
		err := txv.kAppController.GetKDAFeesPoolKApp().Validate(senderAddress, txFee, kdaFee)
		if err != nil {
			return fmt.Errorf("%w, fail to validate KDA fee", err)
		}

		assetFee = kdaFee.GetKDA()
		txFee = kdaFee.GetAmount()
	}

	accountBalance := account.GetBalance(
		assetFee,
		txv.kAppController.GetForkController().EnableSmartContracts(),
	)
	if accountBalance < txFee {
		if len(assetFee) == 0 {
			assetFee = kdautils.KLVIdentifier
		}

		return fmt.Errorf("%w, for address: %s, kda: %s, wanted %v, have %v",
			process.ErrInsufficientFunds,
			txv.pubkeyConverter.Encode(senderAddress),
			string(assetFee),
			txFee,
			accountBalance,
		)
	}

	return nil
}

func (txv *txValidator) checkSignatures(
	interceptedTx process.TxValidatorHandler,
	txHash []byte,
	account state.UserAccountHandler,
) error {
	switch txv.singleSigner.(type) {
	case *disabledSig.DisabledSingleSig:
		return nil
	}

	// get permission by ID
	permission, _, err := account.GetPermission(interceptedTx.PermissionID())
	if err != nil {
		// permission not found for account
		return err
	}

	// the same checks as block processing, through the same helpers (KLR-74)
	err = process.CheckPermissionOperation(interceptedTx, permission)
	if err != nil {
		return err
	}

	signerKeys, err := process.LoadPermissionSignerKeys(txv.keyGen, permission.Signers)
	if err != nil {
		return err
	}

	_, signWeight, err := process.VerifyPermissionSignatures(txv.singleSigner, permission, signerKeys, interceptedTx.Signature(), txHash)
	if err != nil {
		return err
	}

	return process.CheckPermissionThreshold(permission, signWeight)
}

// CheckTxWhiteList will check if the transactions are whitelisted and could be added in pools
func (txv *txValidator) CheckTxWhiteList(data process.InterceptedData) error {
	// TODO: refactor

	if txv.whiteListHandler.IsWhiteListed(data) {
		return nil
	}

	return common.ErrTransactionIsNotWhitelisted
}

// CheckDup will check if transactions are whitelisted and could be added in pools
func (txv *txValidator) CheckDup(txHash []byte) error {
	// check cache pool first for efficiency
	_, exist := txv.dataPool.Transactions().SearchFirstData(txHash)
	if exist {
		return common.ErrDupTransaction
	}

	// check storer
	if txv.txStorer.Has(txHash) == nil {
		return fmt.Errorf("%w: storer", common.ErrDupTransaction)
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (txv *txValidator) IsInterfaceNil() bool {
	return txv == nil
}
