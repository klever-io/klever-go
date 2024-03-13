//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. kda.proto
//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. staking.proto
//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. userKapps.proto
//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. proposal.proto
//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. ito.proto
//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. market.proto
package kapps

// StakingKAppAddress is the hard-coded address of the staking KApp
var StakingKAppAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255}

// KDAKAppAddress is the hard-coded address of the KDA KApp
var KDAKAppAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 255, 255}

// ProposalKAppAddress is the hard-coded address of the proposal KApp
var ProposalKAppAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 255, 255}

// ITOKAppAddress is the hard-coded address of the proposal KApp
var ITOKAppAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 255, 255}

// MarketKAppAddress is the hard-coded address of the proposal KApp
var MarketKAppAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 255, 255}

// ValidatorsKAppAddress is the hard-coded address of the proposal KApp
var ValidatorsKAppAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 255, 255}

// KDAFeesPoolKAppAddress is the hard-coded address of the token fees pool KApp
var KDAFeesPoolKAppAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 6, 255, 255}

var KAppsAddressList = map[string]string{
	string(StakingKAppAddress):     "Staking",
	string(KDAKAppAddress):         "KDA",
	string(ProposalKAppAddress):    "Proposal",
	string(ITOKAppAddress):         "ITO",
	string(MarketKAppAddress):      "Market",
	string(ValidatorsKAppAddress):  "Validators",
	string(KDAFeesPoolKAppAddress): "KDAFeesPool",
}

// Sp is the key separator
var Sp = "/"

// KDAPrefix is the key prefix of the kda/staking kapps
var KDAPrefix = "KDA"

// ProposalPrefix is the key prefix of the proposal kapp
var ProposalPrefix = "PROP"

// ITOPrefix is the key prefix of the ITO kapp
var ITOPrefix = "ITO"

// ITOWhitelistPrefix is the key prefix of the ITO kapp
var ITOWhitelistPrefix = "ITOWL"

// MarketOrderPrefix is the key prefix of the Market kapp
var MarketOrderPrefix = "MKT"

// MarketplacePrefix is the key prefix of the Market kapp
var MarketplacePrefix = "MKTPLACE"

// ProtectedKleverKeyPrefix is the key prefix which is protected from writing in the trie - only for special builtin functions
const ProtectedKleverKeyPrefix = "KLEVER"

// ProtectedKLVKeyPrefix is the key prefix which is protected from writing in the trie - only for special builtin functions
const ProtectedKLVKeyPrefix = "KLV"

// ProtectedKFIKeyPrefix is the key prefix which is protected from writing in the trie - only for special builtin functions
const ProtectedKFIKeyPrefix = "KFI"
