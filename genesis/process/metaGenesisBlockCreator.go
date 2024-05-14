package process

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	ptx "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/genesis"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

// setBalancesToTrie adds balances to trie
func setBalancesToTrie(arg ArgsGenesisBlockCreator) (int, []byte, error) {
	gTX := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Nonce:  0,
			Sender: core.ZeroAddress,
		},
		Receipts:   make([]*transaction.Transaction_Receipt, 0),
		Result:     transaction.Transaction_SUCCESS,
		ResultCode: transaction.Transaction_Ok,
		Signature:  [][]byte{[]byte("klever genesis block")},
	}

	initialAccounts := arg.AccountsParser.InitialAccounts()
	for _, accnt := range initialAccounts {
		err := setBalanceToTrie(arg, accnt, gTX)
		if err != nil {
			return 0, nil, err
		}
	}

	txHash, err := tools.CalculateHash(arg.Marshalizer, arg.Hasher, gTX.RawData)
	if err != nil {
		return 0, nil, err
	}

	marshalizedTx, err := arg.Marshalizer.Marshal(gTX)
	if err != nil {
		return 0, nil, err
	}

	err = arg.Store.GetStorer(retriever.TransactionUnit).Put(txHash, marshalizedTx)
	if err != nil {
		return 0, nil, err
	}

	return len(initialAccounts), txHash, nil
}

func setBalanceToTrie(arg ArgsGenesisBlockCreator, accnt genesis.InitialAccountHandler, gTX *transaction.Transaction) error {
	accWrp, err := arg.Accounts.LoadAccount(accnt.AddressBytes())
	if err != nil {
		return err
	}

	account, ok := accWrp.(state.UserAccountHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	err = account.AddToBalance(accnt.GetBalanceValue(), nil, false)
	if err != nil {
		return err
	}

	klvTransfer := &transaction.TransferContract{
		ToAddress: accnt.AddressBytes(),
		AssetID:   kdautils.KLVIdentifier,
		Amount:    accnt.GetBalanceValue(),
	}

	err = gTX.PushContract(transaction.TXContract_TransferContractType, klvTransfer)
	if err != nil {
		return err
	}

	gTX.Receipts = append(gTX.Receipts, &transaction.Transaction_Receipt{
		Data: [][]byte{
			{byte(ptx.Transfer), byte(len(gTX.GetContracts()) - 1)},
			core.ZeroAddress,
			accnt.AddressBytes(),
			[]byte(strconv.FormatInt(accnt.GetBalanceValue(), 10)),
			kdautils.KLVIdentifier,
			nil,
			[]byte{byte(kapps.KDAData_Fungible)},
		},
	})

	if accnt.GetKFIBalanceValue() > 0 {
		err = account.AddToBalance(accnt.GetKFIBalanceValue(), kdautils.KFIIdentifier, false)
		if err != nil {
			return err
		}

		kfiTransfer := &transaction.TransferContract{
			ToAddress: accnt.AddressBytes(),
			AssetID:   kdautils.KFIIdentifier,
			Amount:    accnt.GetKFIBalanceValue(),
		}

		err = gTX.PushContract(transaction.TXContract_TransferContractType, kfiTransfer)
		if err != nil {
			return err
		}

		gTX.Receipts = append(gTX.Receipts, &transaction.Transaction_Receipt{
			Data: [][]byte{
				{byte(ptx.Transfer), byte(len(gTX.GetContracts()) - 1)},
				core.ZeroAddress,
				accnt.AddressBytes(),
				[]byte(strconv.FormatInt(accnt.GetKFIBalanceValue(), 10)),
				kdautils.KFIIdentifier,
				nil,
				[]byte{byte(kapps.KDAData_Fungible)},
			},
		})
	}

	return arg.Accounts.SaveAccount(account)
}

// setStakingToTrie staking info to trie
func setStakingToTries(arg ArgsGenesisBlockCreator) (int, [][]byte, error) {
	initialAccounts := arg.AccountsParser.InitialAccounts()
	electedNodesInfo, eligibleNodesInfo, err := arg.InitialNodesSetup.InitialNodesInfo()
	if err != nil {
		return 0, nil, err
	}

	validatorsInfo := make([]*state.ValidatorInfo, 0)

	initialNodesInfo := append(electedNodesInfo, eligibleNodesInfo...)

	kdaHandler, stakingHandler, err := getKAppHandlers(arg)
	if err != nil {
		return 0, nil, err
	}

	// compute initial supply
	initSupply := InitialSupply{}
	initSupply.KLV.Max = 10_000_000_000_000_000 // TODO: add to config
	initSupply.KFI.Max = 21_000_000_000_000     // TODO: add to config
	for _, accnt := range initialAccounts {
		initSupply.KLV.Initial += accnt.GetBalanceValue()
		if !check.IfNil(accnt.GetDelegationHandler()) {
			initSupply.KLV.Initial += accnt.GetDelegationHandler().GetValue()
		}
		initSupply.KFI.Initial += accnt.GetKFIBalanceValue()
	}

	if initSupply.KLV.Initial > initSupply.KLV.Max {
		return 0, nil, fmt.Errorf("%w: KLV", genesis.ErrEntireSupplyMismatch)
	}
	if initSupply.KFI.Initial > initSupply.KFI.Max {
		return 0, nil, fmt.Errorf("%w: KFI", genesis.ErrEntireSupplyMismatch)
	}

	err = initKLVAndKFIintoKapps(arg, initSupply, kdaHandler, stakingHandler)
	if err != nil {
		return 0, nil, err
	}

	klvStaking, err := getKLVStakingKApp(arg, stakingHandler)
	if err != nil {
		return 0, nil, err
	}

	var txHashes [][]byte
	var processedAccounts [][]byte
	for _, initialNode := range initialNodesInfo {
		var accnt genesis.InitialAccountHandler

		for i := range initialAccounts {
			if bytes.Equal(initialAccounts[i].AddressBytes(), initialNode.AddressBytes()) {
				accnt = initialAccounts[i]
				break
			}
		}

		if accnt == nil {
			return 0, nil, common.ErrGenesisNodeWithoutOwner
		}

		hash, err := setStakingToTrie(arg, klvStaking, stakingHandler, accnt, initialNode.PubKeyBytes())
		if err != nil {
			return 0, nil, err
		}

		if len(hash) > 0 {
			// append TX to list if any added
			txHashes = append(txHashes, hash)
		}

		validatorsInfo = append(validatorsInfo, &state.ValidatorInfo{
			OwnerAddress: accnt.AddressBytes(),
			PublicKey:    initialNode.PubKeyBytes(),
		})

		processedAccounts = append(processedAccounts, accnt.AddressBytes())
	}

	//process genesis accounts without a node
RemaindersAccLoop:
	for _, accnt := range initialAccounts {
		for _, processedAccnt := range processedAccounts {
			if bytes.Equal(accnt.AddressBytes(), processedAccnt) {
				continue RemaindersAccLoop
			}
		}

		hash, err := setStakingToTrie(arg, klvStaking, stakingHandler, accnt, nil)
		if err != nil {
			return 0, nil, err
		}

		if len(hash) > 0 {
			// append TX to list if any added
			txHashes = append(txHashes, hash)
		}
	}

	// initial update list
	err = arg.KAppController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(0, validatorsInfo)

	return len(initialAccounts), txHashes, err
}

// setPermissionsToTrie adds accounts permission into trie
func setPermissionsToTrie(arg ArgsGenesisBlockCreator) ([][]byte, error) {
	initialAccounts := arg.AccountsParser.InitialAccounts()

	var txHashes [][]byte
	for _, accnt := range initialAccounts {
		hash, err := setPermissionToTrie(arg, accnt)
		if err != nil {
			return nil, err
		}

		if len(hash) > 0 {
			// append TX to list if any added
			txHashes = append(txHashes, hash)
		}
	}

	return txHashes, nil
}

func setPermissionToTrie(
	arg ArgsGenesisBlockCreator,
	accnt genesis.InitialAccountHandler,
) ([]byte, error) {
	accWrp, err := arg.Accounts.LoadAccount(accnt.AddressBytes())
	if err != nil {
		return nil, err
	}

	account, ok := accWrp.(state.UserAccountHandler)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}

	var txHash []byte
	permissionsHandler := accnt.GetPermissionsHandler()
	if !permissionsHandler.IsInterfaceNil() {
		permsContracts := make([]*transaction.AccPermission, 0)
		perms := make([]*state.Permission, 0)
		for idx := 0; idx < permissionsHandler.Len(); idx++ {
			p := permissionsHandler.Get(idx)

			contractSigners := make([]*transaction.AccKey, 0)
			// load keys
			keys := make([]*state.Key, 0)
			for addr, weight := range p.GetSigners() {
				keys = append(keys, &state.Key{
					Address: []byte(addr),
					Weight:  weight,
				})

				contractSigners = append(contractSigners, &transaction.AccKey{
					Address: []byte(addr),
					Weight:  weight,
				})
			}

			// reorder keys by
			sort.SliceStable(keys, func(i, j int) bool {
				switch bytes.Compare(keys[i].Address, keys[j].Address) {
				case -1:
					return true
				case 0, 1:
					return false
				}
				return false
			})

			perm := &state.Permission{
				ID:             p.GetID(),
				Type:           state.Permission_PermissionType(p.GetType()),
				PermissionName: p.GetPermissionName(),
				Threshold:      p.GetThreshold(),
				Operations:     append([]byte{}, p.GetOperations()...),
				Signers:        keys,
			}
			// permission contract
			permsContracts = append(permsContracts, &transaction.AccPermission{
				Type:           transaction.AccPermission_AccPermissionType(p.GetType()),
				PermissionName: perm.PermissionName,
				Threshold:      perm.Threshold,
				Operations:     perm.Operations,
				Signers:        contractSigners,
			})

			perms = append(perms, perm)
		}
		account.SetPermissions(perms)

		// Add Contract and Permission
		permissionTransaction := &transaction.Transaction{
			RawData: &transaction.Transaction_Raw{
				Sender: account.AddressBytes(),
				Nonce:  account.GetNonce(),
			},
			Result:     transaction.Transaction_SUCCESS,
			ResultCode: transaction.Transaction_Ok,
			Receipts:   make([]*transaction.Transaction_Receipt, 0),
			Signature:  [][]byte{[]byte("klever genesis block")},
		}

		updatePermissionContract := &transaction.UpdateAccountPermissionContract{Permissions: permsContracts}

		if err = permissionTransaction.PushContract(transaction.TXContract_UpdateAccountPermissionContractType, updatePermissionContract); err != nil {
			return nil, err
		}
		permissionTransaction.Receipts = append(permissionTransaction.Receipts, &transaction.Transaction_Receipt{
			Data: [][]byte{
				{byte(ptx.UpdateAccountPermission), byte(len(permissionTransaction.GetContracts()) - 1)},
				accnt.AddressBytes(),
			},
		})

		txHash, err = tools.CalculateHash(arg.Marshalizer, arg.Hasher, permissionTransaction.RawData)
		if err != nil {
			return nil, err
		}
		marshalizedTx, err := arg.Marshalizer.Marshal(permissionTransaction)
		if err != nil {
			return nil, err
		}

		err = arg.Store.GetStorer(retriever.TransactionUnit).Put(txHash, marshalizedTx)
		if err != nil {
			return nil, err
		}

		// Increment account nonce to reflect the staking TX added in genesis TX
		account.IncreaseNonce(1)
		err = arg.Accounts.SaveAccount(account)
		if err != nil {
			return nil, err
		}
	}

	return txHash, nil
}

// TODO: refactor genesis creation
func setStakingToTrie(
	arg ArgsGenesisBlockCreator,
	klvStaking *kapps.StakingData,
	stakingHandler state.KAppAccountHandler,
	accnt genesis.InitialAccountHandler,
	peerAddress []byte,
) ([]byte, error) {
	delegation := accnt.GetDelegationHandler()
	if check.IfNil(delegation) {
		return nil, nil
	}

	accWrp, err := arg.Accounts.LoadAccount(accnt.AddressBytes())
	if err != nil {
		return nil, err
	}

	account, ok := accWrp.(state.UserAccountHandler)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}

	var txHash []byte
	if delegation.GetValue() > 0 {
		bucketID := kdautils.ToBucketID(arg.Hasher, []byte(arg.GenesisString), account.AddressBytes(), kdautils.KLVIdentifier, account.GetNonce(), 0, delegation.GetValue())

		userKDA, err := account.GetUserKDA(kdautils.KLVIdentifier, nil, false)
		if err != nil {
			return nil, err
		}

		err = account.Freeze(kdautils.KLVIdentifier, bucketID, delegation.GetValue(), arg.StartEpochNum, arg.GenesisTime, klvStaking, userKDA, true)
		if err != nil {
			return nil, err
		}

		stakingTransaction := &transaction.Transaction{
			RawData: &transaction.Transaction_Raw{
				Sender: account.AddressBytes(),
				Nonce:  account.GetNonce(),
			},
			Result:     transaction.Transaction_SUCCESS,
			ResultCode: transaction.Transaction_Ok,
			Receipts:   make([]*transaction.Transaction_Receipt, 0),
			Signature:  [][]byte{[]byte("klever genesis block")},
		}

		freezeContract := &transaction.FreezeContract{
			AssetID: kdautils.KLVIdentifier,
			Amount:  delegation.GetValue(),
		}

		delegationContract := &transaction.DelegateContract{
			BucketID:  bucketID,
			ToAddress: delegation.AddressBytes(),
		}

		err = stakingTransaction.PushContract(transaction.TXContract_FreezeContractType, freezeContract)
		if err != nil {
			return nil, err
		}

		stakingTransaction.Receipts = append(stakingTransaction.Receipts, &transaction.Transaction_Receipt{
			Data: [][]byte{
				{byte(ptx.Freeze), byte(len(stakingTransaction.GetContracts()) - 1)},
				bucketID,
				account.AddressBytes(),
				[]byte(kdautils.KLVIdentifier),
				[]byte(strconv.FormatInt(delegation.GetValue(), 10)),
			},
		})

		err = stakingTransaction.PushContract(transaction.TXContract_DelegateContractType, delegationContract)
		if err != nil {
			return nil, err
		}

		stakingTransaction.Receipts = append(stakingTransaction.Receipts, &transaction.Transaction_Receipt{
			Data: [][]byte{
				{byte(ptx.Delegate), byte(len(stakingTransaction.GetContracts()) - 1)},
				account.AddressBytes(),
				bucketID,
				delegation.AddressBytes(),
				[]byte(strconv.FormatInt(delegation.GetValue(), 10)),
			},
		})
		stakingTransaction.Receipts = append(stakingTransaction.Receipts, &transaction.Transaction_Receipt{
			Data: [][]byte{
				{byte(ptx.UpdateValidator), byte(len(stakingTransaction.GetContracts()) - 1)},
				delegation.AddressBytes(),
			},
		})

		txHash, err = tools.CalculateHash(arg.Marshalizer, arg.Hasher, stakingTransaction.RawData)
		if err != nil {
			return nil, err
		}
		marshalizedTx, err := arg.Marshalizer.Marshal(stakingTransaction)
		if err != nil {
			return nil, err
		}

		err = arg.Store.GetStorer(retriever.TransactionUnit).Put(txHash, marshalizedTx)
		if err != nil {
			return nil, err
		}

		err = setKLVStakingKApp(arg, stakingHandler, klvStaking)
		if err != nil {
			return nil, err
		}

		err = account.SetUserKDA(kdautils.KLVIdentifier, nil, userKDA)
		if err != nil {
			return nil, err
		}

		// Increment account nonce to reflect the staking TX added in genesis TX
		account.IncreaseNonce(1)
		err = arg.Accounts.SaveAccount(account)
		if err != nil {
			return nil, err
		}

		if len(peerAddress) > 0 {
			kappContext := kapp.NewKappContext(kapp.ArgsNewKAppContext{ContractID: 0, ContractType: transaction.TXContract_CreateValidatorContractType, Block: &block.Block{}, TxNonce: 0})
			arg.KAppController.SetCurrentKAppContext(kappContext)
			// Register Validator
			_, err = arg.KAppController.GetValidatorsKApp().Register(
				&transaction.CreateValidatorContract{
					OwnerAddress: account.AddressBytes(),
					Config: &transaction.ValidatorConfig{
						BLSPublicKey:        peerAddress,
						RewardAddress:       account.AddressBytes(),
						CanDelegate:         true,
						Commission:          0,
						MaxDelegationAmount: 0,
					},
				},
			)
			if err != nil {
				return nil, err
			}
		}

		// Delegate To Validator
		_, _, err = arg.KAppController.GetValidatorsKApp().Delegate(
			account.AddressBytes(),
			arg.GenesisTime,
			0,
			&transaction.DelegateContract{
				ToAddress: delegation.AddressBytes(),
				BucketID:  bucketID,
			},
		)
		if err != nil {
			return nil, err
		}

		err = arg.KAppController.GetValidatorsKApp().GetAccountsCacher().SaveAll()
		if err != nil {
			return nil, err
		}

		_, err = account.Delegate(bucketID, delegation.AddressBytes(), userKDA)
		if err != nil {
			return nil, err
		}

		err = account.SetUserKDA(kdautils.KLVIdentifier, nil, userKDA)
		if err != nil {
			return nil, err
		}

		err = arg.Accounts.SaveAccount(account)
		if err != nil {
			return nil, err
		}
	}

	return txHash, nil
}

// createGenesisBlock will create a genesis block
func createGenesisBlock(
	arg ArgsGenesisBlockCreator,
) (data.HeaderHandler, error) {
	numSetBalances, genesisTransfersHash, err := setBalancesToTrie(arg)
	if err != nil {
		return nil, fmt.Errorf("%w encountered when creating genesis block while setting the balances to trie", err)
	}

	numSetStaking, txHashes, err := setStakingToTries(arg)
	if err != nil {
		return nil, fmt.Errorf("%w encountered when creating genesis block while setting the staking to trie", err)
	}

	txHashesPermissions, err := setPermissionsToTrie(arg)
	if err != nil {
		return nil, fmt.Errorf("%w encountered when creating genesis block while setting account permissions to trie", err)
	}

	txHashes = append(txHashes, txHashesPermissions...)
	txHashes = append(txHashes, genesisTransfersHash)

	accountRootHash, err := arg.Accounts.Commit()
	if err != nil {
		return nil, err
	}

	kappRootHash, err := arg.KAppAccounts.Commit()
	if err != nil {
		return nil, err
	}

	validatorRootHash, err := arg.PeerAccounts.Commit()
	if err != nil {
		return nil, err
	}

	log.Info("genesisBlockCreator.StakingToTrie",
		"numSetStaking", numSetStaking,
		"accountRootHash", accountRootHash,
		"kappRootHash", kappRootHash,
		"validatorRootHash", validatorRootHash,
		"numSetBalances", numSetBalances,
	)

	slot, nonce, epoch := getGenesisBlocksSlotNonceEpoch(arg)

	magicDecoded, err := hex.DecodeString(arg.GenesisString)
	if err != nil {
		return nil, err
	}
	prevHash := arg.Hasher.Compute(arg.GenesisString)

	// TODO: create transactions and compute TxRootHash

	header := &block.Block{
		Header: &block.BlockHeader{
			TrieRoot:           accountRootHash,
			ValidatorsTrieRoot: validatorRootHash,
			KAppsTrieRoot:      kappRootHash,
			ParentHash:         prevHash,
			RandSeed:           accountRootHash,
			PrevRandSeed:       accountRootHash,
			ChainID:            []byte(arg.ChainID),
			SoftwareVersion:    []byte("0"),
			Timestamp:          arg.GenesisTime,
			Slot:               slot,
			Nonce:              nonce,
			Epoch:              epoch,
			IsEpochStart:       true,
			Reserved:           magicDecoded,
		},
		TxHashes: txHashes,
	}

	err = saveGenesisMetaToStorage(arg.Store, arg.Marshalizer, header)
	if err != nil {
		return nil, err
	}

	return header, nil
}

func saveGenesisMetaToStorage(
	storageService retriever.StorageService,
	marshalizer marshal.Marshalizer,
	genesisBlock data.HeaderHandler,
) error {

	epochStartID := core.EpochStartIdentifier(genesisBlock.GetEpoch())

	// TODO: FIXME: review... identifier for startEpoch same BlockUnit
	metaHdrStorage := storageService.GetStorer(retriever.BlockUnit)
	if check.IfNil(metaHdrStorage) {
		return common.ErrNilStorage
	}

	triggerStorage := storageService.GetStorer(retriever.BootstrapUnit)
	if check.IfNil(triggerStorage) {
		return common.ErrNilStorage
	}

	marshaledData, err := marshalizer.Marshal(genesisBlock)
	if err != nil {
		return err
	}

	err = metaHdrStorage.Put([]byte(epochStartID), marshaledData)
	if err != nil {
		return err
	}

	err = triggerStorage.Put([]byte(epochStartID), marshaledData)
	if err != nil {
		return err
	}

	return nil
}

func getKAppHandlers(arg ArgsGenesisBlockCreator) (state.KAppAccountHandler, state.KAppAccountHandler, error) {
	kdaAcc, err := arg.KAppAccounts.LoadAccount(kapps.KDAKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	kdaKapp, ok := kdaAcc.(state.KAppAccountHandler)
	if !ok {
		return nil, nil, common.ErrWrongTypeAssertion
	}

	stakingAcc, err := arg.KAppAccounts.LoadAccount(kapps.StakingKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	stakingKapp, ok := stakingAcc.(state.KAppAccountHandler)
	if !ok {
		return nil, nil, common.ErrWrongTypeAssertion
	}

	return kdaKapp, stakingKapp, nil
}

func getKLVStakingKApp(arg ArgsGenesisBlockCreator, stakingKapp state.KAppAccountHandler) (*kapps.StakingData, error) {
	klvStakedBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(kdautils.KLVKey)
	if err != nil {
		return nil, err
	}
	if len(klvStakedBytes) == 0 {
		return nil, common.ErrEmptyString
	}

	klvStaking := &kapps.StakingData{}
	err = arg.Marshalizer.Unmarshal(klvStaking, klvStakedBytes)
	if err != nil {
		return nil, err
	}

	return klvStaking, nil
}

func setKLVStakingKApp(arg ArgsGenesisBlockCreator, stakingKapp state.KAppAccountHandler, klvStaking *kapps.StakingData) error {
	klvData, err := arg.Marshalizer.Marshal(klvStaking)
	if err != nil {
		return err
	}
	err = stakingKapp.DataTrieTracker().SaveKeyValue(kdautils.KLVKey, klvData)
	if err != nil {
		return err
	}
	return arg.KAppAccounts.SaveAccount(stakingKapp)
}

// TODO: Move to a config file
func initKLVAndKFIintoKapps(arg ArgsGenesisBlockCreator, initialSupply InitialSupply, kdaKapp, stakingKapp state.KAppAccountHandler) error {
	klvStaking := &kapps.StakingData{
		InterestType: kapps.StakingData_FPRI,
		TotalStaked:  0,
		FPR: []*kapps.FPRData{
			{
				TotalAmount: 0,
				TotalStaked: 0,
				Epoch:       0,
			},
		},
		MinEpochsToClaim:    1,
		MinEpochsToUnstake:  1,
		MinEpochsToWithdraw: 2,
	}

	kfiStaking := &kapps.StakingData{
		InterestType: kapps.StakingData_FPRI,
		TotalStaked:  0,
		FPR: []*kapps.FPRData{
			{
				TotalAmount: 0,
				TotalStaked: 0,
				Epoch:       0,
			},
		},
		MinEpochsToUnstake:  1,
		MinEpochsToClaim:    1,
		MinEpochsToWithdraw: 2,
	}

	klvStakingData, err := arg.Marshalizer.Marshal(klvStaking)
	if err != nil {
		return err
	}

	kfiStakingData, err := arg.Marshalizer.Marshal(kfiStaking)
	if err != nil {
		return err
	}

	err = stakingKapp.DataTrieTracker().SaveKeyValue(kdautils.KLVKey, klvStakingData)
	if err != nil {
		return err
	}

	err = stakingKapp.DataTrieTracker().SaveKeyValue(kdautils.KFIKey, kfiStakingData)
	if err != nil {
		return err
	}

	URIs := map[string]string{
		"Website":    "https://klever.finance",
		"Whitepaper": "https://bc.klever.finance/wp",
		"Wallet":     "https://klever.finance/wallet",
		"Exchange":   "https://klever.io",
		"Github":     "https://github.com/klever-io",
		"Twitter":    "https://twitter.com/klever_io",
		"Instagram":  "https://instagram.com/klever.io",
	}

	klv := kapps.KDAData{
		ID:                kdautils.KLVIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("KLEVER"),
		Ticker:            kdautils.KLVIdentifier,
		OwnerAddress:      nil,
		Precision:         6,
		InitialSupply:     initialSupply.KLV.Initial,
		CirculatingSupply: initialSupply.KLV.Initial,
		MaxSupply:         initialSupply.KLV.Max,
		MintedValue:       initialSupply.KLV.Initial,
		IssueDate:         arg.GenesisTime,
		Royalties:         &kapps.RoyaltiesData{},
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: true,
		},
		Logo:  "https://bc.klever.finance/logo_klv",
		URIs:  URIs,
		Roles: make([]*kapps.RolesData, 0),
	}

	kfi := kapps.KDAData{
		ID:                kdautils.KFIIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("KLEVER FINANCE"),
		Ticker:            kdautils.KFIIdentifier,
		OwnerAddress:      nil,
		Precision:         6,
		InitialSupply:     initialSupply.KFI.Initial,
		CirculatingSupply: initialSupply.KFI.Initial,
		MaxSupply:         initialSupply.KFI.Max,
		MintedValue:       initialSupply.KFI.Initial,
		IssueDate:         arg.GenesisTime,
		Royalties:         &kapps.RoyaltiesData{},
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: true,
		},
		Logo:  "https://bc.klever.finance/logo_kfi",
		URIs:  URIs,
		Roles: make([]*kapps.RolesData, 0),
	}

	klvData, err := arg.Marshalizer.Marshal(&klv)
	if err != nil {
		return err
	}

	kfiData, err := arg.Marshalizer.Marshal(&kfi)
	if err != nil {
		return err
	}

	err = kdaKapp.DataTrieTracker().SaveKeyValue(kdautils.KLVKey, klvData)
	if err != nil {
		return err
	}
	err = kdaKapp.DataTrieTracker().SaveKeyValue(kdautils.KFIKey, kfiData)
	if err != nil {
		return err
	}

	initKapps := []*kapps.KDAData{&klv, &kfi}

	arg.Indexer.SaveAssets(initKapps)

	err = arg.KAppAccounts.SaveAccount(kdaKapp)
	if err != nil {
		return err
	}

	err = arg.KAppAccounts.SaveAccount(stakingKapp)
	if err != nil {
		return err
	}

	return nil
}
