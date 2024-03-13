package scenjsonparse

import (
	"errors"
	"fmt"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func (p *Parser) processTxKDA(txEsdtRaw orderedjson.OJsonObject) ([]*scenjsonmodel.KDATxData, error) {
	allEsdtData := make([]*scenjsonmodel.KDATxData, 0)

	switch txEsdt := txEsdtRaw.(type) {
	case *orderedjson.OJsonMap:
		if !p.AllowEsdtTxLegacySyntax {
			return nil, fmt.Errorf("wrong KDA Multi-Transfer format, list expected")
		}
		entry, err := p.parseSingleTxEsdtEntry(txEsdt)
		if err != nil {
			return nil, err
		}

		allEsdtData = append(allEsdtData, entry)
	case *orderedjson.OJsonList:
		for _, txEsdtListItem := range txEsdt.AsList() {
			txEsdtMap, isMap := txEsdtListItem.(*orderedjson.OJsonMap)
			if !isMap {
				return nil, fmt.Errorf("wrong KDA Multi-Transfer format")
			}

			entry, err := p.parseSingleTxEsdtEntry(txEsdtMap)
			if err != nil {
				return nil, err
			}

			allEsdtData = append(allEsdtData, entry)
		}
	default:
		return nil, fmt.Errorf("wrong KDA transfer format, expected list")
	}

	return allEsdtData, nil
}

func (p *Parser) parseSingleTxEsdtEntry(kdaTxEntry *orderedjson.OJsonMap) (*scenjsonmodel.KDATxData, error) {
	kdaData := scenjsonmodel.KDATxData{}
	var err error

	for _, kvp := range kdaTxEntry.OrderedKV {
		switch kvp.Key {
		case "tokenIdentifier":
			kdaData.TokenIdentifier, err = p.processStringAsByteArray(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid KDA token name: %w", err)
			}
		case "nonce":
			kdaData.Nonce, err = p.processUint64(kvp.Value)
			if err != nil {
				return nil, errors.New("invalid account nonce")
			}
		case "value":
			kdaData.Value, err = p.processBigInt(kvp.Value, bigIntUnsignedBytes)
			if err != nil {
				return nil, fmt.Errorf("invalid KDA balance: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown transaction KDA data field: %s", kvp.Key)
		}
	}

	return &kdaData, nil
}
