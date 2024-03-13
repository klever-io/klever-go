package integrationTest

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	ptx "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	"github.com/klever-io/klever-go/kapps"
)

// #################################### Minting Data in Nodes ####################################

func MintAllNodes(nodes []*processorNode.ProcessorNode, initialBalance *big.Int) {
	for _, node := range nodes {
		node.NodeAccount.Balance = big.NewInt(0).Set(initialBalance)
		_ = MintAddress(node, node.NodeAccount.Balance.Int64(), node.NodeAccount.Address)
	}
}

func MintAddress(node *processorNode.ProcessorNode, balance int64, address []byte) error {
	acc, err := node.AccountsAdapter.LoadAccount(address)
	if err != nil {
		return err
	}

	err = acc.(state.UserAccountHandler).AddToBalance(balance, nil)
	if err != nil {
		return err
	}

	err = node.AccountsAdapter.SaveAccount(acc)
	if err != nil {
		return err
	}

	_, err = node.AccountsAdapter.Commit()
	if err != nil {
		return err
	}

	return nil
}

func MintAllWallets(nodes []*processorNode.ProcessorNode, wallets []*processorNode.NodeAccount, balance int64) {
	for _, wallet := range wallets {
		for _, node := range nodes {
			err := MintAddress(node, balance, wallet.Address)
			if err != nil {
				log.Info(fmt.Sprintf("mint wallets - error: %s", err.Error()))
			}
		}
	}
}

// #################################### Retrieve data from nodes ####################################

func GetUserAccount(
	node *processorNode.ProcessorNode,
	address []byte,
) state.UserAccountHandler {
	acc, _ := node.AccountsAdapter.GetExistingAccount(address)
	userAcc := acc.(state.UserAccountHandler)
	return userAcc
}

func GetStaking(
	node *processorNode.ProcessorNode,
	assetId []byte,
) (state.KAppAccountHandler, *kapps.StakingData, error) {
	stakingKapp, err := node.AccountsCacher.LoadKApp(kapps.StakingKAppAddress)
	if err != nil {
		return nil, nil, err
	}
	key := kdautils.ToKDAKey(assetId, nil)

	stakedBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, nil, err
	}
	if len(stakedBytes) == 0 {
		return nil, nil, common.ErrStakingNotFound
	}

	kdaStaking := &kapps.StakingData{}
	err = node.InternalMarshalizer.Unmarshal(kdaStaking, stakedBytes)
	if err != nil {
		return nil, nil, err
	}

	return stakingKapp, kdaStaking, err
}

func GetAsset(node *processorNode.ProcessorNode, assetId string) (*kapps.KDAData, error) {
	kda := &kapps.KDAData{}

	assetKDAaccount, err := node.AccountsCacher.GetExistingKapp(kapps.KDAKAppAddress)
	if err != nil {
		return nil, err
	}

	key := kdautils.ToKDAKey([]byte(assetId), nil)

	kdaBytes, err := assetKDAaccount.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(kdaBytes) == 0 {
		return nil, common.ErrEmptyString
	}

	err = node.InternalMarshalizer.Unmarshal(kda, kdaBytes)
	if err != nil {
		return nil, err
	}

	return kda, nil
}

func GetAssetId(receipts []*transaction.Transaction_Receipt) (string, error) {
	for _, r := range receipts {
		if r != nil && len(r.GetData()) >= 2 {
			switch r.Data[0][0] {
			case 1:
				return string(r.Data[1]), nil
			}
		}
	}

	return "", errors.New("cannot find asset id")
}

func GetMarketplaceId(receipts []*transaction.Transaction_Receipt) (string, error) {
	for _, r := range receipts {
		if r != nil {
			if len(r.GetData()) >= 2 {

				if ptx.ReceiptType(r.GetData()[0][0]) == ptx.CreateMarketplace {
					return hex.EncodeToString(r.GetData()[1]), nil
				}
			}
		}
	}

	return "", errors.New("cannot find marketplace id")
}

func GetOrderID(receipts []*transaction.Transaction_Receipt) (string, error) {
	for _, r := range receipts {
		if r != nil {
			if len(r.GetData()) >= 2 {
				if ptx.ReceiptType(r.GetData()[0][0]) == ptx.Sell {
					return hex.EncodeToString(r.GetData()[1]), nil
				}
			}
		}
	}

	return "", errors.New("cannot find order id")
}

func GetKDAPool(node *processorNode.ProcessorNode, assetId string) (*kdafeespool.KDAFeesPoolData, error) {
	kdaPool := &kdafeespool.KDAFeesPoolData{}

	kdaPoolAccount, err := node.KappsAdapter.LoadAccount(kapps.KDAFeesPoolKAppAddress)
	if err != nil {
		return nil, err
	}

	kdaPoolKapp, ok := kdaPoolAccount.(state.KAppAccountHandler)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}

	poolBytes, err := kdaPoolKapp.DataTrieTracker().RetrieveValue([]byte(assetId))
	if err != nil {
		return nil, err
	}

	err = node.InternalMarshalizer.Unmarshal(kdaPool, poolBytes)
	if err != nil {
		return nil, err
	}

	return kdaPool, nil
}

// #################################### Updating Slot ####################################

// IncrementAndPrintSlot increments the given variable, and prints the message for the beginning of the slot
func IncrementAndPrintSlot(slot uint64) uint64 {
	slot++
	log.Info(fmt.Sprintf("#################################### SLOT %d BEGINS ####################################", slot))

	return slot
}

// UpdateSlot updates the slot for every node
func UpdateSlot(nodes []*processorNode.ProcessorNode, slot uint64) {
	for _, n := range nodes {
		n.SlotManager.SlotIndex = int64(slot)
	}
}

// #################################### Display nodes ####################################

// DisplayAndStartNodes prints each nodes shard ID, sk and pk, and then starts the node
func DisplayNodes(nodes []*processorNode.ProcessorNode) {
	fmt.Println("len nodes :: ", len(nodes))
	for _, n := range nodes {
		// skTxBuff, _ := n.NodeBlockSignKeyPair.Sk.ToByteArray()
		// pkTxBuff, _ := n.NodeBlockSignKeyPair.Pk.ToByteArray()
		pkNode := n.NodesCoordinator.GetOwnPublicKey()

		log.Info(fmt.Sprintf("pkNode: %s",
			processorNode.TestValidatorPubkeyConverter.Encode(pkNode)))

		// log.Info(fmt.Sprintf("skTx: %s, pkTx: %s",
		// 	hex.EncodeToString(skTxBuff),
		// 	processorNode.TestAddressPubkeyConverter.Encode(pkTxBuff)))
	}

	log.Info("Delaying for node bootstrap and topic announcement...")
	time.Sleep(P2pBootstrapDelay)
}

// #################################### Load Config ####################################

func LoadConfig(relativePath string) (config.Config, error) {
	return config.LoadFromPath(relativePath)
}

func LoadEnableEpochsConfig(relativePath string) (config.EnableEpochsConfig, error) {
	return config.LoadEnableEpochsConfig(relativePath)
}
