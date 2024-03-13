package data

import "time"

// Proposal is a structure containing a proposal information
type Proposal struct {
	ProposalID     uint64           `json:"proposalId"`
	Proposer       string           `json:"proposer"`
	TXHash         string           `json:"txHash"`
	ProposalStatus string           `json:"proposalStatus"`
	Parameters     map[int32]string `json:"parameters"`
	OldParameters  map[int32]string `json:"oldParameters"`
	Description    string           `json:"description"`
	EpochStart     uint32           `json:"epochStart"`
	EpochEnd       uint32           `json:"epochEnd"`
	Votes          map[int32]int64  `json:"votes"`
	Timestamp      time.Duration    `json:"timestamp"`
	Voters         []*VotersInfo    `json:"voters"`
	TotalStaked    int64            `json:"totalStaked"`
}

// Votes is a structure containing a votes information
type VotersInfo struct {
	Address   string        `json:"address"`
	Type      int32         `json:"type"`
	Amount    int64         `json:"amount"`
	Timestamp time.Duration `json:"timestamp"`
}

type ProposalParameters struct {
	Parameters map[string]*Parameter `json:"proposalParameters"`
}

type Parameter struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}
