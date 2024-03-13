package scenjsonparse

import (
	"errors"
	"fmt"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func (p *Parser) processBlock(blockRaw orderedjson.OJsonObject) (*scenjsonmodel.Block, error) {
	blockMap, isMap := blockRaw.(*orderedjson.OJsonMap)
	if !isMap {
		return nil, errors.New("unmarshalled block object is not a map")
	}
	bl := scenjsonmodel.Block{}

	for _, kvp := range blockMap.OrderedKV {
		switch kvp.Key {
		case "results":
			resultsRaw, resultsOk := kvp.Value.(*orderedjson.OJsonList)
			if !resultsOk {
				return nil, errors.New("unmarshalled block results object is not a list")
			}
			for _, resRaw := range resultsRaw.AsList() {
				blr, blrErr := p.processTxExpectedResult(resRaw)
				if blrErr != nil {
					return nil, blrErr
				}
				bl.Results = append(bl.Results, blr)
			}
		case "transactions":
			transactionsRaw, transactionsOk := kvp.Value.(*orderedjson.OJsonList)
			if !transactionsOk {
				return nil, errors.New("unmarshalled block transactions object is not a list")
			}
			for _, trRaw := range transactionsRaw.AsList() {
				var txType scenjsonmodel.TransactionType
				isCreate, err := p.txIsCreate(trRaw)
				if err != nil {
					return nil, err
				}
				if isCreate {
					txType = scenjsonmodel.ScDeploy
				} else {
					txType = scenjsonmodel.ScCall
				}
				tr, trErr := p.processTx(txType, trRaw)
				if trErr != nil {
					return nil, trErr
				}
				bl.Transactions = append(bl.Transactions, tr)
			}
		case "blockHeader":
			blh, blhErr := p.processBlockHeader(kvp.Value)
			if blhErr != nil {
				return nil, blhErr
			}
			bl.BlockHeader = blh
		default:
			return nil, fmt.Errorf("unknown block field: %s", kvp.Key)
		}
	}

	if len(bl.Results) != len(bl.Transactions) {
		return nil, errors.New("mismatched number of blocks and transactions")
	}

	return &bl, nil
}

// for old tests the only way to tell if it is a deploy or not is by checkong the "to" field, deploys have empty "to"
func (p *Parser) txIsCreate(txRaw orderedjson.OJsonObject) (bool, error) {
	txRawMap, isMap := txRaw.(*orderedjson.OJsonMap)
	if !isMap {
		return false, errors.New("unmarshalled block transaction is not a map")
	}
	for _, kvp := range txRawMap.OrderedKV {
		switch kvp.Key {
		case "to":
			toStr, err := p.parseString(kvp.Value)
			if err != nil {
				return false, fmt.Errorf("invalid block transaction to: %w", err)
			}
			return len(toStr) == 0, nil
		}
	}
	return false, nil
}

func (p *Parser) processBlockHeader(blhRaw interface{}) (*scenjsonmodel.BlockHeader, error) {
	blhMap, isMap := blhRaw.(*orderedjson.OJsonMap)
	if !isMap {
		return nil, errors.New("unmarshalled block header is not a map")
	}

	blh := scenjsonmodel.BlockHeader{}
	var err error

	for _, kvp := range blhMap.OrderedKV {
		switch kvp.Key {
		case "gasLimit":
			blh.GasLimit, err = p.processBigInt(kvp.Value, bigIntUnsignedBytes)
			if err != nil {
				return nil, fmt.Errorf("invalid block header gasLimit: %w", err)
			}
		case "number":
			blh.Number, err = p.processBigInt(kvp.Value, bigIntUnsignedBytes)
			if err != nil {
				return nil, fmt.Errorf("invalid block header number: %w", err)
			}
		case "difficulty":
			blh.Difficulty, err = p.processBigInt(kvp.Value, bigIntUnsignedBytes)
			if err != nil {
				return nil, fmt.Errorf("invalid block header difficulty: %w", err)
			}
		case "timestamp":
			blh.Timestamp, err = p.processUint64(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid block header timestamp: %w", err)
			}
		case "coinbase":
			blh.Beneficiary, err = p.processBigInt(kvp.Value, bigIntUnsignedBytes)
			if err != nil {
				return nil, fmt.Errorf("invalid block header coinbase: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown block header field: %s", kvp.Key)
		}
	}

	return &blh, nil
}
