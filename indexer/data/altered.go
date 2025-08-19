package data

type AlteredData struct {
	Accounts     AlteredAccountsHandler
	Assets       AlteredAssetsHandler
	Proposals    AlteredProposalsHandler
	Marketplaces AlteredMarketplacesHandler
	KDAPool      AlteredKDAPoolHandler
	Orders       MarketOperationsHandler
	ITO          AlteredITOHandler
	SC           AlteredSmartContractsHandler
}

func NewAlteredData() *AlteredData {
	return &AlteredData{
		Accounts:     NewAlteredAccounts(),
		Assets:       NewAlteredAssets(),
		Proposals:    NewAlteredProposals(),
		Marketplaces: NewAlteredMarketplaces(),
		KDAPool:      NewAlteredKDAPools(),
		Orders:       NewMarketOperations(),
		ITO:          NewAlteredITOs(),
		SC:           NewAlteredSmartContracts(),
	}
}

// AlteredAccount is a structure that holds information about an altered account
type AlteredAccount struct {
	BalanceChange   bool
	IsSender        bool
	IsKDAOperation  bool
	IsNFTOperation  bool
	IsSmartContract bool
	TokenIdentifier string
	NFTNonce        string
	Type            string
	MarketplaceID   string
	OrderID         string
}

// AlteredAccountsHandler defines the actions that an altered accounts handler should do
type AlteredAccountsHandler interface {
	Add(key string, account *AlteredAccount)
	Get(key string) ([]*AlteredAccount, bool)
	GetAll() map[string][]*AlteredAccount
	Len() int
	IsInterfaceNil() bool
}

type alteredAccounts struct {
	altered map[string][]*AlteredAccount
}

// NewAlteredAccounts will create a new instance of alteredAccounts
func NewAlteredAccounts() *alteredAccounts {
	return &alteredAccounts{
		altered: make(map[string][]*AlteredAccount),
	}
}

func (aa *alteredAccounts) Add(key string, account *AlteredAccount) {
	_, ok := aa.altered[key]
	if !ok {
		aa.altered[key] = make([]*AlteredAccount, 0)
		aa.altered[key] = append(aa.altered[key], account)

		return
	}

	isTokenOperation := account.IsKDAOperation || account.IsNFTOperation
	if !isTokenOperation {
		aa.altered[key][0].IsSender = aa.altered[key][0].IsSender || account.IsSender
		return
	}

	senderCount := 0
	for _, elem := range aa.altered[key] {
		newElementIsTokenOperation := account.IsKDAOperation || account.IsNFTOperation
		oldElementIsTokenOperation := elem.IsKDAOperation || elem.IsNFTOperation

		isSender := elem.IsSender || account.IsSender
		if isSender {
			senderCount++
		}

		shouldRewrite := newElementIsTokenOperation && !oldElementIsTokenOperation
		if shouldRewrite {
			elem.TokenIdentifier = account.TokenIdentifier
			elem.NFTNonce = account.NFTNonce
			elem.IsNFTOperation = account.IsNFTOperation
			elem.IsKDAOperation = account.IsKDAOperation
			elem.IsSender = isSender
			elem.Type = account.Type
			return
		}

		alreadyExists := elem.TokenIdentifier == account.TokenIdentifier && elem.NFTNonce == account.NFTNonce
		if alreadyExists {
			elem.IsSender = (elem.IsSender || account.IsSender) && senderCount == 1
			return
		}
	}

	if senderCount > 0 {
		// set isSender to false because regular balance change was already countered
		account.IsSender = false
	}

	aa.altered[key] = append(aa.altered[key], account)
}

func (aa *alteredAccounts) Get(key string) ([]*AlteredAccount, bool) {
	altered, ok := aa.altered[key]

	return altered, ok
}

func (aa *alteredAccounts) Len() int {
	return len(aa.altered)
}

func (aa *alteredAccounts) GetAll() map[string][]*AlteredAccount {
	if aa == nil || aa.altered == nil {
		return map[string][]*AlteredAccount{}
	}

	return aa.altered
}

// IsInterfaceNil returns true if underlying object is nil
func (aa *alteredAccounts) IsInterfaceNil() bool {
	return aa == nil
}

// AlteredProposal
type AlteredProposal struct {
	IsNew bool
}

// AlteredProposalsHandler defines the actions that an altered proposals handler should do
type AlteredProposalsHandler interface {
	Add(key string, proposal *AlteredProposal)
	Get(key string) (*AlteredProposal, bool)
	GetAll() map[string]*AlteredProposal
	Len() int
	IsInterfaceNil() bool
}

type alteredProposals struct {
	altered map[string]*AlteredProposal
}

// NewAlteredProposals will create a new instance of alteredProposals
func NewAlteredProposals() *alteredProposals {
	return &alteredProposals{
		altered: make(map[string]*AlteredProposal),
	}
}

func (aa *alteredProposals) Add(key string, proposal *AlteredProposal) {
	aa.altered[key] = proposal
}

func (aa *alteredProposals) Get(key string) (*AlteredProposal, bool) {
	altered, ok := aa.altered[key]

	return altered, ok
}

func (aa *alteredProposals) Len() int {
	return len(aa.altered)
}

func (aa *alteredProposals) GetAll() map[string]*AlteredProposal {
	if aa == nil || aa.altered == nil {
		return map[string]*AlteredProposal{}
	}

	return aa.altered
}

// IsInterfaceNil returns true if underlying object is nil
func (aa *alteredProposals) IsInterfaceNil() bool {
	return aa == nil
}

// AlteredMarketplaces
type AlteredMarketplaces struct {
	IsNew bool
}

// AlteredMarketplacesHandler defines the actions that an altered Marketplaces handler should do
type AlteredMarketplacesHandler interface {
	Add(key string, proposal *AlteredMarketplaces)
	Get(key string) (*AlteredMarketplaces, bool)
	GetAll() map[string]*AlteredMarketplaces
	Len() int
	IsInterfaceNil() bool
}

type alteredMarketplaces struct {
	altered map[string]*AlteredMarketplaces
}

// NewAlteredMarketplaces will create a new instance of alteredMarketplaces
func NewAlteredMarketplaces() *alteredMarketplaces {
	return &alteredMarketplaces{
		altered: make(map[string]*AlteredMarketplaces),
	}
}

func (am *alteredMarketplaces) Add(key string, market *AlteredMarketplaces) {
	am.altered[key] = market
}

func (am *alteredMarketplaces) Get(key string) (*AlteredMarketplaces, bool) {
	altered, ok := am.altered[key]

	return altered, ok
}

func (am *alteredMarketplaces) Len() int {
	return len(am.altered)
}

func (am *alteredMarketplaces) GetAll() map[string]*AlteredMarketplaces {
	if am == nil || am.altered == nil {
		return map[string]*AlteredMarketplaces{}
	}

	return am.altered
}

// IsInterfaceNil returns true if underlying object is nil
func (am *alteredMarketplaces) IsInterfaceNil() bool {
	return am == nil
}

// AlteredAsset
type AlteredAsset struct {
	IsNew bool
}

// AlteredAssetsHandler defines the actions that an altered assets handler should do
type AlteredAssetsHandler interface {
	Add(key string, asset *AlteredAsset)
	Get(key string) ([]*AlteredAsset, bool)
	GetAll() map[string][]*AlteredAsset
	Len() int
	IsInterfaceNil() bool
}

type alteredAssets struct {
	altered map[string][]*AlteredAsset
}

// NewAlteredAssets will create a new instance of alteredAssets
func NewAlteredAssets() *alteredAssets {
	return &alteredAssets{
		altered: make(map[string][]*AlteredAsset),
	}
}

func (aa *alteredAssets) Add(key string, asset *AlteredAsset) {
	_, ok := aa.altered[key]
	if !ok {
		aa.altered[key] = make([]*AlteredAsset, 0)
		aa.altered[key] = append(aa.altered[key], asset)
		return
	}

	for _, elem := range aa.altered[key] {
		alreadyExists := elem.IsNew == asset.IsNew
		if alreadyExists {
			return
		}
	}

	aa.altered[key] = append(aa.altered[key], asset)
}

func (aa *alteredAssets) Get(key string) ([]*AlteredAsset, bool) {
	altered, ok := aa.altered[key]

	return altered, ok
}

func (aa *alteredAssets) Len() int {
	return len(aa.altered)
}

func (aa *alteredAssets) GetAll() map[string][]*AlteredAsset {
	if aa == nil || aa.altered == nil {
		return map[string][]*AlteredAsset{}
	}

	return aa.altered
}

// IsInterfaceNil returns true if underlying object is nil
func (aa *alteredAssets) IsInterfaceNil() bool {
	return aa == nil
}

// AlteredKDAPool
type AlteredKDAPool struct {
	IsNew bool
}

// AlteredKDAPoolHandler defines the actions that an altered KDA pool handler should do
type AlteredKDAPoolHandler interface {
	Add(key string, pool *AlteredKDAPool)
	Get(key string) ([]*AlteredKDAPool, bool)
	GetAll() map[string][]*AlteredKDAPool
	Len() int
	IsInterfaceNil() bool
}

type alteredKDAPools struct {
	altered map[string][]*AlteredKDAPool
}

// NewAlteredKDAPools will create a new instance of alteredKDAPools
func NewAlteredKDAPools() *alteredKDAPools {
	return &alteredKDAPools{
		altered: make(map[string][]*AlteredKDAPool),
	}
}

func (aa *alteredKDAPools) Add(key string, pool *AlteredKDAPool) {
	_, ok := aa.altered[key]
	if !ok {
		aa.altered[key] = make([]*AlteredKDAPool, 0)
		aa.altered[key] = append(aa.altered[key], pool)
		return
	}

	for _, elem := range aa.altered[key] {
		alreadyExists := elem.IsNew == pool.IsNew
		if alreadyExists {
			return
		}
	}

	aa.altered[key] = append(aa.altered[key], pool)
}

func (aa *alteredKDAPools) Get(key string) ([]*AlteredKDAPool, bool) {
	altered, ok := aa.altered[key]

	return altered, ok
}

func (aa *alteredKDAPools) Len() int {
	return len(aa.altered)
}

func (aa *alteredKDAPools) GetAll() map[string][]*AlteredKDAPool {
	if aa == nil || aa.altered == nil {
		return map[string][]*AlteredKDAPool{}
	}

	return aa.altered
}

// IsInterfaceNil returns true if underlying object is nil
func (aa *alteredKDAPools) IsInterfaceNil() bool {
	return aa == nil
}
