package mock

import (
	"bytes"
	"context"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/templates"
)

// DatabaseWriterStub -
type DatabaseWriterStub struct {
	DocExistsCalled            func(index string, id string) bool
	GetCalled                  func(index string, id string) (templates.Object, error)
	DoRequestCalled            func(req *esapi.IndexRequest) error
	DoBulkRequestCalled        func(buff *bytes.Buffer, index string) error
	DoBulkRemoveCalled         func(index string, hashes []string) error
	DoMultiGetCalled           func(query templates.Object, index string) (templates.Object, error)
	ConvertObjectToOrderCalled func(obj object) (*data.Order, error)
	ConvertObjectToDataCalled  func(obj object, data any) error
}

type object = map[string]interface{}

// DoRequest -
func (dwm *DatabaseWriterStub) DocExists(index string, id string) bool {
	if dwm.DocExistsCalled != nil {
		return dwm.DocExistsCalled(index, id)
	}
	return false
}

// Get wil do a get request to elaticsearch server
func (dwm *DatabaseWriterStub) Get(index string, id string) (templates.Object, error) {
	if dwm.GetCalled != nil {
		return dwm.GetCalled(index, id)
	}

	return templates.Object{}, nil
}

// DoRequest -
func (dwm *DatabaseWriterStub) DoRequest(req *esapi.IndexRequest) error {
	if dwm.DoRequestCalled != nil {
		return dwm.DoRequestCalled(req)
	}
	return nil
}

// DoBulkRequest -
func (dwm *DatabaseWriterStub) DoBulkRequest(_ context.Context, buff *bytes.Buffer, index string) error {
	if dwm.DoBulkRequestCalled != nil {
		return dwm.DoBulkRequestCalled(buff, index)
	}
	return nil
}

// DoMultiGet -
func (dwm *DatabaseWriterStub) DoMultiGet(query templates.Object, index string) (templates.Object, error) {
	if dwm.DoMultiGetCalled != nil {
		return dwm.DoMultiGetCalled(query, index)
	}

	return nil, nil
}

// DoBulkRemove -
func (dwm *DatabaseWriterStub) DoBulkRemove(index string, hashes []string) error {
	if dwm.DoBulkRemoveCalled != nil {
		return dwm.DoBulkRemoveCalled(index, hashes)
	}

	return nil
}

// ConvertObjectToOrder -
func (dwm *DatabaseWriterStub) ConvertObjectToOrder(obj object) (*data.Order, error) {
	if dwm.ConvertObjectToOrderCalled != nil {
		return dwm.ConvertObjectToOrderCalled(obj)
	}

	return nil, nil
}

func (dwm *DatabaseWriterStub) ConvertObjectToData(obj object, data any) error {
	if dwm.ConvertObjectToOrderCalled != nil {
		return dwm.ConvertObjectToDataCalled(obj, data)
	}

	return nil
}

// CheckAndCreateIndex -
func (dwm *DatabaseWriterStub) CheckAndCreateIndex(_ string) error {
	return nil
}

// CheckAndCreateAlias -
func (dwm *DatabaseWriterStub) CheckAndCreateAlias(_ string, _ string) error {
	return nil
}

// CheckAndCreateTemplate -
func (dwm *DatabaseWriterStub) CheckAndCreateTemplate(_ string, _ *bytes.Buffer) error {
	return nil
}

// CheckAndCreatePolicy -
func (dwm *DatabaseWriterStub) CheckAndCreatePolicy(_ string, _ *bytes.Buffer) error {
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (dwm *DatabaseWriterStub) IsInterfaceNil() bool {
	return dwm == nil
}
