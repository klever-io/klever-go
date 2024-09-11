package blockchain

import (
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
)

func Proposal(fromAddr, description string, parameters map[int32]string, duration uint32) (string, error) {
	proposal := models.ProposalTXRequest{
		Parameters:     parameters,
		EpochsDuration: duration,
		Description:    description,
	}

	data, err := buildRequest(transaction.TXContract_ProposalContractType, fromAddr, []interface{}{proposal})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}

func Vote(fromAddr string, proposalID uint64, amount float64, voteType uint64) (string, error) {
	vote := models.VoteTXRequest{
		Type:       uint32(voteType), // #nosec G115 - type casting
		ProposalID: proposalID,
		Amount:     int64(amount * 1000000),
	}

	data, err := buildRequest(transaction.TXContract_VoteContractType, fromAddr, []interface{}{vote})
	if err != nil {
		return "", err
	}

	return sendSignAndBroadcast(data)
}
