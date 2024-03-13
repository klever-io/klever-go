package dataValidators

import (
	"fmt"
	"sync"

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
	interceptedData, ok := interceptedTx.(process.InterceptedData)
	if ok {
		if txv.whiteListHandler.IsWhiteListed(interceptedData) {
			return nil
		}
	}

	senderAddress := interceptedTx.SenderAddress()
	accountHandler, err := txv.accounts.GetExistingAccount(senderAddress)
	if err != nil {
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

	accountNonce := accountHandler.GetNonce()
	txNonce := interceptedTx.Nonce()
	lowerNonceInTx := txNonce < accountNonce
	veryHighNonceInTx := txNonce > accountNonce+uint64(txv.maxNonceDeltaAllowed)
	isTxRejected := lowerNonceInTx || veryHighNonceInTx
	if isTxRejected {
		return fmt.Errorf("%w lowerNonceInTx: %v, veryHighNonceInTx: %v",
			process.ErrWrongTransaction,
			lowerNonceInTx,
			veryHighNonceInTx,
		)
	}

	account, ok := accountHandler.(state.UserAccountHandler)
	if !ok {
		return fmt.Errorf("%w, account is not of type *state.Account, address: %s",
			process.ErrWrongTypeAssertion,
			txv.pubkeyConverter.Encode(senderAddress),
		)
	}

	// if TX paying fees with KDA, check KDA balance
	txFee := interceptedTx.Fee()
	assetFee := []byte(nil)
	kdaFee := interceptedTx.KDAFee()
	if !check.IfNil(kdaFee) {
		// validate if asset +fee is valid
		err = txv.kAppController.GetKDAFeesPoolKApp().Validate(txFee, kdaFee)
		if err != nil {
			return fmt.Errorf("%w, fail to validate KDA fee", err)
		}

		assetFee = kdaFee.GetKDA()
		txFee = kdaFee.GetAmount()
	}

	accountBalance := account.GetBalance(assetFee)
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

	// check if permission is allowed by TX
	if permission.Type != state.Permission_Owner {
		// check on TX
		err = interceptedTx.ValidatePermission(permission.Operations)
		if err != nil {
			return err
		}
	}

	signersPub := make(map[string]crypto.PublicKey)
	wait := sync.WaitGroup{}
	var mu sync.Mutex
	var loadErr error
	for _, signer := range permission.Signers {
		wait.Add(1)
		go func(addrPub string, wg *sync.WaitGroup, sig map[string]crypto.PublicKey, mut *sync.Mutex) {
			defer wg.Done()
			senderPubKey, err := txv.keyGen.PublicKeyFromByteArray([]byte(addrPub))
			if err != nil {
				// signer address is invalid
				loadErr = err
				return
			}
			mut.Lock()
			sig[addrPub] = senderPubKey
			mut.Unlock()
		}(string(signer.Address), &wait, signersPub, &mu)
	}

	wait.Wait()
	if loadErr != nil {
		return loadErr
	}

	signWeight := int64(0)
	for _, s1 := range interceptedTx.Signature() {
		// check keys
		match := false
		for _, signer := range permission.Signers {
			pub := signersPub[string(signer.Address)]
			if txv.singleSigner.Verify(pub, interceptedData.Hash(), s1) == nil {
				signWeight += signer.Weight // Use signer weight
				match = true
				break
			}
		}
		if !match {
			// no signer found in account permission for this signature
			return common.ErrInvalidSignature
		}
	}

	// check threshold
	if signWeight < permission.Threshold {
		return fmt.Errorf("%w: (%d/%d)", common.ErrSignatureThreshold, signWeight, permission.Threshold)
	}

	// TODO: add to whitelist?
	return nil
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
	//TODO: Investigate if it is needed
	// if txv.txStorer.Has(txHash) == nil {
	// 	return fmt.Errorf("%w: storer", common.ErrDupTransaction)
	// }

	// _, exist := txv.dataPool.Transactions().SearchFirstData(txHash)
	// if exist {
	// 	return common.ErrDupTransaction
	// }

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (txv *txValidator) IsInterfaceNil() bool {
	return txv == nil
}
