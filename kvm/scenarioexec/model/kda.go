package scenjsonmodel

// KDATxData models the transfer of tokens in a tx
type KDATxData struct {
	TokenIdentifier JSONBytesFromString
	Nonce           JSONUint64
	Value           JSONBigInt
}

// KDAInstance models an instance of an NFT/SFT, with its own nonce
type KDAInstance struct {
	AssetType  JSONUint64
	Nonce      JSONUint64
	Balance    JSONBigInt
	Creator    JSONBytesFromString
	Royalties  JSONUint64
	Hash       JSONBytesFromString
	Uris       JSONValueList
	Attributes JSONBytesFromTree
	CanBurn    bool
}

// KDAData models an account holding an KDA token
type KDAData struct {
	TokenIdentifier JSONBytesFromString
	Instances       []*KDAInstance
	LastNonce       JSONUint64
	Roles           []string
	Frozen          JSONUint64
}

// CheckKDAInstance checks an instance of an NFT/SFT, with its own nonce
type CheckKDAInstance struct {
	Nonce      JSONUint64
	Balance    JSONCheckBigInt
	Creator    JSONCheckBytes
	Royalties  JSONCheckUint64
	Uris       JSONCheckValueList
	Attributes JSONCheckBytes
}

// NewCheckKDAInstance creates an instance with all fields unspecified.
func NewCheckKDAInstance() *CheckKDAInstance {
	return &CheckKDAInstance{
		Nonce:      JSONUint64Zero(),
		Balance:    JSONCheckBigIntUnspecified(),
		Creator:    JSONCheckBytesUnspecified(),
		Royalties:  JSONCheckUint64Unspecified(),
		Uris:       JSONCheckValueListUnspecified(),
		Attributes: JSONCheckBytesUnspecified(),
	}
}

// CheckKDAData checks the KDA tokens held by an account
type CheckKDAData struct {
	TokenIdentifier JSONBytesFromString
	Instances       []*CheckKDAInstance
	LastNonce       JSONCheckUint64
	Roles           []string
	Frozen          JSONCheckUint64
}
