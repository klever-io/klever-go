package indexer

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	ptx "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/converters"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
)

const (
	KLVAssetID = "KLV"
	KFIAssetID = "KFI"
)

type txDatabaseProcessor struct {
	*commonProcessor
	hasher         hashing.Hasher
	marshalizer    marshal.Marshalizer
	isInImportMode bool
}

func newTxDatabaseProcessor(
	hasher hashing.Hasher,
	marshalizer marshal.Marshalizer,
	addressPubkeyConverter core.PubkeyConverter,
	validatorPubkeyConverter core.PubkeyConverter,
	isInImportMode bool,
) *txDatabaseProcessor {
	return &txDatabaseProcessor{
		hasher:      hasher,
		marshalizer: marshalizer,
		commonProcessor: &commonProcessor{
			addressPubkeyConverter:   addressPubkeyConverter,
			validatorPubkeyConverter: validatorPubkeyConverter,
		},
		isInImportMode: isInImportMode,
	}
}

func (tdp *txDatabaseProcessor) prepareTransactionsForDatabase(
	header nodeData.HeaderHandler,
	txPool *indexer.Pool,
) ([]*data.Transaction, map[string]*data.Transaction, *data.AlteredData, error) {
	transactions := make(map[string]*data.Transaction)
	ad := data.NewAlteredData()

	// Always index KLV for mint/burn/FPR in block production
	ad.Assets.Add(string(kdautils.KLVIdentifier), &data.AlteredAsset{
		IsNew: false,
	})

	// Always index KFI for mint/burn/FPR in block production
	ad.Assets.Add(string(kdautils.KFIIdentifier), &data.AlteredAsset{
		IsNew: false,
	})

	txs := getTransactions(txPool)
	for hash, tx := range txs {
		txHash := hex.EncodeToString([]byte(hash))
		dbTx := tdp.BuildTransaction(tx.Transaction, txHash, header)
		// always index sender balance change
		ad.Accounts.Add(dbTx.Sender, &data.AlteredAccount{
			IsSender:      true,
			BalanceChange: true,
		})

		err := tdp.DecodeContract(dbTx, tx.Transaction, ad.Accounts, ad.ITO, ad.SC, header.GetTimestamp())
		if err != nil {
			return nil, nil, nil, err
		}

		tdp.indexReceipt(
			dbTx.Receipts,
			ad,
			header.GetTimestamp(),
			dbTx.Hash,
			dbTx.Sender,
		)

		transactions[hash] = dbTx
		delete(txPool.Txs, hash)
	}

	transactions = tdp.setTransactionSearchOrder(transactions)

	return converters.ConvertMapTxsToSlice(transactions), transactions, ad, nil
}

func (ei *elasticProcessor) prepareAndIndexLogs(logsAndEvents []*nodeData.LogData, txsMap map[string]*data.Transaction, timestamp int64, buffSlice *data.BufferSlice) error {
	if !ei.isIndexEnabled(logsIndex) {
		return nil
	}

	logsDB := ei.logsAndEventsProc.PrepareLogsForDB(logsAndEvents, txsMap, timestamp)

	return SerializeLogs(logsDB, buffSlice, logsIndex)
}

func (ei *elasticProcessor) indexScDeploys(deployData map[string]*data.ScDeployInfo, buffSlice *data.BufferSlice) error {
	if !ei.isIndexEnabled(scDeploysIndex) {
		return nil
	}

	return SerializeSCDeploys(deployData, buffSlice, scDeploysIndex)
}

func (tdp *txDatabaseProcessor) indexReceipt(
	receipts []map[string]interface{},
	ad *data.AlteredData,
	timestamp int64,
	txHash string,
	sender string,
) {
	for _, m := range receipts {
		switch m["type"] {
		case ptx.Transfer:

			assetID := m["assetId"].(string)
			marketplaceId := ""
			if v, ok := m["marketplaceId"]; ok {
				marketplaceId = v.(string)
			}
			orderId := ""
			if v, ok := m["orderId"]; ok {
				orderId = v.(string)
			}

			// if from/to address is Zero Address must index asset
			if m["from"].(string) == ZeroAddressDecoded || m["to"].(string) == ZeroAddressDecoded {
				ad.Assets.Add(m["assetId"].(string), &data.AlteredAsset{
					IsNew: false,
				})
			}

			if len(assetID) == 0 || assetID == KLVAssetID {
				ad.Accounts.Add(m["from"].(string), &data.AlteredAccount{
					IsSender:      true,
					BalanceChange: true,
					MarketplaceID: marketplaceId,
					OrderID:       orderId,
				})

				alteredAccountTo := &data.AlteredAccount{
					IsSender:      false,
					BalanceChange: true,
					MarketplaceID: marketplaceId,
					OrderID:       orderId,
				}

				if m["from"].(string) == ZeroAddressDecoded {
					alteredAccountTo.IsKDAOperation = true
					alteredAccountTo.TokenIdentifier = KLVAssetID
				}

				ad.Accounts.Add(
					m["to"].(string),
					alteredAccountTo,
				)

				continue
			}

			// check if NFT
			assetIDs := strings.Split(assetID, kapps.Sp)
			IsNFTOperation := len(assetIDs) == 2
			TokenIdentifier := string(assetIDs[0])
			NFTNonce := ""
			if IsNFTOperation {
				// format to nonce
				NFTNonce = string(assetIDs[1])
			}

			ad.Accounts.Add(m["from"].(string), &data.AlteredAccount{
				IsSender:        true,
				IsKDAOperation:  true,
				IsNFTOperation:  IsNFTOperation,
				TokenIdentifier: TokenIdentifier,
				NFTNonce:        NFTNonce,
				MarketplaceID:   marketplaceId,
				OrderID:         orderId,
			})
			ad.Accounts.Add(m["to"].(string), &data.AlteredAccount{
				IsSender:        false,
				IsKDAOperation:  true,
				IsNFTOperation:  IsNFTOperation,
				TokenIdentifier: TokenIdentifier,
				NFTNonce:        NFTNonce,
				MarketplaceID:   marketplaceId,
				OrderID:         orderId,
			})

		case ptx.CreateKDA:
			ad.Assets.Add(m["assetId"].(string), &data.AlteredAsset{
				IsNew: false,
			})

		case ptx.UpdateKDA:
			ad.Assets.Add(m["assetId"].(string), &data.AlteredAsset{
				IsNew: true,
			})

		case ptx.Freeze, ptx.Unfreeze:
			assetID := m["assetId"].(string)

			ad.Accounts.Add(m["from"].(string), &data.AlteredAccount{
				IsKDAOperation:  true,
				TokenIdentifier: assetID,
			})

			// need to update the asset when is a FPR freeze or unfreeze
			if assetID != KLVAssetID && assetID != KFIAssetID {
				ad.Assets.Add(assetID, &data.AlteredAsset{
					IsNew: false,
				})
			}

		case ptx.Proposal:
			ad.Proposals.Add(m["proposalId"].(string), &data.AlteredProposal{
				IsNew: true,
			})

		case ptx.ProposalVote:
			ad.Proposals.Add(m["proposalId"].(string), &data.AlteredProposal{
				IsNew: false,
			})

		case ptx.Delegate:
			ad.Accounts.Add(m["from"].(string), &data.AlteredAccount{
				IsSender:        true,
				IsKDAOperation:  true,
				TokenIdentifier: KLVAssetID,
			})

		case ptx.UpdateValidator:
			ad.Accounts.Add(m["id"].(string), &data.AlteredAccount{
				IsSender: false,
			})

		case ptx.ConfigITO:
			ad.Assets.Add(m["assetId"].(string), &data.AlteredAsset{
				IsNew: false,
			})

			_, itoAlreadyAdded := ad.ITO.Get(m["assetId"].(string))
			// only updates by config ito receipt when used by a builtin function
			if !itoAlreadyAdded {
				ad.ITO.Add(m["assetId"].(string), &data.AlteredITOs{
					IsNew: true,
				})
			}

		case ptx.SetITOPrices:
			ad.Assets.Add(m["assetId"].(string), &data.AlteredAsset{
				IsNew: false,
			})

		case ptx.CreateMarketplace:
			ad.Marketplaces.Add(m["marketplaceId"].(string), &data.AlteredMarketplaces{
				IsNew: true,
			})

		case ptx.ConfigMarketplace:
			ad.Marketplaces.Add(m["marketplaceId"].(string), &data.AlteredMarketplaces{
				IsNew: false,
			})

		case ptx.UpdateAccountPermission:
			ad.Accounts.Add(m["address"].(string), &data.AlteredAccount{
				IsSender: false,
			})

		case ptx.SignedBy:
			// nothing to do

		case ptx.Buy:
			ad.Orders.Add(m["orderId"].(string), &data.MarketOperations{
				IsNew:   false,
				IsClaim: false,
				BuyOrder: &data.BuyOrder{
					CurrencyID: m["currencyId"].(string),
					Amount:     m["amount"].(int64),
					Bidder:     m["bidder"].(string),
				},
				Executed:       m["executed"].(bool),
				BuyOrderTxHash: txHash,
			})

		case ptx.Sell:
			ad.Orders.Add(m["orderId"].(string), &data.MarketOperations{
				IsNew:       true,
				OrderTxHash: txHash,
				Timestamp:   timestamp,
			})

		case ptx.Claim:
			a := m["assetId"].(string)
			if len(a) == 0 {
				continue
			}
			b := m["assetIdReceived"].(string)

			ad.Accounts.Add(sender, &data.AlteredAccount{
				IsKDAOperation:  true,
				TokenIdentifier: a,
			})
			ad.Assets.Add(a, &data.AlteredAsset{
				IsNew: false,
			})

			if a == b || len(b) == 0 {
				continue
			}

			ad.Accounts.Add(sender, &data.AlteredAccount{
				IsKDAOperation:  true,
				TokenIdentifier: b,
			})

			ad.Assets.Add(b, &data.AlteredAsset{
				IsNew: false,
			})

			if m["claimTypeString"].(string) == "MarketClaim" {
				ad.Orders.Add(m["orderId"].(string), &data.MarketOperations{
					IsNew:   false,
					IsClaim: true,
				})
			}

		case ptx.Withdraw:
			ad.Accounts.Add(m["from"].(string), &data.AlteredAccount{
				IsKDAOperation:  true,
				TokenIdentifier: m["assetId"].(string),
			})

		case ptx.UpdateMetadata:
			TokenIdentifier := m["assetId"].(string)
			NFTNonce := m["nftNonce"].(string)

			ad.Accounts.Add(m["owner"].(string), &data.AlteredAccount{
				IsSender:        false,
				IsKDAOperation:  true,
				IsNFTOperation:  true,
				TokenIdentifier: TokenIdentifier,
				NFTNonce:        NFTNonce,
			})
		case ptx.UpdateKDAPool:
			if pool, ok := m["poolId"].(string); ok {
				ad.KDAPool.Add(pool, &data.AlteredKDAPool{
					IsNew: true,
				})
			}

			ad.Accounts.Add(sender, &data.AlteredAccount{
				IsSender:        true,
				IsKDAOperation:  true,
				TokenIdentifier: m["poolId"].(string),
			})

		case ptx.UpdateITO:
			asset := m["assetId"].(string)

			if address, ok := m["address"].(string); ok {
				if ito, has := ad.ITO.Get(asset); has {
					alteredAddress, ok := ito.AlteredAddresses[address]
					if !ok {
						alteredAddress = data.AlteredITOAddresses{}
					}
					alteredAddress.NftsBuyed += m["amount"].(int64)
					ito.AlteredAddresses[address] = alteredAddress
				} else {
					ad.ITO.Add(asset, &data.AlteredITOs{
						AlteredAddresses: map[string]data.AlteredITOAddresses{
							address: {NftsBuyed: m["amount"].(int64)},
						},
					})
				}
			}

		case ptx.Deposit:
			ad.Accounts.Add(m["from"].(string), &data.AlteredAccount{
				BalanceChange:   true,
				IsKDAOperation:  true,
				TokenIdentifier: m["currencyId"].(string),
			})

			switch m["depositType"] {
			case "0": // transaction.DepositContract_FPRDeposit
				ad.Assets.Add(m["assetId"].(string), &data.AlteredAsset{
					IsNew: false,
				})
			case "1": // transaction.DepositContract_KDAPool
				ad.KDAPool.Add(m["assetId"].(string), &data.AlteredKDAPool{
					IsNew: false,
				})
			}
		case ptx.CancelOrder:
			ad.Orders.Add(m["orderId"].(string), &data.MarketOperations{
				IsNew:      false,
				IsClaim:    false,
				IsCanceled: true,
			})

		case ptx.SCTrigger:
			ad.Accounts.Add(m["contract"].(string), &data.AlteredAccount{
				IsSmartContract: true,
			})

		case ptx.SetAccountName:
			updatedAddress := m["address"].(string)
			addressBytes, _ := tdp.addressPubkeyConverter.Decode(updatedAddress)

			isSender := updatedAddress == sender
			isSmartContract := core.IsSmartContractAddress(addressBytes)
			ad.Accounts.Add(updatedAddress, &data.AlteredAccount{
				IsSender:        isSender,
				IsSmartContract: isSmartContract,
			})
		default:
			_ = bugsnag.Notify(fmt.Errorf("indexReceipt not able to index. Hash: %s, Type: %v", txHash, m["type"]))
			log.Error("indexReceipt not able to index", "type", m["type"], "txHash", txHash)
		}
	}
}

func (tdp *txDatabaseProcessor) setTransactionSearchOrder(transactions map[string]*data.Transaction) map[string]*data.Transaction {
	currentOrder := uint32(0)
	for _, tx := range transactions {
		tx.SearchOrder = currentOrder
		currentOrder++
	}

	return transactions
}

func getTransactions(txPool *indexer.Pool) map[string]*nodeData.TransactionWithResults {
	transactions := make(map[string]*nodeData.TransactionWithResults)
	for txHash, txHandler := range txPool.Txs {
		tx, ok := txHandler.(*transaction.Transaction)
		if !ok {
			continue
		}

		transactions[txHash] = nodeData.NewTransactionWithResults(tx)
	}

	for _, txLog := range txPool.Logs {
		txWithResults, ok := transactions[txLog.TxHash]
		if !ok {
			continue
		}

		txWithResults.Log = txLog
	}

	return transactions
}
