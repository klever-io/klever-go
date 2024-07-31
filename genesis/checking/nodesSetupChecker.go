package checking

import (
	"fmt"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/genesis"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
)

const minimumAcceptedNodePrice = 0

type nodeSetupChecker struct {
	accountsParser           genesis.AccountsParser
	initialNodePrice         int64
	validatorPubkeyConverter core.PubkeyConverter
	keyGenerator             crypto.KeyGenerator
}

type delegationAddress struct {
	value   int64
	address string
}

// NewNodesSetupChecker will create a node setup checker able to check the initial nodes against the provided genesis values
func NewNodesSetupChecker(
	accountsParser genesis.AccountsParser,
	initialNodePrice int64,
	validatorPubkeyConverter core.PubkeyConverter,
	keyGenerator crypto.KeyGenerator,
) (*nodeSetupChecker, error) {
	if check.IfNil(accountsParser) {
		return nil, genesis.ErrNilAccountsParser
	}
	if initialNodePrice < minimumAcceptedNodePrice {
		return nil, fmt.Errorf("%w, minimum accepted is %d",
			genesis.ErrInvalidInitialNodePrice, minimumAcceptedNodePrice)
	}
	if check.IfNil(validatorPubkeyConverter) {
		return nil, genesis.ErrNilPubkeyConverter
	}
	if check.IfNil(keyGenerator) {
		return nil, genesis.ErrNilKeyGenerator
	}

	return &nodeSetupChecker{
		accountsParser:           accountsParser,
		initialNodePrice:         initialNodePrice,
		validatorPubkeyConverter: validatorPubkeyConverter,
		keyGenerator:             keyGenerator,
	}, nil
}

// Check will check that each and every initial node has a backed staking address
// also, it checks that the amount staked (either directly or delegated) matches exactly the total
// staked value defined in the genesis file
func (nsc *nodeSetupChecker) Check(initialNodes []sharding.GenesisNodeInfoHandler) error {
	err := nsc.ckeckGenesisNodes(initialNodes)
	if err != nil {
		return err
	}

	initialAccounts := nsc.getClonedInitialAccounts()
	delegated := nsc.createDelegatedValues(initialAccounts)

	return nsc.checkRemainderInitialAccounts(initialAccounts, delegated)
}

func (nsc *nodeSetupChecker) ckeckGenesisNodes(initialNodes []sharding.GenesisNodeInfoHandler) error {
	for _, node := range initialNodes {
		err := nsc.keyGenerator.CheckPublicKeyValid(node.PubKeyBytes())
		if err != nil {
			return fmt.Errorf("%w for node's public key `%s`, error: %s",
				genesis.ErrInvalidPubKey,
				nsc.validatorPubkeyConverter.Encode(node.PubKeyBytes()),
				err.Error(),
			)
		}
	}

	return nil
}

func (nsc *nodeSetupChecker) getClonedInitialAccounts() []genesis.InitialAccountHandler {
	initialAccounts := nsc.accountsParser.InitialAccounts()
	clonedInitialAccounts := make([]genesis.InitialAccountHandler, len(initialAccounts))

	for idx, ia := range initialAccounts {
		clonedInitialAccounts[idx] = ia.Clone()
	}

	return clonedInitialAccounts
}

// checkRemainderInitialAccounts checks that both staked value and delegated value is 0, meaning that all
// subtractions occurred perfectly
func (nsc *nodeSetupChecker) checkRemainderInitialAccounts(
	initialAccounts []genesis.InitialAccountHandler,
	delegated map[string]*delegationAddress,
) error {
	for _, delegation := range delegated {
		if delegation.value != 0 {
			return fmt.Errorf("%w for delegation address %s, remainder %d",
				genesis.ErrInvalidDelegationValue,
				delegation.address,
				delegation.value,
			)
		}
	}

	return nil
}

func (nsc *nodeSetupChecker) createDelegatedValues(initialAccounts []genesis.InitialAccountHandler) map[string]*delegationAddress {
	delegated := make(map[string]*delegationAddress)

	for _, ia := range initialAccounts {
		delegation := ia.GetDelegationHandler()
		if check.IfNil(delegation) {
			continue
		}
		delegationAddressBytes := delegation.AddressBytes()
		if len(delegationAddressBytes) == 0 {
			continue
		}

		delegatedAddr := delegated[string(delegationAddressBytes)]
		if delegatedAddr == nil {
			delegatedAddr = &delegationAddress{
				address: delegation.GetAddress(),
				value:   0,
			}

			delegated[string(delegationAddressBytes)] = delegatedAddr
		}

		delegatedAddr.value += delegation.GetValue()
		delegation.SetValue(0)
	}

	return delegated
}

// IsInterfaceNil returns if underlying object is true
func (nsc *nodeSetupChecker) IsInterfaceNil() bool {
	return nsc == nil
}
