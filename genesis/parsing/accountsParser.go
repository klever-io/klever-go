package parsing

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/genesis"
	"github.com/klever-io/klever-go/genesis/data"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

// accountsParser hold data for initial accounts decoded data from json file
type accountsParser struct {
	initialAccounts []*data.InitialAccount
	pubkeyConverter core.PubkeyConverter
	keyGenerator    crypto.KeyGenerator
}

// NewAccountsParser creates a new decoded accounts genesis structure from json config file
func NewAccountsParser(
	genesisFilePath string,
	pubkeyConverter core.PubkeyConverter,
	keyGenerator crypto.KeyGenerator,
) (*accountsParser, error) {
	if check.IfNil(pubkeyConverter) {
		return nil, genesis.ErrNilPubkeyConverter
	}
	if check.IfNil(keyGenerator) {
		return nil, genesis.ErrNilKeyGenerator
	}

	initialAccounts := make([]*data.InitialAccount, 0)
	err := tools.LoadJSONFile(&initialAccounts, genesisFilePath)
	if err != nil {
		return nil, err
	}

	gp := &accountsParser{
		initialAccounts: initialAccounts,
		pubkeyConverter: pubkeyConverter,
		keyGenerator:    keyGenerator,
	}

	err = gp.process()
	if err != nil {
		return nil, err
	}

	return gp, nil
}

func (ap *accountsParser) process() error {
	totalSupply := int64(0)
	for _, initialAccount := range ap.initialAccounts {
		err := ap.parseElement(initialAccount)
		if err != nil {
			return err
		}

		supply, err := ap.checkInitialAccount(initialAccount)
		if err != nil {
			return err
		}

		totalSupply += supply
	}

	err := ap.checkForDuplicates()
	if err != nil {
		return err
	}

	return nil
}

func (ap *accountsParser) parseElement(initialAccount *data.InitialAccount) error {
	if len(initialAccount.Address) == 0 {
		return genesis.ErrEmptyAddress
	}
	addressBytes, err := ap.pubkeyConverter.Decode(initialAccount.Address)
	if err != nil {
		return fmt.Errorf("%w for `%s`", genesis.ErrInvalidAddress, initialAccount.Address)
	}

	err = ap.keyGenerator.CheckPublicKeyValid(addressBytes)
	if err != nil {
		return fmt.Errorf("%w for `%s`, error: %s",
			genesis.ErrInvalidPubKey,
			initialAccount.Address,
			err.Error(),
		)
	}

	initialAccount.SetAddressBytes(addressBytes)

	err = ap.parseDelegationElement(initialAccount)
	if err != nil {
		return err
	}

	return ap.parsePermissionElement(initialAccount)
}

func (ap *accountsParser) parseDelegationElement(initialAccount *data.InitialAccount) error {
	delegationData := initialAccount.Delegation

	if delegationData == nil || delegationData.Value == 0 {
		return nil
	}

	if len(delegationData.Address) == 0 {
		return fmt.Errorf("%w for address '%s'",
			genesis.ErrEmptyDelegationAddress, initialAccount.Address)
	}
	addressBytes, err := ap.pubkeyConverter.Decode(delegationData.Address)
	if err != nil {
		return fmt.Errorf("%w for `%s`, address %s, error %s",
			genesis.ErrInvalidDelegationAddress,
			delegationData.Address,
			initialAccount.Address,
			err.Error(),
		)
	}

	delegationData.SetAddressBytes(addressBytes)

	return nil
}

func (ap *accountsParser) parsePermissionElement(initialAccount *data.InitialAccount) error {
	permissionData := initialAccount.Permissions

	if permissionData == nil || permissionData.Len() == 0 {
		return nil
	}

	for i := 0; i < permissionData.Len(); i++ {
		p := permissionData.Get(i).(*data.Permission)
		totalWeight := int64(0)
		p.SignersBytes = make(map[string]int64, 0)
		for addr, weight := range p.Signers {
			totalWeight += weight
			addressBytes, err := ap.pubkeyConverter.Decode(addr)
			if err != nil {
				return fmt.Errorf("%w for `%s`, address %s, error %s",
					genesis.ErrInvalidSignerAddress,
					addr,
					initialAccount.Address,
					err.Error(),
				)
			}
			p.SignersBytes[string(addressBytes)] = weight
		}

		if totalWeight < p.Threshold {
			return fmt.Errorf("%w for address '%s'",
				genesis.ErrPermissionThreshold, initialAccount.Address)
		}
		p.ID = int32(i) // #nosec G115

		if len(p.Operations) > 0 {
			var err error
			p.OperationsBytes, err = hex.DecodeString(p.Operations)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (ap *accountsParser) checkInitialAccount(initialAccount *data.InitialAccount) (int64, error) {
	if initialAccount.Balance < 0 {
		return 0, fmt.Errorf("%w for '%d', address %s",
			genesis.ErrInvalidBalance,
			initialAccount.Balance,
			initialAccount.Address,
		)
	}
	sum := initialAccount.Balance

	if initialAccount.Delegation != nil {
		if initialAccount.Delegation.Value < 0 {
			return 0, fmt.Errorf("%w for '%d', address %s",
				genesis.ErrInvalidDelegationValue,
				initialAccount.Delegation.Value,
				initialAccount.Address,
			)
		}
		sum += initialAccount.Delegation.Value
	}

	return sum, nil
}

func (ap *accountsParser) checkForDuplicates() error {
	for idx1 := 0; idx1 < len(ap.initialAccounts); idx1++ {
		ia1 := ap.initialAccounts[idx1]
		for idx2 := idx1 + 1; idx2 < len(ap.initialAccounts); idx2++ {
			ia2 := ap.initialAccounts[idx2]
			if ia1.Address == ia2.Address {
				return fmt.Errorf("%w found for '%s'",
					genesis.ErrDuplicateAddress,
					ia1.Address,
				)
			}
		}
	}

	return nil
}

// InitialAccounts return the initial accounts contained by this parser
func (ap *accountsParser) InitialAccounts() []genesis.InitialAccountHandler {
	accounts := make([]genesis.InitialAccountHandler, len(ap.initialAccounts))

	for idx, ia := range ap.initialAccounts {
		accounts[idx] = ia
	}

	return accounts
}

// GetTotalStakedForDelegationAddress returns the total staked value for a provided delegation address
func (ap *accountsParser) GetTotalStakedForDelegationAddress(delegationAddress string) int64 {
	sum := int64(0)

	for _, in := range ap.initialAccounts {
		if in.Delegation.Address == delegationAddress {
			sum += in.Delegation.Value
		}
	}

	return sum
}

// GetInitialAccountsForDelegated returns the initial accounts that are set to the provided delegated address
func (ap *accountsParser) GetInitialAccountsForDelegated(addressBytes []byte) []genesis.InitialAccountHandler {
	list := make([]genesis.InitialAccountHandler, 0)
	for _, ia := range ap.initialAccounts {
		if bytes.Equal(ia.Delegation.AddressBytes(), addressBytes) {
			list = append(list, ia)
		}
	}

	return list
}

// IsInterfaceNil returns if underlying object is true
func (ap *accountsParser) IsInterfaceNil() bool {
	return ap == nil
}
