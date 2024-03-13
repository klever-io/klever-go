package indexer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/klever-io/klever-go/core"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	ptx "github.com/klever-io/klever-go/core/process/transaction"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/converters"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/templates/noKibana"
	"github.com/klever-io/klever-go/indexer/templates/withKibana"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
)

type commonProcessor struct {
	addressPubkeyConverter   core.PubkeyConverter
	validatorPubkeyConverter core.PubkeyConverter
}

func NewCommonProcessor(
	addressPubkeyConverter core.PubkeyConverter,
	validatorPubkeyConverter core.PubkeyConverter,
) (CommonProcessor, error) {
	if check.IfNil(addressPubkeyConverter) {
		return nil, ErrNilPubkeyConverter
	}
	if check.IfNil(validatorPubkeyConverter) {
		return nil, ErrNilPubkeyConverter
	}

	return &commonProcessor{
		addressPubkeyConverter:   addressPubkeyConverter,
		validatorPubkeyConverter: validatorPubkeyConverter,
	}, nil
}

// GetElasticTemplatesAndPolicies will return elastic templates and policies
func GetElasticTemplatesAndPolicies(useKibana bool) (map[string]*bytes.Buffer, map[string]*bytes.Buffer, error) {
	var indexTemplates map[string]*bytes.Buffer
	indexPolicies := make(map[string]*bytes.Buffer)

	if useKibana {
		indexTemplates = getTemplatesKibana()
		indexPolicies = getPolicies()

		return indexTemplates, indexPolicies, nil
	}

	indexTemplates = getTemplatesNoKibana()

	return indexTemplates, indexPolicies, nil
}

func getTemplatesKibana() map[string]*bytes.Buffer {
	indexTemplates := make(map[string]*bytes.Buffer)

	indexTemplates["opendistro"] = withKibana.OpenDistro.ToBuffer()
	indexTemplates[txIndex] = withKibana.Transactions.ToBuffer()
	indexTemplates[blockIndex] = withKibana.Blocks.ToBuffer()
	indexTemplates[epochIndex] = withKibana.Epoch.ToBuffer()
	indexTemplates[accountsIndex] = withKibana.Accounts.ToBuffer()
	indexTemplates[accountsKDAIndex] = withKibana.AccountsKDA.ToBuffer()
	indexTemplates[peersAccountsIndex] = withKibana.PeersAccounts.ToBuffer()
	indexTemplates[logsIndex] = withKibana.Logs.ToBuffer()
	indexTemplates[scDeploysIndex] = withKibana.SCDeploys.ToBuffer()
	indexTemplates[assetsIndex] = withKibana.Assets.ToBuffer()
	indexTemplates[proposalsIndex] = withKibana.Proposals.ToBuffer()
	indexTemplates[marketplacesIndex] = withKibana.Marketplaces.ToBuffer()
	indexTemplates[marketplaceOrdersIndex] = withKibana.MarketplaceOrders.ToBuffer()
	indexTemplates[accountsHistoryIndex] = withKibana.AccountsHistory.ToBuffer()
	indexTemplates[kdaPoolsIndex] = withKibana.KDAPools.ToBuffer()
	indexTemplates[iTOsIndex] = withKibana.ITOs.ToBuffer()

	return indexTemplates
}

func getTemplatesNoKibana() map[string]*bytes.Buffer {
	indexTemplates := make(map[string]*bytes.Buffer)

	indexTemplates["opendistro"] = noKibana.OpenDistro.ToBuffer()
	indexTemplates[txIndex] = noKibana.Transactions.ToBuffer()
	indexTemplates[blockIndex] = noKibana.Blocks.ToBuffer()
	indexTemplates[epochIndex] = noKibana.Epoch.ToBuffer()
	indexTemplates[accountsIndex] = noKibana.Accounts.ToBuffer()
	indexTemplates[accountsKDAIndex] = noKibana.AccountsKDA.ToBuffer()
	indexTemplates[peersAccountsIndex] = noKibana.PeersAccounts.ToBuffer()
	indexTemplates[logsIndex] = noKibana.Logs.ToBuffer()
	indexTemplates[scDeploysIndex] = noKibana.SCDeploys.ToBuffer()
	indexTemplates[assetsIndex] = noKibana.Assets.ToBuffer()
	indexTemplates[proposalsIndex] = noKibana.Proposals.ToBuffer()
	indexTemplates[marketplacesIndex] = noKibana.Marketplaces.ToBuffer()
	indexTemplates[marketplaceOrdersIndex] = noKibana.MarketplaceOrders.ToBuffer()
	indexTemplates[accountsHistoryIndex] = noKibana.AccountsHistory.ToBuffer()
	indexTemplates[kdaPoolsIndex] = noKibana.KDAPools.ToBuffer()
	indexTemplates[iTOsIndex] = noKibana.ITOs.ToBuffer()

	return indexTemplates
}

func getPolicies() map[string]*bytes.Buffer {
	indexesPolicies := make(map[string]*bytes.Buffer)

	indexesPolicies[txPolicy] = withKibana.TransactionsPolicy.ToBuffer()
	indexesPolicies[blockPolicy] = withKibana.BlocksPolicy.ToBuffer()
	indexesPolicies[ratingPolicy] = withKibana.RatingPolicy.ToBuffer()
	indexesPolicies[accountsHistoryPolicy] = withKibana.AccountsHistoryPolicy.ToBuffer()

	return indexesPolicies
}

func (cm *commonProcessor) receiptToMap(data [][]byte) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	if len(data) == 0 {
		return m, nil
	}

	if len(data[0]) < 2 {
		return nil, fmt.Errorf("%w: ID (%d/%d)", ErrInvalidDataMapLen, len(data[0]), 2)
	}

	m["type"] = ptx.ReceiptType(data[0][0])
	m["cID"] = int(data[0][1])
	m["typeString"] = ptx.ReceiptType(data[0][0]).String()

	switch m["type"] {
	case ptx.Transfer:
		if len(data) < 7 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 6)
		}

		if len(data) < 9 {
			data = append(data, make([][]byte, 2)...)
		}

		assetId := string(data[4])
		if len(data[5]) > 0 {
			assetId += kapps.Sp + string(data[5])
		}

		txValueInt, err := strconv.ParseInt(string(data[3]), 10, 64)
		if err != nil {
			return nil, err
		}

		m["from"] = cm.addressPubkeyConverter.Encode(data[1])
		m["to"] = cm.addressPubkeyConverter.Encode(data[2])
		m["assetId"] = assetId
		m["value"] = txValueInt
		m["assetType"] = kapps.KDAData_EnumAssetType(data[6][0]).String()
		m["marketplaceId"] = hex.EncodeToString(data[7])
		m["orderId"] = hex.EncodeToString(data[8])
	case ptx.CreateKDA:
		if len(data) < 2 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["assetId"] = string(data[1])
	case ptx.UpdateKDA:
		if len(data) < 2 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["assetId"] = string(data[1])
	case ptx.Freeze:
		if len(data) < 5 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 5)
		}
		m["bucketId"] = hex.EncodeToString(data[1])
		m["from"] = cm.addressPubkeyConverter.Encode(data[2])
		m["assetId"] = string(data[3])
		m["value"] = string(data[4])
	case ptx.Unfreeze:
		if len(data) < 6 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 6)
		}
		m["bucketId"] = hex.EncodeToString(data[1])
		m["availableEpoch"] = string(data[2]) // strConv from int???
		m["from"] = cm.addressPubkeyConverter.Encode(data[3])
		m["assetId"] = string(data[4])
		m["value"] = string(data[5])
	case ptx.Proposal:
		if len(data) < 2 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["proposalId"] = string(data[1])
	case ptx.ProposalVote:
		if len(data) < 5 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 5)
		}
		m["proposalId"] = string(data[1])
		m["voter"] = cm.addressPubkeyConverter.Encode(data[2])
		var err error
		m["voteType"], err = strconv.ParseInt(string(data[3]), 10, 64)
		if err != nil {
			return nil, err
		}
		m["votes"], err = strconv.ParseInt(string(data[4]), 10, 64)
		if err != nil {
			return nil, err
		}
	case ptx.Delegate:
		if len(data) < 5 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 5)
		}
		m["from"] = cm.addressPubkeyConverter.Encode(data[1])
		m["bucketId"] = hex.EncodeToString(data[2])
		m["amountDelegated"] = string(data[4])
		if len(data[3]) > 0 {
			m["delegate"] = cm.addressPubkeyConverter.Encode(data[3])
		} else {
			m["delegate"] = ""
		}
	case ptx.UpdateValidator:
		if len(data) < 2 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["id"] = cm.addressPubkeyConverter.Encode(data[1])
	case ptx.ConfigITO:
		if len(data) < 2 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["assetId"] = string(data[1])
	case ptx.SetITOPrices:
		if len(data) < 2 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["assetId"] = string(data[1])
	case ptx.CreateMarketplace:
		if len(data) < 2 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["marketplaceId"] = hex.EncodeToString(data[1])
	case ptx.ConfigMarketplace:
		if len(data) < 2 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["marketplaceId"] = hex.EncodeToString(data[1])
	case ptx.SignedBy:
		for idx := 1; idx < len(data)-1; idx += 2 {
			m["signer"] = cm.addressPubkeyConverter.Encode(data[idx])
			m["weight"] = string(data[idx+1])
		}
	case ptx.Claim:
		if len(data) < 7 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 7)
		}

		amount, err := strconv.ParseInt(string(data[1]), 10, 64)
		if err != nil {
			return nil, err
		}

		claimType, err := strconv.ParseInt(string(data[6]), 10, 64)
		if err != nil {
			return nil, err
		}

		m["amount"] = amount
		m["orderId"] = hex.EncodeToString(data[2])
		m["marketplaceId"] = hex.EncodeToString(data[3])
		m["assetId"] = string(data[4])
		m["assetIdReceived"] = string(data[5])
		m["claimType"] = claimType
		m["claimTypeString"] = transaction.ClaimContract_EnumClaimType_name[int32(claimType)]
	case ptx.Sell:
		if len(data) < 3 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 3)
		}
		m["orderId"] = hex.EncodeToString(data[1])
		m["marketplaceId"] = hex.EncodeToString(data[2])
	case ptx.Buy:
		if len(data) < 7 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 7)
		}

		amount, err := strconv.ParseInt(string(data[5]), 10, 64)
		if err != nil {
			return nil, err
		}

		executed := false
		if string(data[3]) == "1" {
			executed = true
		}

		m["orderId"] = hex.EncodeToString(data[1])
		m["marketplaceId"] = hex.EncodeToString(data[2])
		m["executed"] = executed
		m["bidder"] = cm.addressPubkeyConverter.Encode(data[4])
		m["amount"] = amount
		m["currencyId"] = string(data[6])
	case ptx.Withdraw:
		if len(data) < 4 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 4)
		}
		m["from"] = cm.addressPubkeyConverter.Encode(data[1])
		m["assetId"] = string(data[2])
		m["amount"] = string(data[3])

	case ptx.UpdateMetadata:
		if len(data) < 4 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 4)
		}
		m["owner"] = cm.addressPubkeyConverter.Encode(data[1])
		m["assetId"] = string(data[2])
		m["nftNonce"] = string(data[3])

	case ptx.UpdateKDAPool:
		if len(data) < 2 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["poolId"] = string(data[1])
	case ptx.UpdateITO:
		if len(data) == 4 {
			amount, err := strconv.ParseInt(string(data[3]), 10, 64)
			if err != nil {
				return nil, err
			}

			m["address"] = cm.addressPubkeyConverter.Encode(data[1])
			m["assetId"] = string(data[2])
			m["amount"] = amount
		} else {
			m["assetId"] = string(data[1])
		}
	case ptx.Deposit:
		if len(data) < 6 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 4)
		}
		m["from"] = cm.addressPubkeyConverter.Encode(data[1])
		m["depositType"] = string(data[2])
		m["amount"] = string(data[3])
		m["assetId"] = string(data[4])
		m["currencyId"] = string(data[5])
	case ptx.CancelOrder:
		if len(data) < 2 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["marketplaceId"] = hex.EncodeToString(data[1])
		m["orderId"] = hex.EncodeToString(data[2])
	case ptx.SCTrigger:
		if len(data) < 3 {
			return nil, fmt.Errorf("%w: (%d/%d)", ErrInvalidDataMapLen, len(data), 2)
		}
		m["triggerType"] = string(data[1])
		m["from"] = cm.addressPubkeyConverter.Encode(data[2])
		m["contract"] = cm.addressPubkeyConverter.Encode(data[3])
	}

	return m, nil
}

func (cm *commonProcessor) BuildTransaction(
	tx *transaction.Transaction,
	txHash string,
	header nodeData.HeaderHandler,
) *data.Transaction {

	receipts := make([]map[string]interface{}, 0)
	for _, receipt := range tx.Receipts {
		// convert slice to map
		data, err := cm.receiptToMap(receipt.Data)
		if err != nil {
			_ = bugsnag.Notify(fmt.Errorf("invalid receipt: %w", err), bugsnag.MetaData{"data": {"receipt": receipt.Data}})
			log.Error("invalid receipt", "error", err.Error())
			continue
		}
		receipts = append(receipts, data)
	}

	status := "success"
	if tx.Result == transaction.Transaction_FAILED {
		status = "fail"
	}

	signatures := make([]string, 0)
	for _, sig := range tx.Signature {
		signatures = append(signatures, hex.EncodeToString(sig))
	}

	var parsedData []string
	for _, d := range tx.RawData.Data {
		parsedData = append(parsedData, hex.EncodeToString(d))
	}

	var kdaFee *data.KDAFee

	if tx.RawData.KDAFee != nil &&
		len(tx.RawData.KDAFee.KDA) > 0 {
		kdaFee = &data.KDAFee{
			KDA:    string(tx.RawData.KDAFee.KDA),
			Amount: tx.RawData.KDAFee.Amount,
		}
	}

	return &data.Transaction{
		Hash:         txHash,
		BlockNum:     header.GetNonce(),
		Sender:       cm.addressPubkeyConverter.Encode(tx.RawData.Sender),
		Nonce:        tx.RawData.Nonce,
		PermissionID: tx.RawData.PermissionID,
		Data:         parsedData,
		Timestamp:    time.Duration(header.GetTimestamp() * 1000),
		KAppFee:      tx.RawData.KAppFee,
		KDAFee:       kdaFee,
		BandwidthFee: tx.RawData.BandwidthFee,
		Status:       status,
		ResultCode:   tx.ResultCode.String(),
		Version:      tx.RawData.Version,
		Receipts:     receipts,
		ChainID:      string(tx.RawData.ChainID),
		Signature:    signatures,
	}
}

func (cm *commonProcessor) DecodeContract(dbTx *data.Transaction, tx *transaction.Transaction, alteredAccounts data.AlteredAccountsHandler, alteredIto data.AlteredITOHandler, blockTimestamp int64) error {
	statusOK := tx.Result == transaction.Transaction_SUCCESS && tx.GetResultCode() == transaction.Transaction_Ok
	if dbTx.Contracts == nil {
		dbTx.Contracts = []*data.TXContract{}
	}

	for _, contract := range tx.RawData.Contract {
		switch contract.Type {
		case transaction.TXContract_TransferContractType:
			transferContract, err := contract.GetTransferContract()
			if err != nil {
				log.Warn("error decoding transfer contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertTransferContract(transferContract))

		case transaction.TXContract_CreateAssetContractType:
			createAssetContract, err := contract.GetCreateAssetContract()
			if err != nil {
				log.Warn("error decoding createAsset contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			convertedAssetContract := cm.convertAssetContractInfo(createAssetContract, blockTimestamp)
			dbTx.Contracts = append(dbTx.Contracts, convertedAssetContract)

		case transaction.TXContract_CreateValidatorContractType:
			createValidatorContract, err := contract.GetCreateValidatorContract()
			if err != nil {
				log.Warn("error decoding createValidator contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			convertedCreateValidatorContract := cm.convertCreateValidatorContract(createValidatorContract)

			dbTx.Contracts = append(dbTx.Contracts, convertedCreateValidatorContract)

		case transaction.TXContract_ValidatorConfigContractType:
			validatorConfigContract, err := contract.GetValidatorConfigContract()
			if err != nil {
				log.Warn("error decoding validatorConfig contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertValidatorConfigContract(validatorConfigContract))

		case transaction.TXContract_FreezeContractType:
			freezeContract, err := contract.GetFreezeContract()
			if err != nil {
				log.Warn("error decoding freeze contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertFreezeContract(freezeContract))

		case transaction.TXContract_UnfreezeContractType:
			unfreezeContract, err := contract.GetUnfreezeContract()
			if err != nil {
				log.Warn("error decoding unfreeze contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertUnfreezeContract(unfreezeContract))

		case transaction.TXContract_DelegateContractType:
			delegateContract, err := contract.GetDelegateContract()
			if err != nil {
				log.Warn("error decoding delegate contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			// TODO: check if receipt changes asset
			if statusOK && alteredAccounts != nil {
				alteredAccounts.Add(dbTx.Sender, &data.AlteredAccount{
					IsKDAOperation:  true,
					TokenIdentifier: KLVAssetID,
				})
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertDelegateContract(delegateContract))

		case transaction.TXContract_UndelegateContractType:
			undelegateContract, err := contract.GetUndelegateContract()
			if err != nil {
				log.Warn("error decoding undelegate contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			// TODO: check if receipt changes asset
			if statusOK && alteredAccounts != nil {
				alteredAccounts.Add(dbTx.Sender, &data.AlteredAccount{
					IsKDAOperation:  true,
					TokenIdentifier: KLVAssetID,
				})
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertUndelegateContract(undelegateContract))

		case transaction.TXContract_DepositContractType:
			convertDepositContract, err := contract.GetDepositContract()
			if err != nil {
				log.Warn("error decoding deposit contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertDepositContract(convertDepositContract))

		case transaction.TXContract_WithdrawContractType:
			withdrawContract, err := contract.GetWithdrawContract()
			if err != nil {
				log.Warn("error decoding withdraw contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertWithdrawContract(withdrawContract))

		case transaction.TXContract_ClaimContractType:
			claimContract, err := contract.GetClaimContract()
			if err != nil {
				log.Warn("error decoding claim contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertClaimContract(claimContract))

		case transaction.TXContract_UnjailContractType:
			dbTx.Contracts = append(dbTx.Contracts, &data.TXContract{
				Type:       transaction.TXContract_UnjailContractType,
				TypeString: transaction.TXContract_UnjailContractType.String(),
			})

		case transaction.TXContract_AssetTriggerContractType:
			assetTriggerContract, err := contract.GetAssetTriggerContract()
			if err != nil {
				log.Warn("error decoding assetTrigger contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertAssetTriggerContract(assetTriggerContract))

		case transaction.TXContract_SetAccountNameContractType:
			setAccountNameContract, err := contract.GetSetAccountNameContract()
			if err != nil {
				log.Warn("error decoding setAccountName contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			// Name is set in sender account which is updated on any TX...

			dbTx.Contracts = append(dbTx.Contracts, cm.convertSetAccountNameContract(setAccountNameContract))

		case transaction.TXContract_ProposalContractType:
			proposalContract, err := contract.GetProposalContract()
			if err != nil {
				log.Warn("error decoding proposal contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			convertedProposalContract := cm.convertProposalContract(proposalContract)
			dbTx.Contracts = append(dbTx.Contracts, convertedProposalContract)

		case transaction.TXContract_VoteContractType:
			voteContract, err := contract.GetVoteContract()
			if err != nil {
				log.Warn("error decoding vote contract for indexing (will skip tx)", "error", err)
				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertVoteContract(voteContract))
		case transaction.TXContract_ConfigITOContractType:
			configITOContract, err := contract.GetConfigITOContract()
			if err != nil {
				log.Warn("error decoding configITO contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			convertedWhiteList := make(map[string]data.WhitelistInfo)

			for key, value := range configITOContract.WhitelistInfo {
				finalAddress := key
				decodedAddress, err := hex.DecodeString(key)
				if err != nil {
					log.Error("can not decode address")
				} else {
					finalAddress = cm.addressPubkeyConverter.Encode(decodedAddress)
				}

				convertedWhiteList[finalAddress] = data.WhitelistInfo{
					Address: finalAddress,
					Limit:   value.GetLimit(),
				}
			}

			if statusOK && alteredIto != nil {
				alteredIto.Add(string(configITOContract.AssetID), &data.AlteredITOs{
					IsNew:                  true,
					AddedAddresses:         convertedWhiteList,
					DefaultLimitPerAddress: configITOContract.DefaultLimitPerAddress,
				})
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertConfigITOContract(configITOContract))
		case transaction.TXContract_SetITOPricesContractType:
			setITOPricesContract, err := contract.GetSetITOPricesContract()
			if err != nil {
				log.Warn("error decoding setITOPrices contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertSetITOPricesContract(setITOPricesContract))
		case transaction.TXContract_BuyContractType:
			buyContract, err := contract.GetBuyContract()
			if err != nil {
				log.Warn("error decoding buy contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertBuyContract(buyContract))
		case transaction.TXContract_SellContractType:
			sellContract, err := contract.GetSellContract()
			if err != nil {
				log.Warn("error decoding sell contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertSellContract(sellContract))
		case transaction.TXContract_CancelMarketOrderContractType:
			cancelMarketOrderContract, err := contract.GetCancelMarketOrderContract()
			if err != nil {
				log.Warn("error decoding cancelMarketplace contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertCancelMarketOrderContract(cancelMarketOrderContract))
		case transaction.TXContract_CreateMarketplaceContractType:
			createMarketplaceContract, err := contract.GetCreateMarketplaceContract()
			if err != nil {
				log.Warn("error decoding createMarketplace contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertCreateMarketplaceContract(createMarketplaceContract))
		case transaction.TXContract_ConfigMarketplaceContractType:
			configMarketplaceContract, err := contract.GetConfigMarketplaceContract()
			if err != nil {
				log.Warn("error decoding configMarketplace contract for indexing (will skip tx)", "error", err)

				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertConfigMarketplaceContract(configMarketplaceContract))
		case transaction.TXContract_UpdateAccountPermissionContractType:
			updateAccountPermissionContract, err := contract.GetUpdateAccountPermissionContract()
			if err != nil {
				log.Warn("error decoding updateAccountPermission contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertUpdateAccountPermissionContract(updateAccountPermissionContract))
		case transaction.TXContract_ITOTriggerContractType:
			ITOTriggerContract, err := contract.GetITOTriggerContract()
			if err != nil {
				log.Warn("error decoding ITO trigger contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			convertedWhiteList := make(map[string]data.WhitelistInfo)

			for key, value := range ITOTriggerContract.WhitelistInfo {
				finalAddress := key
				decodedAddress, err := hex.DecodeString(key)
				if err != nil {
					log.Error("can not decode address")
				} else {
					finalAddress = cm.addressPubkeyConverter.Encode(decodedAddress)
				}

				convertedWhiteList[finalAddress] = data.WhitelistInfo{
					Address: finalAddress,
					Limit:   value.GetLimit(),
				}
			}

			switch ITOTriggerContract.TriggerType {
			case transaction.ITOTriggerContract_UpdateStatus:
				if statusOK && alteredIto != nil {
					alteredIto.Add(string(ITOTriggerContract.AssetID), &data.AlteredITOs{
						IsDisabled: ITOTriggerContract.Status == transaction.ITOTriggerContract_PausedITO,
						IsEnabled: ITOTriggerContract.Status == transaction.ITOTriggerContract_DefaultITO ||
							ITOTriggerContract.Status == transaction.ITOTriggerContract_ActiveITO,
					})
				}
			case transaction.ITOTriggerContract_AddToWhitelist:
				if statusOK && alteredIto != nil {
					alteredIto.Add(string(ITOTriggerContract.AssetID), &data.AlteredITOs{
						AddedAddresses: convertedWhiteList,
					})
				}
			case transaction.ITOTriggerContract_RemoveFromWhitelist:
				if statusOK && alteredIto != nil {
					alteredIto.Add(string(ITOTriggerContract.AssetID), &data.AlteredITOs{
						RemovedAddresses: convertedWhiteList,
					})
				}
			default:
				if statusOK && alteredIto != nil {
					alteredIto.Add(string(ITOTriggerContract.AssetID), &data.AlteredITOs{
						IsNew: false,
					})
				}
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertTriggerITOContract(ITOTriggerContract))
		case transaction.TXContract_SmartContractType:
			smartContract, err := contract.GetSmartContract()
			if err != nil {
				log.Warn("error decoding SmartContract contract for indexing (will skip tx)", "error", err)
				dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

				continue
			}

			dbTx.Contracts = append(dbTx.Contracts, cm.convertSmartContract(smartContract))
		default:
			log.Warn("error decoding unknown contract for indexing (will skip tx)")
			dbTx.Contracts = append(dbTx.Contracts, convertToGenericContract(contract))

			continue
		}
	}

	return nil
}

func (ei *elasticProcessor) serializeTransactions(
	transactions []*data.Transaction,
	bytesBuff *data.BufferSlice,
) error {
	var err error

	for _, tx := range transactions {
		meta, serializedData, errPrepareTx := prepareSerializedDataForATransaction(tx, txIndex)
		if errPrepareTx != nil {
			log.Warn("error preparing transaction for indexing", "tx hash", tx.Hash, "error", err)
			return errPrepareTx
		}

		err = bytesBuff.PutData(meta, serializedData)
		if err != nil {
			log.Warn("elastic search: serialize bulk tx, write meta", "error", err.Error())
			return err
		}

		// TODO: refactor
		metaContracts, serializedContractsData, errPrepareContracts := prepareSerializedDataForTransactionContracts(tx, txIndex)
		if errPrepareContracts != nil {
			log.Warn("error preparing contracts for indexing", "tx hash", tx.Hash, "error", err)
			return errPrepareContracts
		}

		for _, serializedContract := range serializedContractsData {
			err = bytesBuff.PutData(metaContracts, serializedContract)
			if err != nil {
				log.Warn("elastic search: serializedContract bulk tx, write meta", "error", err.Error())
				return err
			}
		}

	}

	return nil
}

// SerializeLogs will serialize the provided logs in a way that Elasticsearch expects a bulk request
func SerializeLogs(logs []*data.Logs, buffSlice *data.BufferSlice, index string) error {
	for _, lg := range logs {
		meta := []byte(fmt.Sprintf(`{ "update" : { "_index":"%s", "_id" : "%s" } }%s`, index, converters.JsonEscape(lg.ID), "\n"))
		serializedData, errMarshal := json.Marshal(lg)
		if errMarshal != nil {
			return errMarshal
		}

		codeToExecute := `
		if ('create' == ctx.op) {
			ctx._source = params.log
		} else {
			if (ctx._source.containsKey('timestamp')) {
				if (ctx._source.timestamp <= params.log.timestamp) {
					ctx._source = params.log
				}
			} else {
				ctx._source = params.log
			}
		}
`
		serializedDataStr := fmt.Sprintf(`{"scripted_upsert": true, "script": {`+
			`"source": "%s",`+
			`"lang": "painless",`+
			`"params": { "log": %s }},`+
			`"upsert": {}}`,
			converters.FormatPainlessSource(codeToExecute), serializedData,
		)

		err := buffSlice.PutData(meta, []byte(serializedDataStr))
		if err != nil {
			return err
		}
	}

	return nil
}

// SerializeSCDeploys will serialize the provided smart contract deploys in a way that Elasticsearch expects a bulk request
func SerializeSCDeploys(deploys map[string]*data.ScDeployInfo, buffSlice *data.BufferSlice, index string) error {
	for scAddr, deployInfo := range deploys {
		meta := []byte(fmt.Sprintf(`{ "update" : { "_index":"%s", "_id" : "%s" } }%s`, index, converters.JsonEscape(scAddr), "\n"))

		serializedData, err := serializeDeploy(deployInfo)
		if err != nil {
			return err
		}

		err = buffSlice.PutData(meta, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func serializeDeploy(deployInfo *data.ScDeployInfo) ([]byte, error) {
	deployInfo.Upgrades = make([]*data.Upgrade, 0)
	serializedData, errPrepareD := json.Marshal(deployInfo)
	if errPrepareD != nil {
		return nil, errPrepareD
	}

	upgradeData := &data.Upgrade{
		TxHash:    deployInfo.TxHash,
		Upgrader:  deployInfo.Creator,
		Timestamp: deployInfo.Timestamp,
	}
	upgradeSerialized, errPrepareU := json.Marshal(upgradeData)
	if errPrepareU != nil {
		return nil, errPrepareU
	}

	codeToExecute := `
		if (!ctx._source.containsKey('upgrades')) {
			ctx._source.upgrades = [params.elem];
		} else {
			ctx._source.upgrades.add(params.elem);
		}
`
	serializedDataStr := fmt.Sprintf(`{"script": {`+
		`"source": "%s",`+
		`"lang": "painless",`+
		`"params": {"elem": %s}},`+
		`"upsert": %s}`,
		converters.FormatPainlessSource(codeToExecute), string(upgradeSerialized), string(serializedData))

	return []byte(serializedDataStr), nil
}

func serializeAssets(assets []*data.Asset, buffSlice *data.BufferSlice, index string) error {
	for _, asset := range assets {
		meta, serializedData, err := prepareSerializedAssetInfo(asset, index)
		if len(meta) == 0 {
			log.Warn("cannot prepare serializes asset info", "error", err)
			return err
		}

		err = buffSlice.PutData(meta, serializedData)
		if err != nil {
			return err
		}

	}

	return nil
}

func serializeUpdateAssets(assets []*data.Asset, buffSlice *data.BufferSlice, index string) error {
	for _, asset := range assets {
		metaData := []byte(fmt.Sprintf(`{ "update" : { "_index":"%s", "_id" : "%s" } }%s`, index, asset.AssetID, "\n"))

		marshalizedAsset, err := json.Marshal(asset)
		if err != nil {
			log.Debug("indexer: marshal",
				"error", "could not serialize asset, will skip indexing",
				"assetId", asset.AssetID)
			return err
		}

		serializedData := []byte(fmt.Sprintf(`{"doc": %s}`, string(marshalizedAsset)))

		err = buffSlice.PutData(metaData, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func serializeProposals(proposals map[string]*data.Proposal, buffSlice *data.BufferSlice) error {

	for _, proposal := range proposals {
		meta, serializedData, errPrepareProposal := prepareSerializedProposalInfo(proposal, proposalsIndex)
		if len(meta) == 0 {
			log.Warn("cannot prepare serializes proposal info", "error", errPrepareProposal)
			return errPrepareProposal
		}

		// append a newline for each element
		serializedData = append(serializedData, "\n"...)

		err := buffSlice.PutData(meta, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func serializeMarketplaces(marketplaces map[string]*data.Marketplace, buffSlice *data.BufferSlice) error {
	for _, marketplace := range marketplaces {
		meta, serializedData, err := prepareSerializedMarketplaceInfo(marketplace, marketplacesIndex)
		if len(meta) == 0 {
			log.Warn("cannot prepare serializes marketplace info", "error", err)
			return err
		}

		err = buffSlice.PutData(meta, serializedData)
		if err != nil {
			return err
		}

	}

	return nil
}

func serializeITOs(itos map[string]*data.ITOInfo, buffSlice *data.BufferSlice) error {
	for assetId, ito := range itos {
		meta, serializedData, err := prepareSerializedITOInfo(assetId, ito, iTOsIndex)
		if len(meta) == 0 {
			log.Warn("cannot prepare serializes ITOs info", "error", err)
			return err
		}

		err = buffSlice.PutData(meta, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func serializeOrders(orders map[string]*data.Order, buffSlice *data.BufferSlice) error {
	for _, order := range orders {
		meta, serializedData, err := prepareSerializedOrderInfo(order, marketplaceOrdersIndex)
		if len(meta) == 0 {
			log.Warn("cannot prepare serializes order info", "error", err)
			return err
		}

		err = buffSlice.PutData(meta, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func serializeAccounts(accounts map[string]*data.AccountInfo, buffSlice *data.BufferSlice, index string) error {
	for address, acc := range accounts {
		meta, serializedData, err := prepareSerializedAccountInfo(address, acc, index)
		if len(meta) == 0 {
			log.Warn("cannot prepare serializes account info", "error", err)
			return err
		}

		err = buffSlice.PutData(meta, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func serializeAccountsKDA(accounts []*data.AccountKDA, buffSlice *data.BufferSlice) error {
	for _, acc := range accounts {
		meta, serializedData, err := prepareSerializedAccountKDAInfo(acc, accountsKDAIndex)
		if len(meta) == 0 {
			log.Warn("cannot prepare serializes account info", "error", err)
			return err
		}

		err = buffSlice.PutData(meta, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func serializePeersAccounts(peersAccounts []*data.ValidatorAccountInfo, buffSlice *data.BufferSlice) error {
	for _, peerAccount := range peersAccounts {
		meta, serializedData, errPreparePeerAccount := prepareSerializedPeerAccount(peerAccount, peersAccountsIndex)
		if len(meta) == 0 {
			log.Warn("cannot prepare serializes peers accounts", "error", errPreparePeerAccount)
			return errPreparePeerAccount
		}

		err := buffSlice.PutData(meta, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func serializeKDAPools(pools []*data.KDAPoolData, buffSlice *data.BufferSlice, index string) error {
	for _, pool := range pools {
		meta, serializedData, err := prepareSerializedPoolInfo(pool, index)
		if len(meta) == 0 {
			log.Warn("cannot prepare serializes pool info", "error", err)
			return err
		}

		err = buffSlice.PutData(meta, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func serializeUpdateKDAPools(pools []*data.KDAPoolData, buffSlice *data.BufferSlice, index string) error {
	for _, pool := range pools {
		metaData := []byte(fmt.Sprintf(`{ "update" : { "_index":"%s", "_id" : "%s" } }%s`, index, pool.KDA, "\n"))

		marshalizedPool, err := json.Marshal(pool)
		if err != nil {
			log.Debug("indexer: marshal",
				"error", "could not serialize pool, will skip indexing",
				"poolID", pool.KDA)
			return err
		}

		serializedData := []byte(fmt.Sprintf(`{"doc": %s}`, string(marshalizedPool)))

		err = buffSlice.PutData(metaData, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func convertToGenericContract(contract *transaction.TXContract) *data.TXContract {
	return &data.TXContract{
		Type:       contract.GetType(),
		TypeString: contract.GetType().String(),
		Parameter:  contract.GetParameter(),
	}
}

func (cm *commonProcessor) convertTransferContract(transferContract *transaction.TransferContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_TransferContractType,
		TypeString: transaction.TXContract_TransferContractType.String(),
		Parameter: data.TransferContract{
			AssetID:      string(transferContract.AssetID),
			ToAddress:    cm.addressPubkeyConverter.Encode(transferContract.ToAddress),
			Amount:       transferContract.Amount,
			KDARoyalties: transferContract.KDARoyalties,
			KLVRoyalties: transferContract.KLVRoyalties,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertAssetContractInfo(createAssetContract *transaction.CreateAssetContract, issueDate int64) *data.TXContract {
	var propertiesInfo *data.PropertiesInfo
	var attributesInfo *data.AttributesInfo
	var rolesInfo []*data.RolesInfo
	var stakingInfo *data.Staking
	var royaltiesInfo *data.RoyaltiesInfo

	properties := createAssetContract.GetProperties()
	if properties != nil {
		propertiesInfo = &data.PropertiesInfo{
			CanFreeze:      properties.CanFreeze,
			CanWipe:        properties.CanWipe,
			CanPause:       properties.CanPause,
			CanMint:        properties.CanMint,
			CanBurn:        properties.CanBurn,
			CanChangeOwner: properties.CanChangeOwner,
			CanAddRoles:    properties.CanAddRoles,
			LimitTransfer:  properties.LimitTransfer,
		}
	}

	royalties := createAssetContract.GetRoyalties()
	if royalties != nil {
		royaltiesAddress := ""
		if len(royalties.Address) > 0 {
			royaltiesAddress = cm.addressPubkeyConverter.Encode(royalties.Address)
		} else {
			royaltiesAddress = cm.addressPubkeyConverter.Encode(createAssetContract.OwnerAddress)
		}

		royaltiesInfo = &data.RoyaltiesInfo{
			Address:            royaltiesAddress,
			TransferPercentage: make([]*data.RoyaltyDataInfo, 0),
			TransferFixed:      royalties.TransferFixed,
			MarketPercentage:   royalties.MarketPercentage,
			MarketFixed:        royalties.MarketFixed,
			ITOPercentage:      royalties.ITOPercentage,
			ITOFixed:           royalties.ITOFixed,
		}

		for _, royaltyData := range royalties.TransferPercentage {
			royaltiesInfo.TransferPercentage = append(royaltiesInfo.TransferPercentage, &data.RoyaltyDataInfo{
				Amount:     royaltyData.Amount,
				Percentage: royaltyData.Percentage,
			})
		}

		for address, splitRoyaltiesInfo := range royalties.SplitRoyalties {

			var convertedAddress string
			decodedAddress, err := hex.DecodeString(address)
			if err != nil {
				log.Error("can not decode address")
				convertedAddress = address
			} else {
				convertedAddress = cm.addressPubkeyConverter.Encode(decodedAddress)
			}

			royaltiesInfo.SplitRoyalties = append(royaltiesInfo.SplitRoyalties, &data.RoyaltySplitInfo{
				Address:                   convertedAddress,
				PercentTransferPercentage: splitRoyaltiesInfo.PercentTransferPercentage,
				PercentTransferFixed:      splitRoyaltiesInfo.PercentTransferFixed,
				PercentMarketPercentage:   splitRoyaltiesInfo.PercentMarketPercentage,
				PercentMarketFixed:        splitRoyaltiesInfo.PercentMarketFixed,
				PercentITOPercentage:      splitRoyaltiesInfo.PercentITOPercentage,
				PercentITOFixed:           splitRoyaltiesInfo.PercentITOFixed,
			})
		}
	}

	attributes := createAssetContract.GetAttributes()
	if attributes != nil {
		attributesInfo = &data.AttributesInfo{
			IsPaused:                 attributes.IsPaused,
			IsNFTMintStopped:         attributes.IsNFTMintStopped,
			IsRoyaltiesChangeStopped: attributes.IsRoyaltiesChangeStopped,
		}
	}

	staking := createAssetContract.Staking
	if staking != nil {
		stakingInfo = &data.Staking{
			Type:                staking.Type.String(),
			APR:                 staking.GetAPR(),
			MinEpochsToClaim:    staking.GetMinEpochsToClaim(),
			MinEpochsToUnstake:  staking.GetMinEpochsToUnstake(),
			MinEpochsToWithdraw: staking.GetMinEpochsToWithdraw(),
		}
	}

	for _, role := range createAssetContract.Roles {
		roleInfo := &data.RolesInfo{
			Address:             cm.addressPubkeyConverter.Encode(role.Address),
			HasRoleMint:         role.HasRoleMint,
			HasRoleSetITOPrices: role.HasRoleSetITOPrices,
		}
		rolesInfo = append(rolesInfo, roleInfo)
	}

	assetContract := &data.TXContract{
		Type:       transaction.TXContract_CreateAssetContractType,
		TypeString: transaction.TXContract_CreateAssetContractType.String(),
		Parameter: data.CreateAssetContract{
			Type:          createAssetContract.Type.String(),
			Name:          string(createAssetContract.Name),
			Ticker:        string(createAssetContract.Ticker),
			Logo:          createAssetContract.GetLogo(),
			OwnerAddress:  cm.addressPubkeyConverter.Encode(createAssetContract.OwnerAddress),
			URIs:          cm.convertURIs(createAssetContract.URIs),
			Precision:     createAssetContract.Precision,
			InitialSupply: createAssetContract.InitialSupply,
			MaxSupply:     createAssetContract.MaxSupply,
			MintedValue:   createAssetContract.InitialSupply,
			BurnedValue:   0,
			IssueDate:     issueDate,
			Royalties:     royaltiesInfo,
			Properties:    propertiesInfo,
			Attributes:    attributesInfo,
			Staking:       stakingInfo,
			Roles:         rolesInfo,
		},
	}

	return assetContract
}

func (cm *commonProcessor) convertAssetInfo(assetInfo *kapps.KDAData) *data.Asset {
	var propertiesInfo *data.PropertiesInfo
	var attributesInfo *data.AttributesInfo
	var rolesInfo []*data.RolesInfo
	var royaltiesInfo *data.RoyaltiesInfo

	properties := assetInfo.GetProperties()
	if properties != nil {
		propertiesInfo = &data.PropertiesInfo{
			CanFreeze:      properties.CanFreeze,
			CanWipe:        properties.CanWipe,
			CanPause:       properties.CanPause,
			CanMint:        properties.CanMint,
			CanBurn:        properties.CanBurn,
			CanChangeOwner: properties.CanChangeOwner,
			CanAddRoles:    properties.CanAddRoles,
			LimitTransfer:  properties.LimitTransfer,
		}
	}

	attributes := assetInfo.GetAttributes()
	if attributes != nil {
		attributesInfo = &data.AttributesInfo{
			IsPaused:                 attributes.IsPaused,
			IsNFTMintStopped:         attributes.IsNFTMintStopped,
			IsRoyaltiesChangeStopped: attributes.IsRoyaltiesChangeStopped,
		}
	}

	royalties := assetInfo.GetRoyalties()
	if royalties != nil {
		addr := ""
		if len(royalties.Address) > 0 {
			addr = cm.addressPubkeyConverter.Encode(royalties.Address)
		}
		royaltiesInfo = &data.RoyaltiesInfo{
			Address:            addr,
			TransferPercentage: make([]*data.RoyaltyDataInfo, 0),
			TransferFixed:      royalties.TransferFixed,
			MarketPercentage:   royalties.MarketPercentage,
			MarketFixed:        royalties.MarketFixed,
			ITOPercentage:      royalties.ITOPercentage,
			ITOFixed:           royalties.ITOFixed,
		}

		for _, royaltyData := range royalties.TransferPercentage {
			royaltiesInfo.TransferPercentage = append(royaltiesInfo.TransferPercentage, &data.RoyaltyDataInfo{
				Amount:     royaltyData.Amount,
				Percentage: royaltyData.Percentage,
			})
		}

		for address, splitRoyaltiesInfo := range royalties.SplitRoyalties {
			var convertedAddress string
			decodedAddress, err := hex.DecodeString(address)
			if err != nil {
				log.Error("can not decode address")
				convertedAddress = address
			} else {
				convertedAddress = cm.addressPubkeyConverter.Encode(decodedAddress)
			}

			royaltiesInfo.SplitRoyalties = append(royaltiesInfo.SplitRoyalties, &data.RoyaltySplitInfo{
				Address:                   convertedAddress,
				PercentTransferPercentage: splitRoyaltiesInfo.PercentTransferPercentage,
				PercentTransferFixed:      splitRoyaltiesInfo.PercentTransferFixed,
				PercentMarketPercentage:   splitRoyaltiesInfo.PercentMarketPercentage,
				PercentMarketFixed:        splitRoyaltiesInfo.PercentMarketFixed,
				PercentITOPercentage:      splitRoyaltiesInfo.PercentITOPercentage,
				PercentITOFixed:           splitRoyaltiesInfo.PercentITOFixed,
			})
		}
	}

	for _, role := range assetInfo.Roles {
		roleInfo := &data.RolesInfo{
			Address:             cm.addressPubkeyConverter.Encode(role.Address),
			HasRoleMint:         role.HasRoleMint,
			HasRoleSetITOPrices: role.HasRoleSetITOPrices,
		}
		rolesInfo = append(rolesInfo, roleInfo)
	}

	ownerAddr := ""
	if len(assetInfo.OwnerAddress) > 0 {
		ownerAddr = cm.addressPubkeyConverter.Encode(assetInfo.OwnerAddress)
	}

	asset := &data.Asset{
		AssetType:         assetInfo.AssetType.String(),
		AssetID:           string(assetInfo.ID),
		Name:              string(assetInfo.Name),
		Ticker:            string(assetInfo.Ticker),
		OwnerAddress:      ownerAddr,
		URIs:              cm.convertURIs(assetInfo.URIs),
		Logo:              assetInfo.Logo,
		Precision:         assetInfo.Precision,
		InitialSupply:     assetInfo.InitialSupply,
		CirculatingSupply: assetInfo.CirculatingSupply,
		MaxSupply:         assetInfo.MaxSupply,
		MintedValue:       assetInfo.MintedValue,
		BurnedValue:       assetInfo.BurnedValue,
		Royalties:         royaltiesInfo,
		Properties:        propertiesInfo,
		Attributes:        attributesInfo,
		Roles:             rolesInfo,
		IssueDate:         assetInfo.IssueDate,
	}

	return asset
}

func (cm *commonProcessor) convertStakingData(stakingData *kapps.StakingData) *data.StakingData {
	staking := &data.StakingData{
		InterestType:        stakingData.GetInterestType().String(),
		APR:                 []*data.APRData{},
		FPR:                 []*data.FPRData{},
		CurrentFPRAmount:    stakingData.GetCurrentFPRAmount(),
		TotalStaked:         stakingData.GetTotalStaked(),
		MinEpochsToClaim:    stakingData.GetMinEpochsToClaim(),
		MinEpochsToUnstake:  stakingData.GetMinEpochsToUnstake(),
		MinEpochsToWithdraw: stakingData.GetMinEpochsToWithdraw(),
	}

	for _, apr := range stakingData.APR {
		aprInfo := &data.APRData{
			Timestamp: apr.GetTimestamp(),
			Epoch:     apr.GetEpoch(),
			Value:     apr.GetValue(),
		}
		staking.APR = append(staking.APR, aprInfo)
	}

	for _, fpr := range stakingData.FPR {

		kdaSlice := make([]*data.KDAFPRData, 0)
		if fpr.KDAS != nil {
			for kdaInfo, v := range fpr.KDAS {
				kdaSlice = append(kdaSlice, &data.KDAFPRData{
					KDA:          kdaInfo,
					TotalAmount:  v.GetTotalAmount(),
					TotalClaimed: v.GetTotalClaimed(),
				})
			}
		}

		fprInfo := &data.FPRData{
			TotalAmount:  fpr.GetTotalAmount(),
			TotalStaked:  fpr.GetTotalStaked(),
			Epoch:        fpr.GetEpoch(),
			TotalClaimed: fpr.GetTotalClaimed(),
			KDA:          kdaSlice,
		}

		staking.FPR = append(staking.FPR, fprInfo)
	}

	return staking
}

func (cm *commonProcessor) convertCreateValidatorContract(createValidatorContract *transaction.CreateValidatorContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_CreateValidatorContractType,
		TypeString: transaction.TXContract_CreateValidatorContractType.String(),
		Parameter: data.CreateValidatorContract{
			OwnerAddress: cm.addressPubkeyConverter.Encode(createValidatorContract.OwnerAddress),
			Config: data.ValidatorConfig{
				BLSPublicKey:        hex.EncodeToString(createValidatorContract.Config.BLSPublicKey),
				RewardAddress:       cm.addressPubkeyConverter.Encode(createValidatorContract.Config.RewardAddress),
				CanDelegate:         createValidatorContract.Config.CanDelegate,
				Commission:          createValidatorContract.Config.Commission,
				MaxDelegationAmount: createValidatorContract.Config.MaxDelegationAmount,
				Logo:                createValidatorContract.Config.Logo,
				URIs:                cm.convertURIs(createValidatorContract.Config.URIs),
				Name:                createValidatorContract.Config.Name,
			},
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertValidatorConfigContract(validatorConfigContract *transaction.ValidatorConfigContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_ValidatorConfigContractType,
		TypeString: transaction.TXContract_ValidatorConfigContractType.String(),
		Parameter: data.ValidatorConfigContract{
			Config: data.ValidatorConfig{
				BLSPublicKey:        hex.EncodeToString(validatorConfigContract.Config.BLSPublicKey),
				RewardAddress:       cm.addressPubkeyConverter.Encode(validatorConfigContract.Config.RewardAddress),
				CanDelegate:         validatorConfigContract.Config.CanDelegate,
				Commission:          validatorConfigContract.Config.Commission,
				MaxDelegationAmount: validatorConfigContract.Config.MaxDelegationAmount,
				Logo:                validatorConfigContract.Config.Logo,
				URIs:                cm.convertURIs(validatorConfigContract.Config.URIs),
				Name:                validatorConfigContract.Config.Name,
			},
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertFreezeContract(freezeContract *transaction.FreezeContract) *data.TXContract {
	assetID := "KLV"
	if len(freezeContract.AssetID) > 0 {
		assetID = string(freezeContract.AssetID)
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_FreezeContractType,
		TypeString: transaction.TXContract_FreezeContractType.String(),
		Parameter: data.FreezeContract{
			Amount:  freezeContract.Amount,
			AssetID: assetID,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertUnfreezeContract(unfreezeContract *transaction.UnfreezeContract) *data.TXContract {
	assetID := "KLV"
	if len(unfreezeContract.AssetID) > 0 {
		assetID = string(unfreezeContract.AssetID)
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_UnfreezeContractType,
		TypeString: transaction.TXContract_UnfreezeContractType.String(),
		Parameter: data.UnfreezeContract{
			AssetID:  assetID,
			BucketID: hex.EncodeToString(unfreezeContract.BucketID),
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertDelegateContract(delegateContract *transaction.DelegateContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_DelegateContractType,
		TypeString: transaction.TXContract_DelegateContractType.String(),
		Parameter: data.DelegateContract{
			ToAddress: cm.addressPubkeyConverter.Encode(delegateContract.ToAddress),
			BucketID:  hex.EncodeToString(delegateContract.BucketID),
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertUndelegateContract(undelegateContract *transaction.UndelegateContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_UndelegateContractType,
		TypeString: transaction.TXContract_UndelegateContractType.String(),
		Parameter: data.UndelegateContract{
			BucketID: hex.EncodeToString(undelegateContract.BucketID),
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertWithdrawContract(withdrawContract *transaction.WithdrawContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_WithdrawContractType,
		TypeString: transaction.TXContract_WithdrawContractType.String(),
		Parameter: data.WithdrawContract{
			AssetID:            string(withdrawContract.AssetID),
			WithdrawTypeString: withdrawContract.WithdrawType.String(),
			WithdrawType:       withdrawContract.WithdrawType,
			Amount:             withdrawContract.Amount,
			CurrencyID:         string(withdrawContract.CurrencyID),
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertDepositContract(depositContract *transaction.DepositContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_DepositContractType,
		TypeString: transaction.TXContract_DepositContractType.String(),
		Parameter: data.DepositContract{
			ID:                string(depositContract.ID),
			DepositTypeString: depositContract.DepositType.String(),
			DepositType:       depositContract.DepositType,
			Amount:            depositContract.Amount,
			CurrencyID:        string(depositContract.CurrencyID),
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertClaimContract(claimContract *transaction.ClaimContract) *data.TXContract {
	id := string(claimContract.ID)
	if claimContract.ClaimType == transaction.ClaimContract_MarketClaim {
		id = hex.EncodeToString(claimContract.ID)
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_ClaimContractType,
		TypeString: transaction.TXContract_ClaimContractType.String(),
		Parameter: data.ClaimContract{
			ClaimType: claimContract.ClaimType.String(),
			ID:        id,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertAssetTriggerContract(assetTriggerContract *transaction.AssetTriggerContract) *data.TXContract {
	var roleInfo *data.RolesInfo
	var stakingInfo *data.Staking
	var kdaPoolInfo *data.KDAPool
	var royaltiesInfo *data.RoyaltiesInfo

	if assetTriggerContract.Role != nil && len(assetTriggerContract.Role.Address) > 0 {
		roleInfo = &data.RolesInfo{
			Address:             cm.addressPubkeyConverter.Encode(assetTriggerContract.Role.Address),
			HasRoleMint:         assetTriggerContract.Role.HasRoleMint,
			HasRoleSetITOPrices: assetTriggerContract.Role.HasRoleSetITOPrices,
		}
	}

	if assetTriggerContract.Staking != nil {
		stakingInfo = &data.Staking{
			Type:                assetTriggerContract.Staking.Type.String(),
			APR:                 assetTriggerContract.Staking.APR,
			MinEpochsToClaim:    assetTriggerContract.Staking.MinEpochsToClaim,
			MinEpochsToUnstake:  assetTriggerContract.Staking.MinEpochsToUnstake,
			MinEpochsToWithdraw: assetTriggerContract.Staking.MinEpochsToWithdraw,
		}
	}

	if assetTriggerContract.KDAPool != nil {
		kdaPoolInfo = &data.KDAPool{
			KDA:          string(assetTriggerContract.AssetID),
			Active:       assetTriggerContract.KDAPool.Active,
			AdminAddress: cm.addressPubkeyConverter.Encode(assetTriggerContract.KDAPool.AdminAddress),
			FRatioKLV:    assetTriggerContract.KDAPool.FRatioKLV,
			FRatioKDA:    assetTriggerContract.KDAPool.FRatioKDA,
		}
	}

	toAddress := ""
	if len(assetTriggerContract.ToAddress) > 0 {
		toAddress = cm.addressPubkeyConverter.Encode(assetTriggerContract.ToAddress)
	}

	royalties := assetTriggerContract.GetRoyalties()
	if royalties != nil {
		royaltiesAddress := ""
		if len(royalties.Address) > 0 {
			royaltiesAddress = cm.addressPubkeyConverter.Encode(royalties.Address)
		}

		royaltiesInfo = &data.RoyaltiesInfo{
			Address:            royaltiesAddress,
			TransferPercentage: make([]*data.RoyaltyDataInfo, 0),
			TransferFixed:      royalties.TransferFixed,
			MarketPercentage:   royalties.MarketPercentage,
			MarketFixed:        royalties.MarketFixed,
			ITOPercentage:      royalties.ITOPercentage,
			ITOFixed:           royalties.ITOFixed,
		}

		for _, royaltyData := range royalties.TransferPercentage {
			royaltiesInfo.TransferPercentage = append(royaltiesInfo.TransferPercentage, &data.RoyaltyDataInfo{
				Amount:     royaltyData.Amount,
				Percentage: royaltyData.Percentage,
			})
		}

		for address, splitRoyaltiesInfo := range royalties.SplitRoyalties {

			var convertedAddress string
			decodedAddress, err := hex.DecodeString(address)
			if err != nil {
				log.Error("can not decode address")
				convertedAddress = address
			} else {
				convertedAddress = cm.addressPubkeyConverter.Encode(decodedAddress)
			}

			royaltiesInfo.SplitRoyalties = append(royaltiesInfo.SplitRoyalties, &data.RoyaltySplitInfo{
				Address:                   convertedAddress,
				PercentTransferPercentage: splitRoyaltiesInfo.PercentTransferPercentage,
				PercentTransferFixed:      splitRoyaltiesInfo.PercentTransferFixed,
				PercentMarketPercentage:   splitRoyaltiesInfo.PercentMarketPercentage,
				PercentMarketFixed:        splitRoyaltiesInfo.PercentMarketFixed,
				PercentITOPercentage:      splitRoyaltiesInfo.PercentITOPercentage,
				PercentITOFixed:           splitRoyaltiesInfo.PercentITOFixed,
			})
		}
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_AssetTriggerContractType,
		TypeString: transaction.TXContract_AssetTriggerContractType.String(),
		Parameter: data.AssetTriggerContract{
			TriggerType: assetTriggerContract.TriggerType.String(),
			AssetID:     string(assetTriggerContract.AssetID),
			ToAddress:   toAddress,
			Amount:      assetTriggerContract.Amount,
			MIME:        string(assetTriggerContract.MIME),
			Logo:        assetTriggerContract.GetLogo(),
			URIs:        cm.convertURIs(assetTriggerContract.URIs),
			Royalties:   royaltiesInfo,
			Role:        roleInfo,
			Staking:     stakingInfo,
			KDAPool:     kdaPoolInfo,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertSetAccountNameContract(setAccountNameContract *transaction.SetAccountNameContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_SetAccountNameContractType,
		TypeString: transaction.TXContract_SetAccountNameContractType.String(),
		Parameter: data.SetAccountNameContract{
			Name: string(setAccountNameContract.Name),
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertProposalContract(proposalContract *transaction.ProposalContract) *data.TXContract {
	parameters := make(map[int32]string)
	for key := range proposalContract.Parameters {
		parameters[key] = string(proposalContract.Parameters[key])
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_ProposalContractType,
		TypeString: transaction.TXContract_ProposalContractType.String(),
		Parameter: data.ProposalContract{
			Parameters:     parameters,
			Description:    string(proposalContract.GetDescription()),
			EpochsDuration: proposalContract.GetEpochsDuration(),
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertProposal(proposalID uint64, proposalData *kapps.ProposalData, oldParametersBytes map[int32][]byte) *data.Proposal {
	parameters := make(map[int32]string, len(proposalData.Parameters))
	for key := range proposalData.Parameters {
		parameters[key] = string(proposalData.Parameters[key])
	}

	oldParameters := make(map[int32]string, len(proposalData.Parameters))
	for key := range proposalData.Parameters {
		oldParameters[key] = string(oldParametersBytes[key])
	}

	convertedProposal := &data.Proposal{
		ProposalID:     proposalID,
		Proposer:       cm.addressPubkeyConverter.Encode(proposalData.Proposer),
		TXHash:         hex.EncodeToString(proposalData.TXHash),
		ProposalStatus: proposalData.ProposalStatus.String(),
		Timestamp:      time.Duration(proposalData.Timestamp * 1000),
		Parameters:     parameters,
		OldParameters:  oldParameters,
		Description:    string(proposalData.Description),
		EpochStart:     proposalData.EpochStart,
		EpochEnd:       proposalData.EpochEnd,
		Votes:          make(map[int32]int64),
		Voters:         make([]*data.VotersInfo, 0),
		TotalStaked:    proposalData.TotalStaked,
	}

	for voteType, amount := range proposalData.Votes {
		convertedProposal.Votes[voteType] = amount
	}

	for voter, detail := range proposalData.Voters {
		addr, err := hex.DecodeString(voter)
		// use hex encodeed if error decoding data
		if err == nil {
			voter = cm.addressPubkeyConverter.Encode(addr)
		}

		convertedProposal.Voters = append(convertedProposal.Voters, &data.VotersInfo{
			Address:   voter,
			Type:      int32(detail.Type),
			Amount:    detail.Amount,
			Timestamp: time.Duration(detail.Timestamp * 1000),
		})
	}

	return convertedProposal
}

func (cm *commonProcessor) convertMarketplace(marketplaceID string, marketplaceData *kapps.Marketplace) *data.Marketplace {
	convertedMarketplace := &data.Marketplace{
		ID:                 marketplaceID,
		Name:               string(marketplaceData.GetName()),
		OwnerAddress:       cm.addressPubkeyConverter.Encode(marketplaceData.OwnerAddress),
		ReferralAddress:    cm.addressPubkeyConverter.Encode(marketplaceData.ReferralAddress),
		ReferralPercentage: marketplaceData.ReferralPercentage,
	}

	return convertedMarketplace
}

func (cm *commonProcessor) convertVoteContract(voteContract *transaction.VoteContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_VoteContractType,
		TypeString: transaction.TXContract_VoteContractType.String(),
		Parameter: data.VoteContract{
			Type:       voteContract.GetType().String(),
			ProposalID: voteContract.GetProposalID(),
			Amount:     voteContract.GetAmount(),
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertAccountBuckets(buckets map[string]*kapps.UserBucket) []data.UserKDABucket {
	convertedBuckets := make([]data.UserKDABucket, 0)

	for bucketId := range buckets {
		delegation := ""
		if len(buckets[bucketId].GetDelegation()) == cm.addressPubkeyConverter.Len() {
			delegation = cm.addressPubkeyConverter.Encode((buckets[bucketId].GetDelegation()))
		}
		convertedBucket := data.UserKDABucket{
			Id:            bucketId,
			StakedAt:      buckets[bucketId].GetStakedAt(),
			StakedEpoch:   buckets[bucketId].GetStakedEpoch(),
			UnstakedEpoch: buckets[bucketId].GetUnstakedEpoch(),
			Value:         buckets[bucketId].GetValue(),
			Delegation:    delegation,
		}
		convertedBuckets = append(convertedBuckets, convertedBucket)
	}

	return convertedBuckets
}

func convertAccountLastClaim(lastClaimInfo *kapps.LastClaim) data.UserKDALastClaim {
	convertedLastClaim := data.UserKDALastClaim{}
	if lastClaimInfo != nil {
		convertedLastClaim.Timestamp = lastClaimInfo.GetTimestamp()
		convertedLastClaim.Epoch = lastClaimInfo.GetEpoch()
	}

	return convertedLastClaim
}

func (cm *commonProcessor) convertConfigITOContract(configITOContract *transaction.ConfigITOContract) *data.TXContract {
	convertedPacksInfo := []*data.PackInfo{}
	for key, packInfo := range configITOContract.PackInfo {
		var convertedPackInfo = &data.PackInfo{
			Key:   key,
			Packs: []*data.PackItem{},
		}
		for _, packItem := range packInfo.Packs {
			convertedPackInfo.Packs = append(convertedPackInfo.Packs, &data.PackItem{
				Amount: packItem.Amount,
				Price:  packItem.Price,
			})
		}
		convertedPacksInfo = append(convertedPacksInfo, convertedPackInfo)
	}

	convertedWhitelistInfo := []*data.WhitelistInfo{}
	for key, whitelistInfo := range configITOContract.WhitelistInfo {
		finalAddress := key
		decodedAddress, err := hex.DecodeString(key)
		if err != nil {
			log.Error("can not decode address")
		} else {
			finalAddress = cm.addressPubkeyConverter.Encode(decodedAddress)
		}

		convertedWhitelistInfo = append(convertedWhitelistInfo, &data.WhitelistInfo{
			Address: finalAddress,
			Limit:   whitelistInfo.Limit,
		})
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_ConfigITOContractType,
		TypeString: transaction.TXContract_ConfigITOContractType.String(),
		Parameter: data.ConfigITOContract{
			AssetID:                string(configITOContract.AssetID),
			ReceiverAddress:        cm.addressPubkeyConverter.Encode(configITOContract.ReceiverAddress),
			Status:                 configITOContract.Status.String(),
			MaxAmount:              configITOContract.MaxAmount,
			PackInfo:               convertedPacksInfo,
			DefaultLimitPerAddress: configITOContract.DefaultLimitPerAddress,
			WhitelistStatus:        configITOContract.WhitelistStatus.String(),
			WhitelistInfo:          convertedWhitelistInfo,
			WhitelistStartTime:     configITOContract.WhitelistStartTime,
			WhitelistEndTime:       configITOContract.WhitelistEndTime,
			StartTime:              configITOContract.StartTime,
			EndTime:                configITOContract.EndTime,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertTriggerITOContract(triggerITOContract *transaction.ITOTriggerContract) *data.TXContract {

	receiverAddress := ""
	if triggerITOContract.ReceiverAddress != nil {
		receiverAddress = cm.addressPubkeyConverter.Encode(triggerITOContract.ReceiverAddress)
	}

	convertedPacksInfo := []*data.PackInfo{}
	for key, packInfo := range triggerITOContract.PackInfo {
		var convertedPackInfo = &data.PackInfo{
			Key:   key,
			Packs: []*data.PackItem{},
		}
		for _, packItem := range packInfo.Packs {
			convertedPackInfo.Packs = append(convertedPackInfo.Packs, &data.PackItem{
				Amount: packItem.Amount,
				Price:  packItem.Price,
			})
		}
		convertedPacksInfo = append(convertedPacksInfo, convertedPackInfo)
	}

	convertedWhitelistInfo := []*data.WhitelistInfo{}
	for key, whitelistInfo := range triggerITOContract.WhitelistInfo {
		finalAddress := key
		decodedAddress, err := hex.DecodeString(key)
		if err != nil {
			log.Error("can not decode address")
		} else {
			finalAddress = cm.addressPubkeyConverter.Encode(decodedAddress)
		}

		convertedWhitelistInfo = append(convertedWhitelistInfo, &data.WhitelistInfo{
			Address: finalAddress,
			Limit:   whitelistInfo.Limit,
		})
	}

	ITOStatus := ""
	if triggerITOContract.TriggerType == transaction.ITOTriggerContract_UpdateStatus {
		ITOStatus = triggerITOContract.Status.String()
	}

	whitelistStatus := ""
	if triggerITOContract.TriggerType == transaction.ITOTriggerContract_UpdateStatus {
		whitelistStatus = triggerITOContract.WhitelistStatus.String()
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_ITOTriggerContractType,
		TypeString: transaction.TXContract_ITOTriggerContractType.String(),
		Parameter: data.ITOTriggerContract{
			TriggerType:            triggerITOContract.TriggerType.String(),
			AssetID:                string(triggerITOContract.AssetID),
			ReceiverAddress:        receiverAddress,
			Status:                 ITOStatus,
			MaxAmount:              triggerITOContract.MaxAmount,
			PackInfo:               convertedPacksInfo,
			DefaultLimitPerAddress: triggerITOContract.DefaultLimitPerAddress,
			WhitelistStatus:        whitelistStatus,
			WhitelistInfo:          convertedWhitelistInfo,
			WhitelistStartTime:     triggerITOContract.WhitelistStartTime,
			WhitelistEndTime:       triggerITOContract.WhitelistEndTime,
			StartTime:              triggerITOContract.StartTime,
			EndTime:                triggerITOContract.EndTime,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertSetITOPricesContract(setITOPricesContract *transaction.SetITOPricesContract) *data.TXContract {
	var convertedPacksInfo []*data.PackInfo
	for key, packInfo := range setITOPricesContract.PackInfo {
		var convertedPackInfo = &data.PackInfo{
			Key:   key,
			Packs: []*data.PackItem{},
		}
		for _, packItem := range packInfo.Packs {
			convertedPackInfo.Packs = append(convertedPackInfo.Packs, &data.PackItem{
				Amount: packItem.Amount,
				Price:  packItem.Price,
			})
		}
		convertedPacksInfo = append(convertedPacksInfo, convertedPackInfo)
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_SetITOPricesContractType,
		TypeString: transaction.TXContract_SetITOPricesContractType.String(),
		Parameter: data.SetITOPricesContract{
			AssetID:  string(setITOPricesContract.GetAssetID()),
			PackInfo: convertedPacksInfo,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertBuyContract(buyContract *transaction.BuyContract) *data.TXContract {
	id := string(buyContract.ID)
	if buyContract.BuyType == transaction.BuyContract_MarketBuy {
		id = hex.EncodeToString(buyContract.ID)
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_BuyContractType,
		TypeString: transaction.TXContract_BuyContractType.String(),
		Parameter: data.BuyContract{
			BuyType:    buyContract.BuyType.String(),
			ID:         id,
			CurrencyID: string(buyContract.CurrencyID),
			Amount:     buyContract.Amount,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertSellContract(sellContract *transaction.SellContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_SellContractType,
		TypeString: transaction.TXContract_SellContractType.String(),
		Parameter: data.SellContract{
			MarketType:    sellContract.MarketType.Enum().String(),
			MarketplaceID: hex.EncodeToString(sellContract.MarketplaceID),
			AssetID:       string(sellContract.AssetID),
			CurrencyID:    string(sellContract.CurrencyID),
			Price:         sellContract.Price,
			ReservePrice:  sellContract.ReservePrice,
			EndTime:       sellContract.EndTime,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertCancelMarketOrderContract(cancelMarketOrderContract *transaction.CancelMarketOrderContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_CancelMarketOrderContractType,
		TypeString: transaction.TXContract_CancelMarketOrderContractType.String(),
		Parameter: data.CancelMarketOrderContract{
			OrderID: hex.EncodeToString(cancelMarketOrderContract.OrderID),
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertCreateMarketplaceContract(createMarketplaceContract *transaction.CreateMarketplaceContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_CreateMarketplaceContractType,
		TypeString: transaction.TXContract_CreateMarketplaceContractType.String(),
		Parameter: data.CreateMarketplaceContract{
			Name:               string(createMarketplaceContract.Name),
			ReferralAddress:    cm.addressPubkeyConverter.Encode(createMarketplaceContract.ReferralAddress),
			ReferralPercentage: createMarketplaceContract.ReferralPercentage,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertConfigMarketplaceContract(configMarketplaceContract *transaction.ConfigMarketplaceContract) *data.TXContract {
	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_ConfigMarketplaceContractType,
		TypeString: transaction.TXContract_ConfigMarketplaceContractType.String(),
		Parameter: data.ConfigMarketplaceContract{
			MarketplaceID:      hex.EncodeToString(configMarketplaceContract.MarketplaceID),
			Name:               string(configMarketplaceContract.Name),
			ReferralAddress:    cm.addressPubkeyConverter.Encode(configMarketplaceContract.ReferralAddress),
			ReferralPercentage: configMarketplaceContract.ReferralPercentage,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertUpdateAccountPermissionContract(updateAccountPermissionContract *transaction.UpdateAccountPermissionContract) *data.TXContract {
	permissions := make([]data.AccPermission, 0)

	for _, p := range updateAccountPermissionContract.Permissions {
		keys := make([]data.AccKey, 0)
		for _, k := range p.Signers {
			keys = append(keys, data.AccKey{
				Address: cm.addressPubkeyConverter.Encode(k.Address),
				Weight:  k.Weight,
			})
		}
		permissions = append(permissions, data.AccPermission{
			Type:           int32(p.Type),
			PermissionName: p.PermissionName,
			Threshold:      p.Threshold,
			Operations:     hex.EncodeToString(p.Operations),
			Signers:        keys,
		})
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_UpdateAccountPermissionContractType,
		TypeString: transaction.TXContract_UpdateAccountPermissionContractType.String(),
		Parameter: data.UpdateAccountPermissionContract{
			Permissions: permissions,
		},
	}

	return convertedContract
}

func (cm *commonProcessor) convertURIs(uris map[string]string) []*data.URI {
	convertedURIs := []*data.URI{}

	for key, value := range uris {
		convertedURIs = append(convertedURIs, &data.URI{
			Key:   key,
			Value: value,
		})
	}

	return convertedURIs
}

func (cm *commonProcessor) convertKDAPoolData(kdaPool *kdafeespool.KDAFeesPoolData) *data.KDAPoolData {
	return &data.KDAPoolData{
		OwnerAddress:  cm.addressPubkeyConverter.Encode(kdaPool.OwnerAddress),
		KDA:           string(kdaPool.KDA),
		Active:        kdaPool.Active,
		KLVBalance:    kdaPool.KLVBalance,
		KDABalance:    kdaPool.KDABalance,
		ConvertedFees: kdaPool.ConvertedFees,
		AdminAddress:  cm.addressPubkeyConverter.Encode(kdaPool.AdminAddress),
		FRatioKLV:     kdaPool.FRatioKLV,
		FRatioKDA:     kdaPool.FRatioKDA,
	}
}

func (cm *commonProcessor) convertSmartContract(smartContract *transaction.SmartContract) *data.TXContract {
	address := ""
	if len(smartContract.Address) > 0 {
		address = cm.addressPubkeyConverter.Encode(smartContract.Address)
	}

	callValue := make([]data.SCCallValue, len(smartContract.CallValue))
	i := 0
	for key, value := range smartContract.CallValue {
		callValue[i] = data.SCCallValue{
			Asset:        key,
			Value:        value.Amount,
			KDARoyalties: value.KDARoyalties,
			KLVRoyalties: value.KLVRoyalties,
		}
		i++
	}

	convertedContract := &data.TXContract{
		Type:       transaction.TXContract_SmartContractType,
		TypeString: transaction.TXContract_SmartContractType.String(),
		Parameter: data.SmartContract{
			Type:      smartContract.GetType().String(),
			TypeValue: int32(smartContract.GetType()),
			Address:   address,
			CallValue: callValue,
		},
	}

	return convertedContract
}

func serializeAccountsHistory(accounts map[string]*data.AccountBalanceHistory, buffSlice *data.BufferSlice, index string) error {
	for address, acc := range accounts {
		meta, serializedData, err := prepareSerializedAccountBalanceHistory(address, acc, index)
		if err != nil {
			log.Warn("cannot prepare serializes account balance history", "error", err)
			return err
		}

		err = buffSlice.PutData(meta, serializedData)
		if err != nil {
			log.Warn("cannot put data to buffer slice", "error", err)
			return err
		}
	}

	return nil
}

func prepareSerializedAccountBalanceHistory(address string, account *data.AccountBalanceHistory, index string) ([]byte, []byte, error) {
	meta := []byte(fmt.Sprintf(`{ "index" : { "_index":"%s", "_id" : "%s" } }%s`, index, address, "\n"))
	serializedData, err := json.Marshal(account)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize account history entry, will skip indexing",
			"address", address)
		return nil, nil, err
	}

	return meta, serializedData, nil
}

func prepareSerializedDataForATransaction(
	tx *data.Transaction,
	index string,
) ([]byte, []byte, error) {
	marshaledTx, err := json.Marshal(tx)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize transaction, will skip indexing",
			"tx hash", tx.Hash)
		return nil, nil, err
	}

	meta := []byte(fmt.Sprintf(`{ "update" : { "_index":"%s", "_id" : "%s" } }%s`, index, tx.Hash, "\n"))
	log.Trace("indexer tx:", "meta", string(meta), "marshaledTx", string(marshaledTx))

	upsertScript := []byte(fmt.Sprintf(`{"script":{"source":"ctx._source.hash = '%s';","lang": "painless"},"upsert":%s}`, tx.Hash, string(marshaledTx)))
	return meta, upsertScript, nil
}

func prepareSerializedDataForTransactionContracts(tx *data.Transaction, index string) ([]byte, [][]byte, error) {
	metaData := []byte(fmt.Sprintf(`{ "update" : { "_index":"%s", "_id" : "%s" } }%s`, index, tx.Hash, "\n"))

	marshaledTx, err := json.Marshal(tx)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize transaction, will skip indexing",
			"tx hash", tx.Hash)
		return nil, nil, err
	}

	var serializedContractsData [][]byte
	for contractIndex, contract := range tx.Contracts {
		marshaledContractParameters, err := json.Marshal(contract.Parameter)
		if err != nil {
			log.Debug("indexer: marshal",
				"error", "could not serialize transaction, will skip indexing",
				"tx hash", tx.Hash)
			return nil, nil, err
		}

		serializedData := []byte(fmt.Sprintf(`{"script":{"source":"`+
			`ctx._source.contract[params.contractIndex].type = params.contractType;`+
			`ctx._source.contract[params.contractIndex].parameter = params.contractParams;`+
			`","lang": "painless","params":`+
			`{"contractIndex": %d, "contractType": %d, "contractParams": %s}},"upsert":%s}`,
			contractIndex, contract.Type, string(marshaledContractParameters), string(marshaledTx)))

		serializedContractsData = append(serializedContractsData, serializedData)
	}

	return metaData, serializedContractsData, nil
}

func serializedDataForUpdateAccounts(accounts map[string]*data.AccountInfo, buffSlice *data.BufferSlice, index string) error {
	for address, acc := range accounts {
		metaData := []byte(fmt.Sprintf(`{ "update" : { "_index":"%s", "_id" : "%s" } }%s`, index, address, "\n"))

		var err error
		pData := []byte("[]")
		if len(acc.Permissions) > 0 {
			pData, err = json.Marshal(acc.Permissions)
			if err != nil {
				return err
			}
		}

		serializedData := []byte(fmt.Sprintf(`{"script":{"source":"`+
			`ctx._source.name = params.name;`+
			`ctx._source.nonce = params.nonce;`+
			`ctx._source.rootHash = params.rootHash;`+
			`ctx._source.balance = params.balance;`+
			`ctx._source.frozenBalance = params.frozenBalance;`+
			`ctx._source.allowance = params.allowance;`+
			`ctx._source.permissions = params.permissions;`+
			`","lang": "painless","params":`+
			`{"name": "%s", "nonce": %d, "rootHash": "%s", "balance": %d, "frozenBalance": %d,"allowance": %d, "permissions": %s}}}`,
			acc.Name, acc.Nonce, acc.RootHash, acc.Balance, acc.FrozenBalance, acc.Allowance, string(pData)))

		err = buffSlice.PutData(metaData, serializedData)
		if err != nil {
			return err
		}
	}

	return nil
}

func prepareSerializedAssetInfo(asset *data.Asset, index string) ([]byte, []byte, error) {
	meta := []byte(fmt.Sprintf(`{ "index" : { "_index":"%s", "_id" : "%s" } }%s`, index, asset.AssetID, "\n"))
	serializedData, err := json.Marshal(asset)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize asset, will skip indexing",
			"asset", asset)
		return nil, nil, err
	}

	return meta, serializedData, nil
}

func prepareSerializedPoolInfo(pool *data.KDAPoolData, index string) ([]byte, []byte, error) {
	meta := []byte(fmt.Sprintf(`{ "index" : { "_index":"%s", "_id" : "%s" } }%s`, index, pool.KDA, "\n"))
	serializedData, err := json.Marshal(pool)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize asset, will skip indexing",
			"pool", pool.KDA)
		return nil, nil, err
	}

	return meta, serializedData, nil
}

func prepareSerializedPeerAccount(peerAccount *data.ValidatorAccountInfo, index string) ([]byte, []byte, error) {
	meta := []byte(fmt.Sprintf(`{ "index" : { "_index":"%s", "_id" : "%s" } }%s`, index, peerAccount.OwnerAddress, "\n"))
	serializedData, err := json.Marshal(peerAccount)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize peer account, will skip indexing",
			"peer account", peerAccount)
		return nil, nil, err
	}

	return meta, serializedData, nil
}

func prepareSerializedProposalInfo(proposal *data.Proposal, index string) ([]byte, []byte, error) {
	meta := []byte(fmt.Sprintf(`{ "index" : { "_index":"%s", "_id" : "%s" } }%s`, index, strconv.FormatUint(proposal.ProposalID, 10), "\n"))
	serializedData, err := json.Marshal(proposal)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize proposal, will skip indexing",
			"proposal", proposal)
		return nil, nil, err
	}

	return meta, serializedData, nil
}

func prepareSerializedMarketplaceInfo(marketplace *data.Marketplace, index string) ([]byte, []byte, error) {
	meta := []byte(fmt.Sprintf(`{ "index" : { "_index":"%s", "_id" : "%s" } }%s`, index, marketplace.ID, "\n"))
	serializedData, err := json.Marshal(marketplace)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize marketplace, will skip indexing",
			"marketplace", marketplace)
		return nil, nil, err
	}

	return meta, serializedData, nil
}

func prepareSerializedITOInfo(assetIt string, ito *data.ITOInfo, index string) ([]byte, []byte, error) {
	meta := []byte(fmt.Sprintf(`{ "index" : { "_index":"%s", "_id" : "%s" } }%s`, index, assetIt, "\n"))
	serializedData, err := json.Marshal(ito)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize ITOs, will skip indexing",
			"ito", ito)
		return nil, nil, err
	}

	return meta, serializedData, nil
}

func prepareSerializedOrderInfo(order *data.Order, index string) ([]byte, []byte, error) {
	meta := []byte(fmt.Sprintf(`{ "index" : { "_index":"%s", "_id" : "%s" } }%s`, index, order.OrderID, "\n"))
	serializedData, err := json.Marshal(order)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize order, will skip indexing",
			"order", order)
		return nil, nil, err
	}

	return meta, serializedData, nil
}

func prepareSerializedAccountInfo(address string, account *data.AccountInfo, index string) ([]byte, []byte, error) {
	meta := []byte(fmt.Sprintf(`{ "index" : { "_index":"%s", "_id" : "%s" } }%s`, index, address, "\n"))
	serializedData, err := json.Marshal(account)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize account, will skip indexing",
			"address", address)
		return nil, nil, err
	}

	return meta, serializedData, nil
}

func prepareSerializedAccountKDAInfo(acc *data.AccountKDA, index string) ([]byte, []byte, error) {
	// delete if balance is zero and it has no buckets
	if len(acc.Buckets) == 0 && acc.Balance == 0 && acc.FrozenBalance == 0 {
		meta := prepareDeleteAccountKDAInfo(acc, index)
		return meta, nil, nil
	}

	meta := []byte(fmt.Sprintf(`{ "index" : { "_index":"%s", "_id" : "%s" } }%s`, index, fmt.Sprintf("%s-%s", acc.AccountAddress, acc.AssetID), "\n"))
	serializedData, err := json.Marshal(acc)
	if err != nil {
		log.Debug("indexer: marshal",
			"error", "could not serialize account, will skip indexing",
			"address", acc.AccountAddress)
		return nil, nil, err
	}

	return meta, serializedData, nil
}

func prepareDeleteAccountKDAInfo(acc *data.AccountKDA, index string) []byte {
	meta := []byte(fmt.Sprintf(`{ "delete" : { "_index":"%s", "_id" : "%s" } }%s`, index, fmt.Sprintf("%s-%s", acc.AccountAddress, acc.AssetID), "\n"))

	return meta
}
