package indexer

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/logsevents"
	"github.com/klever-io/klever-go/indexer/templates"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memcache"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

const numDecimalsInFloatBalance = 10  // Number of decimals to use when storing balances as float64
const revertAccountBatchSize = 10_000 // Elasticsearch max result window

var ZeroAddressDecoded = "klv1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqpgm89z"

type elasticProcessor struct {
	*txDatabaseProcessor

	elasticClient          DatabaseClientHandler
	parser                 *dataParser
	enabledIndexes         map[string]struct{}
	accountsDB             state.AccountsAdapter
	kappsDB                state.AccountsAdapter
	kappsController        kapp.KAppController
	dividerForDenomination float64
	balancePrecision       float64
	logsAndEventsProc      LogsAndEventsHandler
	cacher                 storage.MemoryCacher
}

// NewElasticProcessor creates an elasticsearch es and handles saving
func NewElasticProcessor(arguments ArgElasticProcessor) (ElasticProcessor, error) {
	err := checkArgElasticProcessor(arguments)
	if err != nil {
		return nil, err
	}

	ei := &elasticProcessor{
		elasticClient: arguments.DBClient,
		parser: &dataParser{
			hasher:      arguments.Hasher,
			marshalizer: arguments.Marshalizer,
		},
		enabledIndexes:         arguments.EnabledIndexes,
		accountsDB:             arguments.AccountsDB,
		kappsDB:                arguments.KappsDB,
		kappsController:        arguments.KAppController,
		balancePrecision:       math.Pow(10, float64(numDecimalsInFloatBalance)),
		dividerForDenomination: math.Pow(10, float64(tools.MaxInt(arguments.Denomination, 0))),
	}

	argsLogsAndEventsProc := logsevents.ArgsLogsAndEventsProcessor{
		PubKeyConverter: arguments.AddressPubkeyConverter,
		Marshalizer:     arguments.Marshalizer,
		Hasher:          arguments.Hasher,
	}

	logsAndEventsProc, err := logsevents.NewLogsAndEventsProcessor(argsLogsAndEventsProc)
	if err != nil {
		return nil, err
	}

	ei.logsAndEventsProc = logsAndEventsProc
	memCache, err := memcache.NewCache(arguments.CacheExpirationTime, arguments.CacheCleanUpInterval)
	if err != nil {
		return nil, err
	}
	ei.cacher = memCache

	ei.txDatabaseProcessor = newTxDatabaseProcessor(
		arguments.Hasher,
		arguments.Marshalizer,
		arguments.AddressPubkeyConverter,
		arguments.ValidatorPubkeyConverter,
		arguments.IsInImportDBMode,
	)

	if arguments.UseKibana {
		err = ei.initWithKibana(arguments.IndexTemplates, arguments.IndexPolicies)
		if err != nil {
			return nil, err
		}
	} else {
		err = ei.initNoKibana(arguments.IndexTemplates)
		if err != nil {
			return nil, err
		}
	}

	return ei, nil
}

func checkArgElasticProcessor(arguments ArgElasticProcessor) error {
	if check.IfNil(arguments.DBClient) {
		return ErrNilDatabaseClient
	}
	if check.IfNil(arguments.Marshalizer) {
		return common.ErrNilMarshalizer
	}
	if check.IfNil(arguments.Hasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(arguments.AddressPubkeyConverter) {
		return ErrNilPubkeyConverter
	}
	if check.IfNil(arguments.ValidatorPubkeyConverter) {
		return ErrNilPubkeyConverter
	}
	if check.IfNil(arguments.AccountsDB) {
		return ErrNilAccountsDB
	}
	if check.IfNil(arguments.KappsDB) {
		return ErrNilKAppsDB
	}
	if check.IfNil(arguments.KAppController) {
		return ErrNilKAppController
	}

	return nil
}

func (ei *elasticProcessor) initWithKibana(indexTemplates, indexPolicies map[string]*bytes.Buffer) error {
	err := ei.createOpenDistroTemplates(indexTemplates)
	if err != nil {
		return err
	}

	// TODO: Re-activate after we think of a solid way to handle forks+rotating indexes
	/*err = ei.createIndexPolicies(indexPolicies)
	if err != nil {
		return err
	}*/

	err = ei.createIndexTemplates(indexTemplates)
	if err != nil {
		return err
	}

	err = ei.createIndexes()
	if err != nil {
		return err
	}

	err = ei.createAliases()
	if err != nil {
		return err
	}

	return ei.ensureFieldMappings(indexTemplates)
}

func (ei *elasticProcessor) initNoKibana(indexTemplates map[string]*bytes.Buffer) error {
	err := ei.createOpenDistroTemplates(indexTemplates)
	if err != nil {
		return err
	}

	err = ei.createIndexTemplates(indexTemplates)
	if err != nil {
		return err
	}

	err = ei.createIndexes()
	if err != nil {
		return err
	}

	err = ei.createAliases()
	if err != nil {
		return err
	}

	return ei.ensureFieldMappings(indexTemplates)
}

// nolint: unused
func (ei *elasticProcessor) createIndexPolicies(indexPolicies map[string]*bytes.Buffer) error {
	indexesPolicies := []string{txPolicy, blockPolicy, ratingPolicy, accountsHistoryPolicy}
	for _, indexPolicyName := range indexesPolicies {
		indexPolicy := getTemplateByName(indexPolicyName, indexPolicies)
		if indexPolicy != nil {
			err := ei.elasticClient.CheckAndCreatePolicy(indexPolicyName, indexPolicy)
			if err != nil {
				log.Error("check and create policy", "policy", indexPolicy, "err", err)
				return err
			}
		}
	}

	return nil
}

func (ei *elasticProcessor) createOpenDistroTemplates(indexTemplates map[string]*bytes.Buffer) error {
	opendistroTemplate := getTemplateByName("opendistro", indexTemplates)
	if opendistroTemplate != nil {
		err := ei.elasticClient.CheckAndCreateTemplate("opendistro", opendistroTemplate)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ei *elasticProcessor) createIndexTemplates(indexTemplates map[string]*bytes.Buffer) error {
	indexes := []string{
		txIndex,
		blockIndex,
		accountsIndex,
		accountsKDAIndex,
		logsIndex,
		scDeploysIndex,
		assetsIndex,
		proposalsIndex,
		accountsHistoryIndex,
		peersAccountsIndex,
		kdaPoolsIndex,
		iTOsIndex,
	}
	for _, index := range indexes {
		indexTemplate := getTemplateByName(index, indexTemplates)
		if indexTemplate != nil {
			err := ei.elasticClient.CheckAndCreateTemplate(index, indexTemplate)
			if err != nil {
				log.Error("check and create template", "err", err)
				return err
			}
		}
	}
	return nil
}

func (ei *elasticProcessor) createIndexes() error {
	indexes := []string{txIndex, blockIndex, accountsIndex, accountsKDAIndex, assetsIndex, proposalsIndex, accountsHistoryIndex, peersAccountsIndex, kdaPoolsIndex, iTOsIndex}
	for _, index := range indexes {
		indexName := fmt.Sprintf("%s-000001", index)
		err := ei.elasticClient.CheckAndCreateIndex(indexName)
		if err != nil {
			log.Error("check and create index", "indexName", indexName, "err", err)
			return err
		}
	}
	return nil
}

// ensureFieldMappings brings the transactions mapping up to date on a cluster that may
// predate the properties in templates.TransactionsAddedProperties: the stored template is
// rewritten so indices created from now on carry them, and the live index gains whichever
// of them it does not map yet. Both run on every start-up; the live index is read before it
// is written, so a node whose index is already up to date sends no write.
func (ei *elasticProcessor) ensureFieldMappings(indexTemplates map[string]*bytes.Buffer) error {
	if template := getTemplateByName(txIndex, indexTemplates); template != nil {
		err := ei.elasticClient.PutTemplate(txIndex, template)
		if err != nil {
			log.Error("put template", "index", txIndex, "err", err)
			return err
		}
	}

	properties := templates.Object{"properties": templates.TransactionsAddedProperties}

	err := ei.elasticClient.CheckAndUpdateMapping(txIndex, properties)
	if err != nil {
		log.Error("check and update mapping", "index", txIndex, "err", err)
		return err
	}

	return nil
}

func (ei *elasticProcessor) createAliases() error {
	indexes := []string{
		txIndex,
		blockIndex,
		accountsIndex,
		accountsKDAIndex,
		logsIndex,
		scDeploysIndex,
		assetsIndex,
		proposalsIndex,
		accountsHistoryIndex,
		peersAccountsIndex,
		kdaPoolsIndex,
		iTOsIndex,
	}
	for _, index := range indexes {
		indexName := fmt.Sprintf("%s-000001", index)
		err := ei.elasticClient.CheckAndCreateAlias(index, indexName)
		if err != nil {
			log.Error("check and create alias", "err", err)
			return err
		}
	}

	return nil
}

// SaveHeader will prepare and save information about a header in elasticsearch server
func (ei *elasticProcessor) SaveHeader(
	header nodeData.HeaderHandler,
	signer []byte,
	txsSize int,
	validators []string,
) error {
	if !ei.isIndexEnabled(blockIndex) {
		return nil
	}

	var buff bytes.Buffer

	serializedBlock, headerHash, err := ei.parser.getSerializedElasticBlockAndHeaderHash(header, signer, txsSize, validators)
	if err != nil {
		return err
	}

	buff.Grow(len(serializedBlock))
	_, err = buff.Write(serializedBlock)
	if err != nil {
		return err
	}

	req := &esapi.IndexRequest{
		Index:      blockIndex,
		DocumentID: hex.EncodeToString(headerHash),
		Body:       bytes.NewReader(buff.Bytes()),
		Refresh:    ei.indexRefresh(),
	}

	if err := ei.elasticClient.DoRequest(req); err != nil {
		return err
	}

	return nil
}

// RemoveHeader will remove a block from elasticsearch server
func (ei *elasticProcessor) RemoveHeader(header nodeData.HeaderHandler) error {
	headerHash, err := tools.CalculateHash(ei.marshalizer, ei.hasher, header.GetBlockHeader())
	if err != nil {
		return err
	}

	return ei.elasticClient.DoBulkRemove(blockIndex, []string{hex.EncodeToString(headerHash)})
}

// RemoveTransactions will remove all transactions that are in header from elasticsearch server
func (ei *elasticProcessor) RemoveTransactions(blk nodeData.HeaderHandler) error {
	if blk == nil || len(blk.GetTxHashes()) == 0 {
		return nil
	}

	encodedTXsHashes := make([]string, 0)
	for _, txHash := range blk.GetTxHashes() {
		encodedTXsHashes = append(encodedTXsHashes, hex.EncodeToString(txHash))
	}

	return ei.elasticClient.DoBulkRemove(txIndex, encodedTXsHashes)
}

// RemoveAccountsHistory will remove account history entries for a specific timestamp
func (ei *elasticProcessor) RemoveAccountsHistory(blockTimestamp int64) error {
	if !ei.isIndexEnabled(accountsHistoryIndex) {
		return nil
	}

	return ei.elasticClient.DoBulkRemoveByTimestamp(accountsHistoryIndex, toMilliseconds(blockTimestamp))
}

// RevertAccountBalances reverts the accounts index to the current state from the account trie
// This method should be called after RevertStateToBlock, as it reads the current balance from the trie
func (ei *elasticProcessor) RevertAccountBalances(blockTimestamp int64) error {
	if !ei.isIndexEnabled(accountsIndex) {
		return nil
	}
	// Query accounts index to find all accounts modified at this timestamp
	query := templates.Object{
		"query": templates.Object{
			"term": templates.Object{
				"updatedAt": toMilliseconds(blockTimestamp),
			},
		},
		"size":    revertAccountBatchSize,
		"_source": []string{"address", "balance", "frozenBalance"},
	}

	body, err := encode(query)
	if err != nil {
		return err
	}

	res, err := ei.elasticClient.DoSearch(accountsIndex, &body)
	if err != nil {
		return err
	}

	accounts, err := ei.extractAccountData(res)
	if err != nil {

		return err
	}

	if len(accounts) == 0 {
		log.Debug("RevertAccountBalances: no accounts to revert", "blockTimestamp", blockTimestamp)
		return nil
	}

	// For each account modified in this block, get current balance from trie and update index
	for _, accountInfo := range accounts {
		err := ei.revertSingleAccountBalanceFromTrie(accountInfo.Address, accountInfo.Balance, accountInfo.FrozenBalance)
		if err != nil {
			log.Warn("RevertAccountBalances: failed to revert account",
				"address", accountInfo.Address,
				"error", err.Error())
			continue
		}
	}

	return nil
}

// extractAccountAddresses extracts account addresses and balance information from the search response
func (ei *elasticProcessor) extractAccountData(response templates.Object) ([]*data.AccountBalanceInfo, error) {
	hits, ok := response["hits"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing hits")
	}

	innerHits, ok := hits["hits"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing inner hits")
	}

	accounts := make([]*data.AccountBalanceInfo, 0, len(innerHits))
	for _, hit := range innerHits {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		address, _ := source["address"].(string)
		if address == "" {
			continue
		}

		balance, _ := source["balance"].(float64)
		frozenBalance, _ := source["frozenBalance"].(float64)

		accounts = append(accounts, &data.AccountBalanceInfo{
			Address:       address,
			Balance:       int64(balance),
			FrozenBalance: int64(frozenBalance),
		})
	}

	return accounts, nil
}

// revertSingleAccountBalanceFromTrie reads the current balance from the account trie
// and updates the accounts index with that balance.
// This assumes RevertStateToBlock has already been called, so the trie is at the correct state.
// oldBalance and oldFrozenBalance are the values from the accounts-history index (for logging comparison).
func (ei *elasticProcessor) revertSingleAccountBalanceFromTrie(address string, oldBalance, oldFrozenBalance int64) error {
	addressBytes, err := ei.addressPubkeyConverter.Decode(address)
	if err != nil {
		return fmt.Errorf("failed to decode address %s: %w", address, err)
	}

	account, err := ei.accountsDB.LoadAccount(addressBytes)
	if err != nil {
		// If account doesn't exist in trie, revert to zero balance
		log.Debug("RevertAccountBalances: account not found in trie, reverting to zero balance",
			"address", address,
			"oldBalance", oldBalance,
			"oldFrozenBalance", oldFrozenBalance,
			"newBalance", 0,
			"newFrozenBalance", 0)
		return ei.updateAccountBalance(address, 0, 0)
	}

	userAccount, ok := account.(state.UserAccountHandler)
	if !ok {
		return fmt.Errorf("account is not a user account: %s", address)
	}

	// Get balance and frozen balance from the trie
	balance := userAccount.GetBalance(kdautils.KLVIdentifier, true)

	// Get frozen balance from KLV KDA
	frozenBalanceInt64 := int64(0)
	userKDA, err := userAccount.GetUserKDA(kdautils.KLVIdentifier, nil, true)
	if err == nil {
		frozenBalanceInt64 = userKDA.FrozenBalance
	}

	log.Debug("RevertAccountBalances: reverting account balance",
		"address", address,
		"oldBalance", oldBalance,
		"oldFrozenBalance", oldFrozenBalance,
		"newBalance", balance,
		"newFrozenBalance", frozenBalanceInt64)

	return ei.updateAccountBalance(address, balance, frozenBalanceInt64)
}

func (ei *elasticProcessor) updateAccountBalance(address string, balance int64, frozenBalance int64) error {
	// Create update script to modify only balance and frozenBalance fields
	update := templates.Object{
		"script": templates.Object{
			"source": "ctx._source.balance = params.balance; ctx._source.frozenBalance = params.frozenBalance",
			"params": templates.Object{
				"balance":       balance,
				"frozenBalance": frozenBalance,
			},
		},
	}

	updateBody, err := encode(update)
	if err != nil {
		return err
	}

	return ei.elasticClient.DoUpdate(accountsIndex, address, &updateBody)
}

// SaveTransactions saves a block's transactions to elasticsearch. If prepared
// is a non-nil *data.PreparedBlockData, its contents are reused; otherwise
// prepareTransactionsForDatabase runs here as a fallback.
func (ei *elasticProcessor) SaveTransactions(
	header nodeData.HeaderHandler,
	pool *indexer.Pool,
	prepared any,
) error {
	headerTimestamp := header.GetTimestamp()

	txs, txsMap, ad, err := ei.resolvePreparedBlockData(header, pool, prepared)
	if err != nil {
		return err
	}

	ld := ei.logsAndEventsProc.ExtractDataFromLogs(pool, txs, headerTimestamp)
	buffers := data.NewBufferSlice(data.DefaultMaxBulkSize)

	if err := ei.indexBlockArtifacts(buffers, headerTimestamp, txs, txsMap, ad, ld, pool); err != nil {
		return err
	}

	if err := ei.doBulkRequests("", buffers.Buffers()); err != nil {
		log.Warn("indexer indexing bulk of transactions", "error", err.Error())
		return err
	}

	return nil
}

// resolvePreparedBlockData returns the prepared block data if provided,
// otherwise it runs prepareTransactionsForDatabase as a fallback.
func (ei *elasticProcessor) resolvePreparedBlockData(
	header nodeData.HeaderHandler,
	pool *indexer.Pool,
	prepared any,
) ([]*data.Transaction, map[string]*data.Transaction, *data.AlteredData, error) {
	if p, ok := prepared.(*data.PreparedBlockData); ok && p != nil {
		return p.Txs, p.TxsMap, p.Altered, nil
	}
	return ei.prepareTransactionsForDatabase(header, pool)
}

// indexBlockArtifacts runs every sub-index step for a block. The list is
// linear; on the first error the pipeline stops and the error is returned.
func (ei *elasticProcessor) indexBlockArtifacts(
	buffers *data.BufferSlice,
	headerTimestamp int64,
	txs []*data.Transaction,
	txsMap map[string]*data.Transaction,
	ad *data.AlteredData,
	ld *data.PreparedLogsResults,
	pool *indexer.Pool,
) error {
	steps := []func() error{
		func() error { return ei.indexTransactions(txs, buffers) },
		func() error { return ei.indexKDAPools(ad.KDAPool.GetAll(), buffers) },
		func() error { return ei.indexAssets(ad.Assets.GetAll(), buffers) },
		func() error { return ei.indexProposals(ad.Proposals.GetAll(), buffers) },
		func() error { return ei.indexITOs(ad.ITO.GetAll(), buffers) },
		func() error { return ei.indexMarketplaces(ad.Marketplaces.GetAll(), buffers) },
		func() error { return ei.indexOrders(ad.Orders.GetAll(), buffers) },
		func() error { return ei.indexAlteredAccounts(headerTimestamp, ad.Accounts.GetAll(), buffers) },
		func() error { return ei.prepareAndIndexLogs(pool.Logs, txsMap, headerTimestamp, buffers) },
		func() error { return ei.indexScDeploys(ld.ScDeploys, buffers) },
		func() error { return ei.indexAlteredSmartContracts(ld.AlteredSCs, buffers) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func (ei *elasticProcessor) indexTransactions(txs []*data.Transaction, bytesBuff *data.BufferSlice) error {
	if !ei.isIndexEnabled(txIndex) {
		return nil
	}

	return ei.serializeTransactions(txs, bytesBuff)
}

func (ei *elasticProcessor) getProposal(proposalKapp state.KAppAccountHandler, proposalID uint64) (*data.Proposal, error) {
	key := kdautils.ToProposalKey(proposalID)

	proposalBytes, err := proposalKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(proposalBytes) == 0 {
		return nil, common.ErrEmptyString
	}

	proposal := &kapps.ProposalData{}
	err = ei.marshalizer.Unmarshal(proposal, proposalBytes)
	if err != nil {
		return nil, err
	}

	// Get Actual Network Parameters
	acnt, err := ei.kappsDB.LoadAccount(kapps.ProposalKAppAddress)
	if err != nil {
		return nil, err
	}

	proposalKApp, ok := acnt.(state.KAppAccountHandler)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}

	controllerBytes, err := proposalKApp.DataTrieTracker().RetrieveValue(kdautils.ProposalControllerKey)
	if err != nil {
		return nil, err
	}

	proposalController := &kapps.ProposalController{}
	err = ei.marshalizer.Unmarshal(proposalController, controllerBytes)
	if err != nil {
		return nil, err
	}

	oldParams := make(map[int32][]byte, 0)
	for index := range proposal.Parameters {
		oldParameter, ok := proposalController.ActiveParameters[index]
		if !ok {
			continue
		}
		oldParams[index] = oldParameter.Value
	}

	return ei.convertProposal(proposalID, proposal, oldParams), nil
}

func (ei *elasticProcessor) getITO(itoKapp state.KAppAccountHandler, assetID string) (*data.ITOInfo, error) {
	key := kdautils.ToITOKey([]byte(assetID))

	itoBytes, err := itoKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(itoBytes) == 0 {
		return nil, common.ErrEmptyString
	}

	ito := &kapps.ITOData{}
	err = ei.marshalizer.Unmarshal(ito, itoBytes)
	if err != nil {
		return nil, err
	}

	return ei.convertITO(ito), nil
}

func (cm *commonProcessor) convertITO(itoData *kapps.ITOData) *data.ITOInfo {
	var packInfo []*data.PackInfo
	for key, pack := range itoData.PackData {
		var packs []*data.PackItem
		for _, p := range pack.Packs {
			packs = append(packs, &data.PackItem{
				Amount: p.Amount,
				Price:  p.Price,
			})
		}

		packInfo = append(packInfo, &data.PackInfo{
			Key:   key,
			Packs: packs,
		})
	}

	convertedMarketplace := &data.ITOInfo{
		IsActive:               itoData.IsActive,
		MaxAmount:              itoData.MaxAmount,
		MintedAmount:           itoData.MintedAmount,
		ReceiverAddress:        cm.addressPubkeyConverter.Encode(itoData.ReceiverAddress),
		PackData:               packInfo,
		DefaultLimitPerAddress: itoData.DefaultLimitPerAddress,
		IsWhitelistActive:      itoData.IsWhitelistActive,
		WhitelistInfo:          nil,
		WhitelistStartTime:     itoData.WhitelistStartTime,
		WhitelistEndTime:       itoData.WhitelistEndTime,
		StartTime:              itoData.StartTime,
		EndTime:                itoData.EndTime,
	}

	return convertedMarketplace
}

func (ei *elasticProcessor) indexProposals(proposals map[string]*data.AlteredProposal, buffSlice *data.BufferSlice) error {
	if !ei.isIndexEnabled(proposalsIndex) || len(proposals) == 0 {
		return nil
	}

	toRecord := make(map[string]*data.Proposal)
	proposalKDAaccount, err := ei.kappsDB.LoadAccount(kapps.ProposalKAppAddress)
	if err != nil {
		return err
	}

	proposalKapp, ok := proposalKDAaccount.(state.KAppAccountHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	for proposalIDString := range proposals {
		// convert to uin64
		proposalID, err := strconv.ParseUint(proposalIDString, 10, 64)
		if err != nil {
			return err
		}

		convertedProposal, err := ei.getProposal(proposalKapp, proposalID)
		if err != nil {
			return err
		}
		toRecord[proposalIDString] = convertedProposal

	}

	err = serializeProposals(toRecord, buffSlice)
	if err != nil {
		return err
	}

	return nil
}

func (ei *elasticProcessor) getMarketplace(marketplaceKapp state.KAppAccountHandler, marketplaceID string) (*data.Marketplace, error) {
	marketplaceIDBytes, err := hex.DecodeString(marketplaceID)
	if err != nil {
		return nil, err
	}

	key := kdautils.ToMarketplaceKey(marketplaceIDBytes)

	marketplaceBytes, err := marketplaceKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(marketplaceBytes) == 0 {
		return nil, common.ErrEmptyString
	}

	marketplace := &kapps.Marketplace{}
	err = ei.marshalizer.Unmarshal(marketplace, marketplaceBytes)
	if err != nil {
		return nil, err
	}

	return ei.convertMarketplace(marketplaceID, marketplace), nil
}

func (ei *elasticProcessor) indexMarketplaces(marketplaces map[string]*data.AlteredMarketplaces, buffSlice *data.BufferSlice) error {
	if !ei.isIndexEnabled(marketplacesIndex) || len(marketplaces) == 0 {
		return nil
	}

	toRecord := make(map[string]*data.Marketplace)
	marketplaceKDAaccount, err := ei.kappsDB.LoadAccount(kapps.MarketKAppAddress)
	if err != nil {
		return err
	}

	marketplaceKapp, ok := marketplaceKDAaccount.(state.KAppAccountHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	for marketplaceIDString := range marketplaces {
		convertedMarketplace, err := ei.getMarketplace(marketplaceKapp, marketplaceIDString)
		if err != nil {
			return err
		}

		toRecord[marketplaceIDString] = convertedMarketplace
	}

	err = serializeMarketplaces(toRecord, buffSlice)
	if err != nil {
		return err
	}

	return nil
}

type itoIndexAux struct {
	ITO       *data.ITOInfo
	WhiteList map[string]*data.WhitelistInfo
}

func (ei *elasticProcessor) indexITOs(itos map[string]*data.AlteredITOs, buffSlice *data.BufferSlice) error {
	if !ei.isIndexEnabled(iTOsIndex) || len(itos) == 0 {
		return nil
	}
	itoRecords := make(map[string]*itoIndexAux)

	for assetID, itoInfo := range itos {

		if itoRecord, ok := itoRecords[assetID]; ok {
			if itoInfo.IsDisabled {
				itoRecord.ITO.IsActive = false
			}

			if itoInfo.IsEnabled {
				itoRecord.ITO.IsActive = true
			}

			for address, info := range itoInfo.AddedAddresses {
				itoRecord.WhiteList[address] = &data.WhitelistInfo{
					Address: address,
					Limit:   info.Limit,
				}
			}
			for address, alteredInfo := range itoInfo.AlteredAddresses {
				if info, ok := itoRecord.WhiteList[address]; ok {
					info.Limit -= alteredInfo.NftsBuyed
					if info.Limit < 0 {
						info.Limit += itoInfo.DefaultLimitPerAddress
					}
					itoRecord.WhiteList[address] = info
				}
			}
			for address := range itoInfo.RemovedAddresses {
				delete(itoRecord.WhiteList, address)
			}

			itoRecord.ITO.AssetID = assetID
			itoRecords[assetID] = itoRecord
			continue
		}

		itoKappAccount, err := ei.kappsDB.LoadAccount(kapps.ITOKAppAddress)
		if err != nil {
			return err
		}

		itoKapp, ok := itoKappAccount.(state.KAppAccountHandler)
		if !ok {
			return common.ErrWrongTypeAssertion
		}

		ito, err := ei.getITO(itoKapp, assetID)
		if err != nil {
			return err
		}

		ito.AssetID = assetID

		itoHandler := &itoIndexAux{
			ITO:       ito,
			WhiteList: ito.ConvertToMap(),
		}

		if itoInfo.IsNew {
			for address, info := range itoInfo.AddedAddresses {
				itoHandler.WhiteList[address] = &data.WhitelistInfo{
					Address: address,
					Limit:   info.Limit,
				}
			}
			for address, alteredInfo := range itoInfo.AlteredAddresses {
				if info, ok := itoHandler.WhiteList[address]; ok {
					info.Limit -= alteredInfo.NftsBuyed
					if info.Limit < 0 {
						info.Limit += itoInfo.DefaultLimitPerAddress
					}
					itoHandler.WhiteList[address] = info
				}
			}
			for address := range itoInfo.RemovedAddresses {
				delete(itoHandler.WhiteList, address)
			}

			itoHandler.ITO.Timestamp = itoInfo.Timestamp
			itoRecords[assetID] = itoHandler

			continue
		}

		if itoInfo.IsDisabled {
			itoHandler.ITO.IsActive = false
		}

		if itoInfo.IsEnabled {
			itoHandler.ITO.IsActive = true
		}

		decodedBody, err := ei.elasticClient.Get(iTOsIndex, assetID)
		if err != nil {
			log.Warn("indexITOs.ElasticGet", "error:", err.Error())
		} else {
			var fromDb *data.ITOInfo

			if err := ei.elasticClient.ConvertObjectToData(decodedBody, &fromDb); err != nil {
				return err
			}

			if fromDb != nil {
				// sync current ito whitelist from db
				itoHandler.ITO.WhitelistInfo = fromDb.WhitelistInfo
				itoHandler.WhiteList = fromDb.ConvertToMap()
			}
		}

		for address, info := range itoInfo.AddedAddresses {
			itoHandler.WhiteList[address] = &data.WhitelistInfo{
				Address: address,
				Limit:   info.Limit,
			}
		}
		for address, alteredInfo := range itoInfo.AlteredAddresses {
			if info, ok := itoHandler.WhiteList[address]; ok {
				info.Limit -= alteredInfo.NftsBuyed
				itoHandler.WhiteList[address] = info
			}
		}
		for address := range itoInfo.RemovedAddresses {
			delete(itoHandler.WhiteList, address)
		}

		itoRecords[assetID] = itoHandler
	}

	toRecord := make(map[string]*data.ITOInfo)
	for asset, rec := range itoRecords {
		ito := rec.ITO
		ito.SetWhiteListFromMap(rec.WhiteList)
		toRecord[asset] = ito
	}

	err := serializeITOs(toRecord, buffSlice)
	if err != nil {
		return err
	}

	return nil
}

func (ei *elasticProcessor) indexOrders(orders map[string]*data.MarketOperations, buffSlice *data.BufferSlice) error {
	if !ei.isIndexEnabled(marketplaceOrdersIndex) || len(orders) == 0 {
		return nil
	}

	marketplaceKDAaccount, err := ei.kappsDB.LoadAccount(kapps.MarketKAppAddress)
	if err != nil {
		return err
	}

	marketplaceKapp, ok := marketplaceKDAaccount.(state.KAppAccountHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	toRecord := make(map[string]*data.Order)
	for id, alteredOrder := range orders {
		if alteredOrder.IsNew {
			decodedId, err := hex.DecodeString(id)
			if err != nil {
				return err
			}

			key := kdautils.ToMarketOrderKey(decodedId)
			orderBytes, err := marketplaceKapp.DataTrieTracker().RetrieveValue(key)
			if err != nil && !errors.Is(err, common.ErrNilTrie) {
				return err
			}
			if len(orderBytes) == 0 {
				return common.ErrNotFoundInKApp
			}

			order := &kapps.MarketOrderData{}
			err = ei.marshalizer.Unmarshal(order, orderBytes)
			if err != nil {
				return err
			}

			convertedOrder := &data.Order{
				OrderID:       id,
				MarketType:    order.MarketType.String(),
				MarketplaceID: hex.EncodeToString(order.MarketplaceID),
				CollectionID:  string(order.CollectionID),
				AssetID:       string(order.AssetID),
				CurrencyID:    string(order.CurrencyID),
				Price:         order.Price,
				ReservePrice:  order.ReservePrice,
				EndTime:       order.EndTime,
				Status:        data.Created,
				OrderTxHash:   alteredOrder.OrderTxHash,
				OwnerAddress:  ei.addressPubkeyConverter.Encode(order.GetOwnerAddress()),
				Timestamp:     alteredOrder.Timestamp, // add timestamp of transaction only if order is new
			}

			toRecord[id] = convertedOrder
		} else {
			decodedBody, err := ei.elasticClient.Get(marketplaceOrdersIndex, id)
			if err != nil {
				log.Warn("indexITOs.ElasticGet", "error:", err.Error())
			} else {
				order, err := ei.elasticClient.ConvertObjectToOrder(decodedBody)
				if err != nil {
					return err
				}

				if alteredOrder.IsClaim {
					for _, buyOrder := range order.BuyOrders {
						if buyOrder.Amount == alteredOrder.BuyOrder.Amount && buyOrder.Bidder == alteredOrder.BuyOrder.Bidder {
							buyOrder.Status = data.Fulfilled
						}
					}
					order.Status = data.Fulfilled
				} else if alteredOrder.IsCanceled {
					order.Status = data.Canceled
				} else {
					buyOrder := &data.BuyOrder{
						CurrencyID: alteredOrder.BuyOrder.CurrencyID,
						Amount:     alteredOrder.BuyOrder.Amount,
						Bidder:     alteredOrder.BuyOrder.Bidder,
					}

					if alteredOrder.Executed {
						buyOrder.Status = data.Fulfilled
						order.Status = data.Fulfilled
					} else {
						buyOrder.Status = data.Created
					}

					order.CurrentBid = alteredOrder.BuyOrder.Amount
					order.CurrentBidder = alteredOrder.BuyOrder.Bidder
					order.CurrentBuyOrderTxHash = alteredOrder.BuyOrderTxHash
					order.BuyOrders = append(order.BuyOrders, buyOrder)
				}

				toRecord[id] = order
			}
		}

	}

	err = serializeOrders(toRecord, buffSlice)
	if err != nil {
		return err
	}

	return nil

}

func (ei *elasticProcessor) UpdateProposalsAndParameters(proposalsIDs []string) error {
	if !ei.isIndexEnabled(proposalsIndex) {
		return nil
	}

	// Update existing active proposals
	if len(proposalsIDs) > 0 {
		proposalToIndex := data.NewAlteredProposals()
		for _, proposal := range proposalsIDs {
			proposalToIndex.Add(proposal, &data.AlteredProposal{
				IsNew: false,
			})
		}

		buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)
		err := ei.indexProposals(proposalToIndex.GetAll(), buffSlice)
		if err != nil {
			return err
		}

		err = ei.doBulkRequests(proposalsIndex, buffSlice.Buffers())
		if err != nil {
			return err
		}
	}

	return nil
}

func (ei *elasticProcessor) indexKDAPools(pools map[string][]*data.AlteredKDAPool, buffSlice *data.BufferSlice) error {
	if !ei.isIndexEnabled(kdaPoolsIndex) || len(pools) == 0 {
		return nil
	}

	var poolsSlice []*data.KDAPoolData
	var poolsUpdateSlice []*data.KDAPoolData

	kdaPoolAccount, err := ei.kappsDB.LoadAccount(kapps.KDAFeesPoolKAppAddress)
	if err != nil {
		return err
	}
	kdaPoolKapp, ok := kdaPoolAccount.(state.KAppAccountHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	for poolID := range pools {
		poolBytes, err := kdaPoolKapp.DataTrieTracker().RetrieveValue([]byte(poolID))
		if err != nil && !errors.Is(err, common.ErrNilTrie) {
			return err
		}

		kdaPool := &kdafeespool.KDAFeesPoolData{}
		if len(poolBytes) == 0 {
			continue
		}

		err = ei.marshalizer.Unmarshal(kdaPool, poolBytes)
		if err != nil {
			return err
		}

		poolData := ei.convertKDAPoolData(kdaPool)

		exist := ei.elasticClient.DocExists(kdaPoolsIndex, poolID)
		if !exist {
			initialValue := false
			poolData.Hidden = &initialValue
			poolData.Verified = &initialValue

			poolsSlice = append(poolsSlice, poolData)
		} else {
			poolsUpdateSlice = append(poolsUpdateSlice, poolData)
		}
	}

	err = serializeUpdateKDAPools(poolsUpdateSlice, buffSlice, kdaPoolsIndex)
	if err != nil {
		return err
	}

	err = serializeKDAPools(poolsSlice, buffSlice, kdaPoolsIndex)
	if err != nil {
		return err
	}

	return nil
}

const SFTSeparator = "%2F"

func (ei *elasticProcessor) indexAssets(assets map[string][]*data.AlteredAsset, buffSlice *data.BufferSlice) error {
	if !ei.isIndexEnabled(assetsIndex) || len(assets) == 0 {
		return nil
	}

	var assetsSlice []*data.Asset
	var assetsUpdateSlice []*data.Asset

	// Initialize kdaPoolKapp to check if asset has kda pool
	kdaPoolAccount, err := ei.kappsDB.LoadAccount(kapps.KDAFeesPoolKAppAddress)
	if err != nil {
		return err
	}

	kdaPoolKapp, ok := kdaPoolAccount.(state.KAppAccountHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	for assetName := range assets {
		assetKDAaccount, err := ei.kappsDB.LoadAccount(kapps.KDAKAppAddress)
		if err != nil {
			return err
		}

		kdaKapp, ok := assetKDAaccount.(state.KAppAccountHandler)
		if !ok {
			return common.ErrWrongTypeAssertion
		}

		assetSplit := strings.Split(assetName, kapps.Sp)
		assetID := assetSplit[0]

		key := kdautils.ToKDAKey([]byte(assetID), nil)

		kdaBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(key)
		if err != nil {
			return err
		}
		if len(kdaBytes) == 0 {
			return common.ErrEmptyString
		}

		kda := &kapps.KDAData{}
		err = ei.marshalizer.Unmarshal(kda, kdaBytes)
		if err != nil {
			return err
		}

		asset := ei.convertAssetInfo(kda)

		if kda.AssetType == kapps.KDAData_SemiFungible {
			err = ei.getSFTMeta(assetSplit, asset)
			if err != nil {
				return err
			}

			assetID = strings.ReplaceAll(assetName, kapps.Sp, SFTSeparator)
			asset.AssetID = assetName
		} else {
			//Check Staking Data
			stakingKDAAccount, err := ei.kappsDB.LoadAccount(kapps.StakingKAppAddress)
			if err != nil {
				return err
			}

			stakingKapp, ok := stakingKDAAccount.(state.KAppAccountHandler)
			if !ok {
				return common.ErrWrongTypeAssertion
			}

			stakedBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(key)
			if err != nil {
				return err
			}

			if len(stakedBytes) != 0 {
				kdaStaking := &kapps.StakingData{}
				err = ei.marshalizer.Unmarshal(kdaStaking, stakedBytes)
				if err != nil {
					return err
				}

				asset.Staking = ei.convertStakingData(kdaStaking)
			}
		}
		exist := ei.elasticClient.DocExists(assetsIndex, assetID)
		if !exist {
			initialAssetPermissionsState := false
			asset.Hidden = &initialAssetPermissionsState
			asset.Verified = &initialAssetPermissionsState
			asset.HasKdaPool = false
			asset.Tags = []string{}

			assetsSlice = append(assetsSlice, asset)
		} else {
			//check if asset have kda pool activated
			poolBytes, err := kdaPoolKapp.DataTrieTracker().RetrieveValue([]byte(assetID))
			if err != nil && !errors.Is(err, common.ErrNilTrie) {
				return err
			}

			if len(poolBytes) != 0 {
				kdaPool := &kdafeespool.KDAFeesPoolData{}
				if len(poolBytes) == 0 {
					continue
				}

				err = ei.marshalizer.Unmarshal(kdaPool, poolBytes)
				if err != nil {
					return err
				}

				poolData := ei.convertKDAPoolData(kdaPool)
				asset.HasKdaPool = poolData.Active
			}
			assetsUpdateSlice = append(assetsUpdateSlice, asset)
		}
	}

	err = serializeUpdateAssets(assetsUpdateSlice, buffSlice, assetsIndex)
	if err != nil {
		return err
	}

	err = serializeAssets(assetsSlice, buffSlice, assetsIndex)
	if err != nil {
		return err
	}

	return nil
}

func (ei *elasticProcessor) getSFTMeta(assetParts []string, asset *data.Asset) error {
	if len(assetParts) != 2 {
		return nil
	}

	meta, err := ei.kappsController.GetSystemAccountKApp().SFTGetMetaUncached([]byte(assetParts[0]), []byte(assetParts[1]))
	if err != nil {
		return err
	}

	// prepare to index
	asset.Metadata = ei.convertSFTMeta(meta)
	asset.URIs = nil
	asset.InitialSupply = 0
	asset.CirculatingSupply = 0
	asset.MaxSupply = 0
	asset.MintedValue = 0
	asset.BurnedValue = 0
	asset.Royalties = nil
	isSft := true
	asset.IsSFT = &isSft
	asset.Staking = nil
	asset.Attributes = nil
	asset.Roles = nil
	asset.Tags = nil

	return nil
}

func (ei *elasticProcessor) SaveAssets(assetsSlice []*data.Asset) error {
	if !ei.isIndexEnabled(assetsIndex) {
		return nil
	}

	buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

	err := serializeAssets(assetsSlice, buffSlice, assetsIndex)
	if err != nil {
		return err
	}

	return ei.doBulkRequests(assetsIndex, buffSlice.Buffers())
}

func (ei *elasticProcessor) indexAlteredSmartContracts(alteredSCsHandler data.AlteredSmartContractsHandler, buffSlice *data.BufferSlice) error {
	if !ei.isIndexEnabled(scDeploysIndex) {
		return nil
	}
	alteredSCs := make(map[string][]*data.AlteredSmartContract)
	if alteredSCsHandler != nil {
		alteredSCs = alteredSCsHandler.GetAll()
	}

	return SerializeAlteredSmartContracts(alteredSCs, buffSlice, scDeploysIndex)
}

func (ei *elasticProcessor) indexAlteredAccounts(
	blockTimestamp int64,
	accounts map[string][]*data.AlteredAccount,
	buffSlice *data.BufferSlice,
) error {
	if !ei.isIndexEnabled(accountsIndex) || len(accounts) == 0 {
		return nil
	}

	accountsToIndex := make([]state.UserAccountHandler, 0)
	accountsKDAToIndex := make([]*data.AccountKDA, 0)

	for address, alteredInfo := range accounts {
		addressBytes, err := ei.addressPubkeyConverter.Decode(address)
		if err != nil {
			log.Warn("cannot decode address", "address", address, "error", err)
			continue
		}

		var account state.AccountHandler

		account, err = ei.accountsDB.LoadAccount(addressBytes)
		if err != nil {
			log.Warn("cannot load user account", "address", address, "error", err)
			continue
		}

		userAccount, ok := account.(state.UserAccountHandler)
		if !ok {
			log.Warn("cannot cast AccountHandler to type UserAccountHandler")
			continue
		}

		accountsToIndex = append(accountsToIndex, userAccount)

		// check if it is a kapp account
		if kappName := kapps.KAppsAddressList[string(addressBytes)]; len(kappName) > 0 {
			log.Debug("index kapp altered account", "address", address, "kappName", kappName)
			account, err = ei.kappsDB.LoadAccount(addressBytes)
			if err != nil {
				log.Warn("cannot load kapp account", "address", address, "error", err)
				continue
			}
		}

		for _, alteredData := range alteredInfo {
			if alteredData.IsKDAOperation {
				accountKDA, ok := account.(state.AccountHandlerWithGetUserKDA)
				if !ok {
					log.Warn("cannot cast AccountHandler to type AccountHandlerWithGetUserKDA")
					continue
				}

				d, err := ei.getKDAInfo(accountKDA, address, alteredData)
				if err != nil {
					log.Error("error getting KDA Info", "error", err.Error())
					continue
				}

				accountsKDAToIndex = append(accountsKDAToIndex, d)
			}
		}
	}

	if len(accountsToIndex) == 0 {
		log.Debug("no account to index from provided transactions")
		return nil
	}

	accountsSlice := make([]*data.Account, len(accountsToIndex))
	for idx, account := range accountsToIndex {
		accountsSlice[idx] = &data.Account{
			UserAccount: account,
			IsSender:    false,
		}
	}

	err := serializeAccountsKDA(accountsKDAToIndex, buffSlice)
	if err != nil {
		return err
	}
	// ei.saveAccountsKDAHistory(blockTimestamp, accountsKDAToIndex)

	return ei.saveAccounts(blockTimestamp, accountsSlice, buffSlice)
}

func (ei *elasticProcessor) getKDAInfo(account state.AccountHandlerWithGetUserKDA, accountAddress string, alteredData *data.AlteredAccount) (*data.AccountKDA, error) {
	var nonce []byte
	assetID := alteredData.TokenIdentifier
	if len(alteredData.NFTNonce) > 0 {
		nonce = []byte(alteredData.NFTNonce)
		assetID += kapps.Sp + alteredData.NFTNonce
	}

	userKDA, err := account.GetUserKDA([]byte(alteredData.TokenIdentifier), nonce, true)
	if err != nil {
		log.Warn("cannot retrieve KDA", "address", accountAddress, "tokenIdentifier", assetID, "error", err)
		return nil, err
	}

	//Get Asset Precision
	//TODO: Add cache for this
	assetKDAaccount, err := ei.kappsDB.LoadAccount(kapps.KDAKAppAddress)
	if err != nil {
		return nil, err
	}

	kdaKapp, ok := assetKDAaccount.(state.KAppAccountHandler)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}

	key := kdautils.ToKDAKey([]byte(alteredData.TokenIdentifier), nil)

	kdaBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(kdaBytes) == 0 {
		return nil, common.ErrEmptyString
	}

	kda := &kapps.KDAData{}
	err = ei.marshalizer.Unmarshal(kda, kdaBytes)
	if err != nil {
		log.Warn("cannot retrive KDA", "address", accountAddress, "tokenIdentifier", alteredData.TokenIdentifier, "error", err)
		return nil, err
	}

	//Check Staking Data to get the address type
	stakingKDAAccount, err := ei.kappsDB.LoadAccount(kapps.StakingKAppAddress)
	if err != nil {
		return nil, err
	}

	stakingKapp, ok := stakingKDAAccount.(state.KAppAccountHandler)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}

	stakedBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}

	kdaStaking := &kapps.StakingData{}
	if len(stakedBytes) != 0 {
		err = ei.marshalizer.Unmarshal(kdaStaking, stakedBytes)
		if err != nil {
			return nil, err
		}
	}

	buckets := ei.convertAccountBuckets(userKDA.Buckets)

	unfrozenBalance := int64(0)
	for _, bucket := range buckets {
		if bucket.UnstakedEpoch != core.DefaultUnstakedEpoch {
			unfrozenBalance += bucket.Value
		}
	}

	accountKDA := &data.AccountKDA{
		AccountAddress:  accountAddress,
		AssetID:         assetID,
		Collection:      alteredData.TokenIdentifier,
		AssetName:       string(kda.Name),
		AssetType:       kda.AssetType,
		Balance:         userKDA.Balance,
		Precision:       kda.Precision,
		FrozenBalance:   userKDA.FrozenBalance,
		UnfrozenBalance: unfrozenBalance,
		LastClaim:       convertAccountLastClaim(userKDA.LastClaim),
		Buckets:         buckets,
		Metadata:        string(userKDA.Metadata),
		MIME:            string(userKDA.MIME),
		MarketplaceID:   alteredData.MarketplaceID,
		OrderID:         alteredData.OrderID,
		StakingType:     kdaStaking.GetInterestType(),
	}

	if len(alteredData.NFTNonce) > 0 {
		nftNonce, err := strconv.ParseUint(alteredData.NFTNonce, 10, 64)
		if err != nil {
			log.Warn("Unable to parse NFT Nonce", "error", err)
		}

		accountKDA.NFTNonce = nftNonce
	}

	return accountKDA, nil
}

// SaveAccounts will prepare and save information about provided accounts in elasticsearch server
func (ei *elasticProcessor) SaveAccounts(
	blockTimestamp int64,
	accountsSlice []*data.Account,
) error {
	if !ei.isIndexEnabled(accountsIndex) {
		return nil
	}

	buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

	err := ei.saveAccounts(blockTimestamp, accountsSlice, buffSlice)
	if err != nil {
		return err
	}

	return ei.doBulkRequests(accountsIndex, buffSlice.Buffers())
}

// dispatchAccountEvents sends account events to the event queue if enabled.
func dispatchAccountEvents(accountsMap map[string]*data.AccountInfo) {
	if UseEventQueue && len(accountsMap) > 0 {
		trySendEvent(Event{
			EvType:  ACCOUNTS,
			Message: accountsMap,
		})
	}
}

func (ei *elasticProcessor) saveAccounts(
	blockTimestamp int64,
	accountsSlice []*data.Account,
	buffSlice *data.BufferSlice,
) error {
	if !ei.isIndexEnabled(accountsIndex) && len(accountsSlice) > 0 {
		return nil
	}

	newAccountsMap := make(map[string]*data.AccountInfo)
	updateAccountsMap := make(map[string]*data.AccountInfo)
	historyAccountsMap := make(map[string]*data.AccountInfo)

	for _, userAccount := range accountsSlice {
		acc, err := buildAccountInfo(ei.addressPubkeyConverter, ei.kappsController, userAccount.UserAccount, blockTimestamp)
		if err != nil {
			return err
		}

		address := acc.Address
		if ei.elasticClient.DocExists(accountsIndex, address) {
			updateAccountsMap[address] = acc
		} else {
			newAccountsMap[address] = acc
		}
		historyAccountsMap[address] = acc
	}

	if err := serializeAccounts(newAccountsMap, buffSlice, accountsIndex); err != nil {
		return err
	}

	if err := serializedDataForUpdateAccounts(updateAccountsMap, buffSlice, accountsIndex); err != nil {
		return err
	}

	return ei.saveAccountsHistory(blockTimestamp, historyAccountsMap)
}

func (ei *elasticProcessor) saveAccountsHistory(blockTimestamp int64, accountsInfoMap map[string]*data.AccountInfo) error {
	if !ei.isIndexEnabled(accountsHistoryIndex) {
		return nil
	}

	buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

	accountsMap := make(map[string]*data.AccountBalanceHistory)
	for address, userAccount := range accountsInfoMap {
		acc := &data.AccountBalanceHistory{
			Address:       address,
			Balance:       userAccount.Balance,
			FrozenBalance: userAccount.FrozenBalance,
			Timestamp:     toMilliseconds(blockTimestamp),
		}
		addressKey := fmt.Sprintf("%s_%d", address, blockTimestamp)
		accountsMap[addressKey] = acc
	}

	err := serializeAccountsHistory(accountsMap, buffSlice, accountsHistoryIndex)
	if err != nil {
		return err
	}

	err = ei.doBulkRequests(accountsHistoryIndex, buffSlice.Buffers())
	if err != nil {
		log.Warn("indexer: indexing bulk of accounts history",
			"error", err.Error())
		return err
	}

	return nil
}

// SavePeersAccounts will prepare and save information about a peer account in elasticsearch server
func (ei *elasticProcessor) SavePeersAccounts(validators []kapp.ValidatorAccountInfoHandler) error {
	if !ei.isIndexEnabled(peersAccountsIndex) {
		return nil
	}

	valsConvertedList := make([]*data.ValidatorAccountInfo, 0)
	for _, val := range validators {
		valsConvertedList = append(valsConvertedList, &data.ValidatorAccountInfo{
			OwnerAddress:                        ei.addressPubkeyConverter.Encode(val.GetOwnerAddress()),
			BLSPublicKey:                        hex.EncodeToString(val.GetBlsPubKey()),
			RewardsAddress:                      ei.addressPubkeyConverter.Encode(val.GetRewardsAddress()),
			RegisterNonce:                       val.GetRegisterNonce(),
			SelfStaked:                          val.GetSelfStaked(),
			SelfStake:                           val.GetSelfStake(),
			TotalStake:                          val.GetTotalStake(),
			JailedEpoch:                         val.GetJailedEpoch(),
			Jailed:                              val.GetJailed(),
			Waiting:                             val.GetWaiting(),
			NumJailed:                           val.GetNumJailed(),
			TotalSlash:                          val.GetTotalSlash(),
			CanDelegate:                         val.GetCanDelegate(),
			MaxDelegation:                       val.GetMaxDelegation(),
			Commission:                          val.GetCommission(),
			TotalRewards:                        val.GetTotalRewards(),
			Name:                                val.GetName(),
			Logo:                                val.GetLogo(),
			URIs:                                ei.convertURIs(val.GetURIs()),
			List:                                val.GetStats().GetListString(),
			Index:                               val.GetStats().GetIndex(),
			AccumulatedFees:                     val.GetStats().GetAccumulatedFees(),
			ValidatorSuccessRate:                &data.SignRate{NumSuccess: val.GetStats().GetValidatorSuccessRateSuccess(), NumFailure: val.GetStats().GetValidatorSuccessRateFailure()},
			LeaderSuccessRate:                   &data.SignRate{NumSuccess: val.GetStats().GetLeaderSuccessRateSuccess(), NumFailure: val.GetStats().GetLeaderSuccessRateFailure()},
			ValidatorIgnoredSignaturesRate:      val.GetStats().GetValidatorIgnoredSignaturesRate(),
			Rating:                              val.GetStats().GetRating(),
			TempRating:                          val.GetStats().GetTempRating(),
			NumSelectedInSuccessBlocks:          val.GetStats().GetNumSelectedInSuccessBlocks(),
			ConsecutiveProposerMisses:           val.GetStats().GetConsecutiveProposerMisses(),
			TotalValidatorSuccessRate:           &data.SignRate{NumSuccess: val.GetStats().GetTotalValidatorSuccessRateSuccess(), NumFailure: val.GetStats().GetTotalValidatorSuccessRateFailure()},
			TotalLeaderSuccessRate:              &data.SignRate{NumSuccess: val.GetStats().GetTotalLeaderSuccessRateSuccess(), NumFailure: val.GetStats().GetTotalLeaderSuccessRateFailure()},
			TotalValidatorIgnoredSignaturesRate: val.GetStats().GetTotalValidatorIgnoredSignaturesRate(),
		})
	}

	buffSlice := data.NewBufferSlice(data.DefaultMaxBulkSize)

	err := serializePeersAccounts(valsConvertedList, buffSlice)
	if err != nil {
		return err
	}

	err = ei.doBulkRequests(peersAccountsIndex, buffSlice.Buffers())
	if err != nil {
		log.Warn("indexer: indexing bulk of peers accounts",
			"error", err.Error())
		return err
	}

	return nil
}

func (ei *elasticProcessor) SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler) error {
	if !ei.isIndexEnabled(epochIndex) {
		return nil
	}

	validatorsInfo := ei.buildEpochValidatorsInfo(validators)

	kdaKapp, err := ei.loadKAppHandler(kapps.KDAKAppAddress)
	if err != nil {
		return err
	}
	stakingKapp, err := ei.loadKAppHandler(kapps.StakingKAppAddress)
	if err != nil {
		return err
	}

	klvKey := kdautils.ToKDAKey([]byte(kdautils.KLVIdentifier), nil)
	kfiKey := kdautils.ToKDAKey([]byte(kdautils.KFIIdentifier), nil)

	klvKDA, err := ei.readEpochKDAData(kdaKapp, klvKey)
	if err != nil {
		return err
	}
	klvStaking, err := ei.readEpochStakingData(stakingKapp, klvKey)
	if err != nil {
		return err
	}
	kfiKDA, err := ei.readEpochKDAData(kdaKapp, kfiKey)
	if err != nil {
		return err
	}
	kfiStaking, err := ei.readEpochStakingData(stakingKapp, kfiKey)
	if err != nil {
		return err
	}

	epochInfo := &data.EpochInfo{
		Epoch:                epoch,
		Validators:           validatorsInfo,
		KFICirculatingSupply: kfiKDA.CirculatingSupply,
		KLVCirculatingSupply: klvKDA.CirculatingSupply,
		KFITotalStaked:       kfiStaking.TotalStaked,
		KLVTotalStaked:       klvStaking.TotalStaked,
	}

	return ei.indexEpochInfo(epoch, epochInfo)
}

// buildEpochValidatorsInfo maps validator account handlers into the indexer's ValidatorInfo docs.
func (ei *elasticProcessor) buildEpochValidatorsInfo(validators []kapp.ValidatorAccountInfoHandler) []*data.ValidatorInfo {
	var validatorsInfo []*data.ValidatorInfo
	for _, val := range validators {
		validatorsInfo = append(validatorsInfo, &data.ValidatorInfo{
			OwnerAddress:   ei.addressPubkeyConverter.Encode(val.GetOwnerAddress()),
			RewardsAddress: ei.addressPubkeyConverter.Encode(val.GetRewardsAddress()),
			PublicKey:      hex.EncodeToString(val.GetBlsPubKey()),
			List:           val.GetStats().GetListString(),
			SelfStaked:     val.GetSelfStaked(),
			SelfStake:      val.GetSelfStake(),
			TotalStake:     val.GetTotalStake(),
			JailedEpoch:    val.GetJailedEpoch(),
			Jailed:         val.GetJailed(),
			Waiting:        val.GetWaiting(),
			NumJailed:      val.GetNumJailed(),
			TotalSlash:     val.GetTotalSlash(),
			CanDelegate:    val.GetCanDelegate(),
			MaxDelegation:  val.GetMaxDelegation(),
			Commission:     val.GetCommission(),
			TotalRewards:   val.GetTotalRewards(),
			Name:           val.GetName(),
			Logo:           val.GetLogo(),
			URIs:           ei.convertURIs(val.GetURIs()),
		})
	}
	return validatorsInfo
}

// loadKAppHandler loads a KApp account and asserts it to the handler interface.
func (ei *elasticProcessor) loadKAppHandler(address []byte) (state.KAppAccountHandler, error) {
	account, err := ei.kappsDB.LoadAccount(address)
	if err != nil {
		return nil, err
	}
	handler, ok := account.(state.KAppAccountHandler)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}
	return handler, nil
}

// readEpochKDAData reads and unmarshals a KDAData record; a missing record is an error.
func (ei *elasticProcessor) readEpochKDAData(app state.KAppAccountHandler, key []byte) (*kapps.KDAData, error) {
	raw, err := app.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, common.ErrEmptyString
	}
	kda := &kapps.KDAData{}
	if err := ei.marshalizer.Unmarshal(kda, raw); err != nil {
		return nil, err
	}
	return kda, nil
}

// readEpochStakingData reads and unmarshals a StakingData record; a missing record is an error.
func (ei *elasticProcessor) readEpochStakingData(app state.KAppAccountHandler, key []byte) (*kapps.StakingData, error) {
	raw, err := app.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, common.ErrEmptyString
	}
	staking := &kapps.StakingData{}
	if err := ei.marshalizer.Unmarshal(staking, raw); err != nil {
		return nil, err
	}
	return staking, nil
}

// indexEpochInfo marshals the epoch document and writes it to the epoch index.
func (ei *elasticProcessor) indexEpochInfo(epoch uint32, epochInfo *data.EpochInfo) error {
	marshalizedEpochInfo, err := json.Marshal(epochInfo)
	if err != nil {
		log.Debug("indexer: marshal", "error", "could not marshal epoch info")
		return err
	}

	var buff bytes.Buffer
	buff.Grow(len(marshalizedEpochInfo))
	if _, err = buff.Write(marshalizedEpochInfo); err != nil {
		log.Warn("elastic search: epoch, write", "error", err.Error())
	}

	req := &esapi.IndexRequest{
		Index:      epochIndex,
		DocumentID: fmt.Sprint(epoch),
		Body:       bytes.NewReader(buff.Bytes()),
		Refresh:    ei.indexRefresh(),
	}

	return ei.elasticClient.DoRequest(req)
}

func (ei *elasticProcessor) isIndexEnabled(index string) bool {
	_, isEnabled := ei.enabledIndexes[index]
	return isEnabled
}

// indexRefresh returns the Elasticsearch IndexRequest refresh policy.
// Live mode forces refresh so docs are immediately searchable; import-db
// skips force-refresh to speed full-chain ES backfill.
func (ei *elasticProcessor) indexRefresh() string {
	if ei != nil && ei.txDatabaseProcessor != nil && ei.isInImportMode {
		return "false"
	}
	return "true"
}

func getTemplateByName(templateName string, templateList map[string]*bytes.Buffer) *bytes.Buffer {
	if template, ok := templateList[templateName]; ok {
		return template
	}

	log.Debug("elasticProcessor.getTemplateByName", "could not find template", templateName)
	return nil
}

func (ei *elasticProcessor) doBulkRequests(index string, buffSlice []*bytes.Buffer) error {
	var err error
	for idx := range buffSlice {
		ctxWithValue := context.WithValue(context.Background(), contextKey, bulkTopic)
		err = ei.elasticClient.DoBulkRequest(ctxWithValue, buffSlice[idx], index)
		if err != nil {
			return err
		}
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (ei *elasticProcessor) IsInterfaceNil() bool {
	return ei == nil
}
