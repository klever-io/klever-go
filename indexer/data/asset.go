package data

type Asset struct {
	AssetType         string          `json:"assetType"`
	AssetID           string          `json:"assetId"`
	Name              string          `json:"name"`
	Ticker            string          `json:"ticker"`
	OwnerAddress      string          `json:"ownerAddress"`
	AdminAddress      string          `json:"adminAddress"`
	Logo              string          `json:"logo"`
	URIs              []*URI          `json:"uris"`
	Precision         uint32          `json:"precision"`
	InitialSupply     int64           `json:"initialSupply"`
	CirculatingSupply int64           `json:"circulatingSupply"`
	MaxSupply         int64           `json:"maxSupply"`
	MintedValue       int64           `json:"mintedValue"`
	BurnedValue       int64           `json:"burnedValue"`
	IssueDate         int64           `json:"issueDate"`
	Royalties         *RoyaltiesInfo  `json:"royalties"`
	Staking           *StakingData    `json:"staking,omitempty"`
	Properties        *PropertiesInfo `json:"properties"`
	Attributes        *AttributesInfo `json:"attributes"`
	Roles             []*RolesInfo    `json:"roles,omitempty"`
	Hidden            *bool           `json:"hidden,omitempty"`
	IsSFT             *bool           `json:"isSFT,omitempty"`
	Verified          *bool           `json:"verified,omitempty"`
	Tags              []string        `json:"tags,omitempty"`
	Metadata          *Meta           `json:"meta,omitempty"`
	HasKdaPool        bool            `json:"hasKdaPool"`
}

type ITOInfo struct {
	IsActive               bool             `json:"isActive"`
	MaxAmount              int64            `json:"maxAmount,omitempty"`
	MintedAmount           int64            `json:"mintedAmount,omitempty"`
	ReceiverAddress        string           `json:"receiverAddress,omitempty"`
	PackData               []*PackInfo      `json:"packData,omitempty"`
	DefaultLimitPerAddress int64            `json:"defaultLimitPerAddress,omitempty"`
	IsWhitelistActive      bool             `json:"isWhitelistActive,omitempty"`
	WhitelistInfo          []*WhitelistInfo `json:"whitelistInfo,omitempty"`
	WhitelistStartTime     int64            `json:"whitelistStartTime,omitempty"`
	WhitelistEndTime       int64            `json:"whitelistEndTime,omitempty"`
	StartTime              int64            `json:"startTime,omitempty"`
	EndTime                int64            `json:"endTime,omitempty"`
	AssetID                string           `json:"assetId"`
	Timestamp              int64            `json:"timestamp"`
}

func (i *ITOInfo) ConvertToMap() map[string]*WhitelistInfo {
	result := make(map[string]*WhitelistInfo)

	for _, whiteList := range i.WhitelistInfo {
		result[whiteList.Address] = whiteList
	}

	return result
}

func (i *ITOInfo) SetWhiteListFromMap(whitelist map[string]*WhitelistInfo) {
	var list []*WhitelistInfo

	for _, w := range whitelist {
		list = append(list, w)
	}

	i.WhitelistInfo = list
}

type CachedAsset struct {
	Type       string
	Collection string
	Precision  uint32
}
