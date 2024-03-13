package scenjsonparse

import (
	"errors"
	"fmt"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func (p *Parser) processLogList(logsRaw orderedjson.OJsonObject) (scenjsonmodel.LogList, error) {
	if IsStar(logsRaw) {
		return scenjsonmodel.LogList{
			IsUnspecified: false,
			IsStar:        true,
		}, nil
	}

	logList, isList := logsRaw.(*orderedjson.OJsonList)
	if !isList {
		return scenjsonmodel.LogList{}, errors.New("unmarshalled logs list is not a list")
	}
	result := scenjsonmodel.LogList{
		IsUnspecified:    false,
		IsStar:           false,
		MoreAllowedAtEnd: false,
		List:             nil,
	}
	var err error
	for _, logRaw := range logList.AsList() {
		switch logItem := logRaw.(type) {
		case *orderedjson.OJsonString:
			if logItem.Value == "+" {
				result.MoreAllowedAtEnd = true
			} else {
				return scenjsonmodel.LogList{}, errors.New("unmarshalled log entry is an invalid string")
			}
		case *orderedjson.OJsonMap:
			if result.MoreAllowedAtEnd {
				return scenjsonmodel.LogList{}, errors.New("log entry ")
			}

			logEntry := scenjsonmodel.LogEntry{}
			for _, kvp := range logItem.OrderedKV {
				switch kvp.Key {
				case "address":
					logEntry.Address, err = p.parseCheckBytes(kvp.Value)
					if err != nil {
						return scenjsonmodel.LogList{}, fmt.Errorf("invalid log address: %w", err)
					}
				case "endpoint":
					logEntry.Endpoint, err = p.parseCheckBytes(kvp.Value)
					if err != nil {
						return scenjsonmodel.LogList{}, fmt.Errorf("invalid log identifier: %w", err)
					}
				case "topics":
					logEntry.Topics, err = p.parseCheckValueList(kvp.Value)
					if err != nil {
						return scenjsonmodel.LogList{}, fmt.Errorf("invalid log entry topics: %w", err)
					}
				case "data":
					logEntry.Data, err = p.parseCheckValueList(kvp.Value)
					if err != nil {
						return scenjsonmodel.LogList{}, fmt.Errorf("invalid log data: %w", err)
					}
				default:
					return scenjsonmodel.LogList{}, fmt.Errorf("unknown log field: %s", kvp.Key)
				}
			}
			result.List = append(result.List, &logEntry)
		default:
			return scenjsonmodel.LogList{}, errors.New("log entry should be either string or object")
		}
	}

	return result, nil
}
