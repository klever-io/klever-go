package parsing

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/genesis/data"
	"github.com/klever-io/klever-go/genesis/mock"
)

func (ap *accountsParser) SetInitialAccounts(initialAccounts []*data.InitialAccount) {
	ap.initialAccounts = initialAccounts
}

func (ap *accountsParser) Process() error {
	return ap.process()
}

func (ap *accountsParser) SetPukeyConverter(pubkeyConverter core.PubkeyConverter) {
	ap.pubkeyConverter = pubkeyConverter
}

func (ap *accountsParser) SetKeyGenerator(keyGen crypto.KeyGenerator) {
	ap.keyGenerator = keyGen
}

func NewTestAccountsParser(pubkeyConverter core.PubkeyConverter) *accountsParser {
	return &accountsParser{
		pubkeyConverter: pubkeyConverter,
		initialAccounts: make([]*data.InitialAccount, 0),
		keyGenerator:    &mock.KeyGeneratorStub{},
	}
}
