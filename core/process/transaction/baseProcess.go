package transaction

import (
	"bytes"
	"errors"
	"fmt"
	"math"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
)

type baseTxProcessor struct {
	cfg config.Config

	kApps          kapp.KAppController
	pubkeyConv     core.PubkeyConverter
	keyGen         crypto.KeyGenerator
	singleSigner   crypto.SingleSigner
	economicsFee   process.EconomicsDataHandler
	hasher         hashing.Hasher
	marshalizer    marshal.Marshalizer
	accountsCacher state.AccountsCacher
	forkController core.ForkController
	scProcessor    process.SmartContractProcessor
}

func (txProc *baseTxProcessor) GetAccounts(
	adrSrc, adrDst []byte,
) (state.UserAccountHandler, state.UserAccountHandler, error) {
	if len(adrSrc) == 0 || len(adrDst) == 0 {
		return nil, nil, process.ErrNilAddressContainer
	}

	if bytes.Equal(adrSrc, adrDst) {
		account, err := txProc.accountsCacher.LoadUser(adrSrc)
		if err != nil {
			return nil, nil, err
		}

		return account, account, nil
	}

	acntSrc, err := txProc.accountsCacher.LoadUser(adrSrc)
	if err != nil {
		return nil, nil, err
	}

	acntDst, err := txProc.accountsCacher.LoadUser(adrDst)
	if err != nil {
		return nil, nil, err
	}

	return acntSrc, acntDst, nil
}

func (txProc *baseTxProcessor) validateNonce(acnt state.UserAccountHandler, tx *transaction.Transaction) error {
	if acnt.GetNonce() < tx.GetNonce() {
		return process.ErrHigherNonceInTransaction
	}
	if acnt.GetNonce() > tx.GetNonce() {
		return process.ErrLowerNonceInTransaction
	}

	return nil
}

func (txProc *baseTxProcessor) validatePermission(tx *transaction.Transaction, permission *state.Permission, txHash []byte) ([][]byte, error) {
	if permission.Type != state.Permission_Owner {
		err := tx.ValidatePermissionOperation(permission.Operations)
		if err != nil {
			return nil, err
		}
	}

	signersPub, err := txProc.loadSignerPublicKeys(permission.Signers)
	if err != nil {
		return nil, fmt.Errorf("failed to load signer public keys: %w", err)
	}

	signedBy, signWeight, err := txProc.verifySignatures(tx, permission, signersPub, txHash)
	if err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	if signWeight < permission.Threshold {
		return nil, fmt.Errorf("%w: (%d/%d)", common.ErrSignatureThreshold, signWeight, permission.Threshold)
	}

	return signedBy, nil
}

// loadSignerPublicKeys loads public keys for all signers in the permission
func (txProc *baseTxProcessor) loadSignerPublicKeys(signers []*state.Key) (map[string]crypto.PublicKey, error) {
	signersPub := make(map[string]crypto.PublicKey, len(signers))
	for _, signer := range signers {
		senderPubKey, err := txProc.keyGen.PublicKeyFromByteArray(signer.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid signer address: %w", err)
		}
		signersPub[string(signer.Address)] = senderPubKey
	}
	return signersPub, nil
}

// verifySignatures checks all signatures against the provided public keys
func (txProc *baseTxProcessor) verifySignatures(tx *transaction.Transaction, permission *state.Permission, signersPub map[string]crypto.PublicKey, txHash []byte) ([][]byte, int64, error) {
	var signedBy [][]byte
	var signWeight int64

	for _, signature := range tx.Signature {
		match := false
		for _, signer := range permission.Signers {
			pub, ok := signersPub[string(signer.Address)]
			if !ok {
				continue
			}
			if txProc.singleSigner.Verify(pub, txHash, signature) == nil {
				signedBy = append(signedBy, signer.Address)
				signedBy = append(signedBy, []byte(fmt.Sprintf("%d", signer.Weight)))
				signWeight += signer.Weight
				match = true
				// remove the signer from the map to avoid duplicate signatures
				delete(signersPub, string(signer.Address))
				break
			}
		}
		if !match {
			return nil, 0, common.ErrInvalidSignature
		}
	}

	return signedBy, signWeight, nil
}

func (txProc *baseTxProcessor) checkTxValues(tx *transaction.Transaction, acntSnd state.UserAccountHandler, txHash []byte) error {
	if err := txProc.validateNonce(acntSnd, tx); err != nil {
		return err
	}

	permission, _, err := acntSnd.GetPermission(tx.RawData.PermissionID)
	if err != nil {
		return err
	}

	signedBy, err := txProc.validatePermission(tx, permission, txHash)
	if err != nil {
		return err
	}

	// clear receipts and add the receipt signed by
	tx.Receipts = []*transaction.Transaction_Receipt{
		NewReceipt(
			SignedBy,
			defaultTXContractID,
			signedBy...,
		),
	}

	// reset transaction result and result code prior to execution
	tx.Result = transaction.Transaction_SUCCESS
	tx.ResultCode = transaction.Transaction_Ok

	// check if the transaction fee is valid based on contract types and network params
	computedCost, err := txProc.economicsFee.CheckValidityTxValues(tx)
	if err != nil {
		return err
	}

	// check if the account has enough balance to pay the fees
	if err := txProc.CheckPaymentFeeBalance(tx, acntSnd); err != nil {
		return err
	}

	return txProc.UpdateTXGas(tx, computedCost)
}

func (txProc *baseTxProcessor) CheckPaymentFeeBalance(tx *transaction.Transaction, acntSnd state.UserAccountHandler) error {
	// check balance and fee with overflow protection
	bandwidthFee := tx.GetBandwidthFee()
	kappFee := tx.GetKAppFee()
	if bandwidthFee > 0 && kappFee > math.MaxInt64-bandwidthFee {
		return fmt.Errorf("%w: fee calculation overflow", process.ErrOverflow)
	}
	totalFees := bandwidthFee + kappFee

	assetID, kdaFees, err := txProc.computeKDAFees(tx, totalFees)
	if err != nil {
		return err
	}

	// check if account has balance (KLV or KDA)
	accountAssetBalance := acntSnd.GetBalance(assetID, txProc.forkController.EnableSmartContracts())
	if accountAssetBalance < kdaFees {
		return fmt.Errorf("%w, has: %d, wanted: %d",
			process.ErrInsufficientFee,
			accountAssetBalance,
			kdaFees,
		)
	}

	return nil
}

func (txProc *baseTxProcessor) computeKDAFees(tx *transaction.Transaction, totalFees int64) ([]byte, int64, error) {
	if !txProc.forkController.FPRComputeAndKdaFeeFlow() {
		return nil, totalFees, nil
	}

	// check if user is paying with kdaFee and if the pool have enough balance to pay the fees
	kdaFee := tx.GetRawData().GetKDAFee().GetKDA()
	if len(kdaFee) > 0 && string(kdaFee) != string(kdautils.KLVIdentifier) {
		totalFeesKDA, err := txProc.kApps.GetKDAFeesPoolKApp().Compute(totalFees, tx.GetRawData().GetKDAFee())
		if err != nil {
			return nil, totalFees, process.ErrComputeKDAFeeError
		}

		return kdaFee, totalFeesKDA, nil
	}

	return nil, totalFees, nil
}

func (txProc *baseTxProcessor) UpdateTXGas(tx *transaction.Transaction, computedCost *transaction.CostResponse) error {
	gasLimit, gasMultiplier, err := txProc.economicsFee.ComputeGas(tx, computedCost)
	if err != nil {
		return err
	}

	tx.GasLimit = gasLimit
	tx.GasMultiplier = gasMultiplier

	return nil
}

func (txProc *baseTxProcessor) getKApp(address []byte) (state.KAppAccountHandler, error) {
	kapp, err := txProc.accountsCacher.LoadKApp(address)
	if err != nil {
		return nil, err
	}

	return kapp, nil
}

func (txProc *baseTxProcessor) GetStakingKApp(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
	stakingKapp, err := txProc.getKApp(kapps.StakingKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	if assetID == nil {
		return stakingKapp, nil, nil
	}

	key := kdautils.ToKDAKey(assetID, nil)

	stakedBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, nil, err
	}
	if len(stakedBytes) == 0 {
		return nil, nil, common.ErrStakingNotFound
	}

	kdaStaking := &kapps.StakingData{}
	err = txProc.marshalizer.Unmarshal(kdaStaking, stakedBytes)
	if err != nil {
		return nil, nil, err
	}

	return stakingKapp, kdaStaking, nil
}

func (txProc *baseTxProcessor) SetStakingKApp(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
	data, err := txProc.marshalizer.Marshal(staking)
	if err != nil {
		return err
	}

	key := kdautils.ToKDAKey(assetID, nil)

	err = stakingKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (txProc *baseTxProcessor) GetKDAKApp(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
	kdaKapp, err := txProc.getKApp(kapps.KDAKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	if assetID == nil {
		return kdaKapp, nil, nil
	}

	key := kdautils.ToKDAKey(assetID, nil)

	kdaBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, nil, err
	}
	if len(kdaBytes) == 0 {
		return nil, nil, common.ErrAssetNotFound
	}

	kda := &kapps.KDAData{}
	err = txProc.marshalizer.Unmarshal(kda, kdaBytes)
	if err != nil {
		return nil, nil, err
	}

	return kdaKapp, kda, nil
}

func (txProc *baseTxProcessor) SetKDAKApp(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
	data, err := txProc.marshalizer.Marshal(kda)
	if err != nil {
		return err
	}

	key := kdautils.ToKDAKey(assetID, nil)

	err = kdaKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (txProc *baseTxProcessor) GetProposalKApp(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
	proposalKApp, err := txProc.getKApp(kapps.ProposalKAppAddress)
	if err != nil {
		return nil, nil, nil, err
	}

	controllerBytes, err := proposalKApp.DataTrieTracker().RetrieveValue(kdautils.ProposalControllerKey)
	if err != nil {
		return nil, nil, nil, err
	}

	proposalController := &kapps.ProposalController{}
	err = txProc.marshalizer.Unmarshal(proposalController, controllerBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	proposal := &kapps.ProposalData{}
	if proposalID != 0 {
		key := kdautils.ToProposalKey(proposalID)

		proposalBytes, err := proposalKApp.DataTrieTracker().RetrieveValue(key)
		if err != nil {
			return nil, nil, nil, err
		}
		if len(proposalBytes) == 0 {
			return nil, nil, nil, common.ErrProposalNotFound
		}
		err = txProc.marshalizer.Unmarshal(proposal, proposalBytes)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return proposalKApp, proposal, proposalController, nil
}

func (txProc *baseTxProcessor) SetProposalKApp(proposalKapp state.KAppAccountHandler, proposalID uint64, proposal *kapps.ProposalData, controller *kapps.ProposalController) error {
	if controller != nil {
		data, err := txProc.marshalizer.Marshal(controller)
		if err != nil {
			return err
		}

		err = proposalKapp.DataTrieTracker().SaveKeyValue(kdautils.ProposalControllerKey, data)
		if err != nil {
			return err
		}
	}

	data, err := txProc.marshalizer.Marshal(proposal)
	if err != nil {
		return err
	}

	key := kdautils.ToProposalKey(proposalID)
	err = proposalKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (txProc *baseTxProcessor) GetITOKApp(assetID []byte) (state.KAppAccountHandler, *kapps.ITOData, error) {
	itoKApp, err := txProc.getKApp(kapps.ITOKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	if assetID == nil {
		return itoKApp, nil, nil
	}

	key := kdautils.ToITOKey(assetID)

	itoBytes, err := itoKApp.DataTrieTracker().RetrieveValue(key)
	if err != nil && !errors.Is(err, common.ErrNilTrie) {
		return nil, nil, err
	}
	if len(itoBytes) == 0 {
		newITO := &kapps.ITOData{
			PackData: make(map[string]*kapps.PackData),
		}
		return itoKApp, newITO, common.ErrNotFoundInKApp
	}

	ito := &kapps.ITOData{}
	err = txProc.marshalizer.Unmarshal(ito, itoBytes)
	if err != nil {
		return nil, nil, err
	}

	return itoKApp, ito, nil
}

func (txProc *baseTxProcessor) SetITOKApp(itoKApp state.KAppAccountHandler, assetID []byte, ito *kapps.ITOData) error {
	data, err := txProc.marshalizer.Marshal(ito)
	if err != nil {
		return err
	}

	key := kdautils.ToITOKey(assetID)

	err = itoKApp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (txProc *baseTxProcessor) GetITOWhitelistByAddress(assetID []byte, hexAddress string) (state.KAppAccountHandler, *kapps.WhitelistData, error) {
	itoKApp, err := txProc.getKApp(kapps.ITOKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	key := kdautils.ToITOWhitelistKey(assetID, hexAddress)

	itoWhitelistBytes, err := itoKApp.DataTrieTracker().RetrieveValue(key)
	if err != nil && !errors.Is(err, common.ErrNilTrie) {
		return nil, nil, err
	}
	if len(itoWhitelistBytes) == 0 {
		newWhitelist := &kapps.WhitelistData{
			Limit: 0,
		}
		return itoKApp, newWhitelist, common.ErrNotFoundInKApp
	}

	itoWhitelist := &kapps.WhitelistData{}
	err = txProc.marshalizer.Unmarshal(itoWhitelist, itoWhitelistBytes)
	if err != nil {
		return nil, nil, err
	}

	return itoKApp, itoWhitelist, nil
}

func (txProc *baseTxProcessor) SetITOWhitelists(itoKApp state.KAppAccountHandler, assetID []byte, whitelist map[string]*kapps.WhitelistData) error {
	for key, value := range whitelist {
		data, err := txProc.marshalizer.Marshal(value)
		if err != nil {
			return err
		}

		whitelistKey := kdautils.ToITOWhitelistKey(assetID, key)

		err = itoKApp.DataTrieTracker().SaveKeyValue(whitelistKey, data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (txProc *baseTxProcessor) GetMarketOrderKApp(orderID []byte) (state.KAppAccountHandler, *kapps.MarketOrderData, error) {
	marketKapp, err := txProc.getKApp(kapps.MarketKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	if orderID == nil {
		return marketKapp, &kapps.MarketOrderData{}, nil
	}

	key := kdautils.ToMarketOrderKey(orderID)

	marketBytes, err := marketKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil && !errors.Is(err, common.ErrNilTrie) {
		return nil, nil, err
	}
	if len(marketBytes) == 0 {
		return marketKapp, &kapps.MarketOrderData{}, common.ErrNotFoundInKApp
	}

	market := &kapps.MarketOrderData{}
	err = txProc.marshalizer.Unmarshal(market, marketBytes)
	if err != nil {
		return nil, nil, err
	}

	return marketKapp, market, nil
}

func (txProc *baseTxProcessor) SetMarketOrderKApp(marketKapp state.KAppAccountHandler, order *kapps.MarketOrderData) error {
	data, err := txProc.marshalizer.Marshal(order)
	if err != nil {
		return err
	}

	key := kdautils.ToMarketOrderKey(order.ID)

	err = marketKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (txProc *baseTxProcessor) GetMarketplaceKApp(marketplaceID []byte) (state.KAppAccountHandler, *kapps.Marketplace, error) {
	marketKapp, err := txProc.getKApp(kapps.MarketKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	if marketplaceID == nil {
		return marketKapp, &kapps.Marketplace{}, nil
	}

	key := kdautils.ToMarketplaceKey(marketplaceID)

	marketBytes, err := marketKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil && !errors.Is(err, common.ErrNilTrie) {
		return nil, nil, err
	}
	if len(marketBytes) == 0 {
		return marketKapp, &kapps.Marketplace{}, common.ErrNotFoundInKApp
	}

	marketplace := &kapps.Marketplace{}
	err = txProc.marshalizer.Unmarshal(marketplace, marketBytes)
	if err != nil {
		return nil, nil, err
	}

	return marketKapp, marketplace, nil
}

func (txProc *baseTxProcessor) SetMarketplaceKApp(marketplaceKapp state.KAppAccountHandler, marketplace *kapps.Marketplace) error {
	data, err := txProc.marshalizer.Marshal(marketplace)
	if err != nil {
		return err
	}

	key := kdautils.ToMarketplaceKey(marketplace.ID)

	err = marketplaceKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (txProc *baseTxProcessor) ClaimBalance(
	claimType transaction.ClaimContract_EnumClaimType,
	assetID []byte,
	block *block.Block,
	acc state.UserAccountHandler,
	staking *kapps.StakingData,
	kda *kapps.KDAData,
	userKDA *kapps.UserKDA,
) (map[string]int64, error) {
	// Market claim does not update balance based on staking process, return here
	if claimType == transaction.ClaimContract_MarketClaim {
		return nil, nil
	}

	blockTime := block.GetTimestamp()
	epoch := block.GetEpoch()

	// compute gains
	gains, err := acc.Claim(claimType,
		assetID,
		epoch,
		blockTime,
		staking,
		kda,
		userKDA,
		txProc.forkController,
	)
	if err != nil {
		return nil, err
	}

	if claimType == transaction.ClaimContract_StakingClaim {
		if txProc.forkController.ClaimKFI() ||
			gains[string(assetID)] > 0 {
			userKDA.LastClaim.Timestamp = blockTime
			userKDA.LastClaim.Epoch = epoch
		}
	}

	for key, value := range gains {
		if value > 0 {
			var userKDAToUpdate *kapps.UserKDA
			if bytes.Equal([]byte(key), assetID) {
				userKDAToUpdate = userKDA
			}

			err = acc.AddToBalance(value, []byte(key), txProc.forkController.EnableSmartContracts(), userKDAToUpdate)
			if err != nil {
				return nil, err
			}
		}
	}

	return gains, nil
}
