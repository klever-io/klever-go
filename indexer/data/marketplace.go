package data

type Marketplace struct {
	ID                 string `json:"id"`
	Name               string `json:"name,omitempty"`
	OwnerAddress       string `json:"ownerAddress,omitempty"`
	ReferralAddress    string `json:"referralAddress,omitempty"`
	ReferralPercentage uint32 `json:"referralPercentage,omitempty"`
}
