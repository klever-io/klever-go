package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/templates"
)

const errPolicyAlreadyExists = "document already exists"
const numOfErrorsToExtractBulkResponse = 5

type responseErrorHandler func(res *esapi.Response) error

type kibanaResponse struct {
	Error  interface{} `json:"error,omitempty"`
	Status int         `json:"status"`
}

type elasticClient struct {
	elasticBaseUrl string
	es             *elasticsearch.Client
}

// BulkRequestResponse defines the structure of a bulk request response
type BulkRequestResponse struct {
	Errors bool `json:"errors"`
	Items  []struct {
		ItemIndex  *Item `json:"index"`
		ItemUpdate *Item `json:"update"`
	} `json:"items"`
}

// Item defines the structure of a item from a bulk response
type Item struct {
	Index  string `json:"_index"`
	ID     string `json:"_id"`
	Status int    `json:"status"`
	Result string `json:"result"`
	Error  struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
		Cause  struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"caused_by"`
	} `json:"error"`
}

// NewElasticClient will create a new instance of elasticClient
func NewElasticClient(cfg elasticsearch.Config) (*elasticClient, error) {
	if len(cfg.Addresses) == 0 {
		return nil, ErrNoElasticUrlProvided
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	ec := &elasticClient{
		es:             es,
		elasticBaseUrl: cfg.Addresses[0],
	}

	return ec, nil
}

// CheckAndCreateTemplate creates an index template if it does not already exist
func (ec *elasticClient) CheckAndCreateTemplate(templateName string, template *bytes.Buffer) error {
	if ec.templateExists(templateName) {
		return nil
	}

	return ec.createIndexTemplate(templateName, template)
}

// CheckAndCreatePolicy creates a new index policy if it does not already exist
func (ec *elasticClient) CheckAndCreatePolicy(policyName string, policy *bytes.Buffer) error {
	if ec.PolicyExists(policyName) {
		return nil
	}

	return ec.createPolicy(policyName, policy)
}

// CheckAndCreateIndex creates a new index if it does not already exist
func (ec *elasticClient) CheckAndCreateIndex(indexName string) error {
	if ec.indexExists(indexName) {
		return nil
	}

	return ec.createIndex(indexName)
}

// CheckAndCreateAlias creates a new alias if it does not already exist
func (ec *elasticClient) CheckAndCreateAlias(alias string, indexName string) error {
	if ec.aliasExists(alias) {
		return nil
	}

	return ec.createAlias(alias, indexName)
}

// CheckFieldMapping compares the properties with the live mapping of index and returns the
// names of the properties the index does not map yet. A property the index maps with a
// different type is an error: Elasticsearch cannot change a field's type in place, so the
// only way out is a reindex, and a start-up that carried on would write into a mapping the
// filter on that field is not written for.
func (ec *elasticClient) CheckFieldMapping(index string, properties templates.Object) ([]string, error) {
	wanted, err := mappingTypes(properties)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(wanted))
	for name := range wanted {
		names = append(names, name)
	}
	sort.Strings(names)

	res, err := ec.es.Indices.GetFieldMapping(names, ec.es.Indices.GetFieldMapping.WithIndex(index))
	if err != nil {
		return nil, err
	}

	defer closeResponseBody(res, "elasticClient.CheckFieldMapping")

	if res.IsError() {
		return nil, fmt.Errorf("%w: index %s: %s", ErrCouldNotUpdateMapping, index, res.String())
	}

	var live liveFieldMappings
	// An empty body means nothing is mapped, which the PUT that follows is allowed to fix;
	// anything else that fails to decode is a cluster answer this code does not understand.
	if err := json.NewDecoder(res.Body).Decode(&live); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: index %s: %w", ErrCouldNotUpdateMapping, index, err)
	}

	mapped, err := mappedFields(live, wanted)
	if err != nil {
		return nil, err
	}

	missing := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := mapped[name]; !ok {
			missing = append(missing, name)
		}
	}

	return missing, nil
}

// liveFieldMappings is the shape of a field mapping response: concrete index, then field,
// then the leaf carrying the type.
type liveFieldMappings map[string]struct {
	Mappings map[string]struct {
		Mapping map[string]map[string]interface{} `json:"mapping"`
	} `json:"mappings"`
}

// mappedFields returns the wanted properties the live mapping already carries, and refuses
// the first one it carries with another type.
func mappedFields(live liveFieldMappings, wanted map[string]string) (map[string]struct{}, error) {
	mapped := make(map[string]struct{})
	for concreteIndex, indexMappings := range live {
		for name, field := range indexMappings.Mappings {
			if err := checkFieldType(concreteIndex, name, field.Mapping, wanted[name]); err != nil {
				return nil, err
			}
			mapped[name] = struct{}{}
		}
	}

	return mapped, nil
}

func checkFieldType(index string, name string, mapping map[string]map[string]interface{}, want string) error {
	for _, leaf := range mapping {
		if got, _ := leaf["type"].(string); got != want {
			return fmt.Errorf("%w: index %s maps %s as %q, this build needs %q; the index has to be reindexed",
				ErrCouldNotUpdateMapping, index, name, got, want)
		}
	}

	return nil
}

// CheckAndUpdateMapping adds the properties an index does not map yet. Templates only apply
// when an index is created, so a property added to a template later never reaches an index
// that already exists; the first document carrying the field would then type it
// dynamically, as text with a keyword subfield, instead of the type the template names.
// The live mapping is read first, so a node whose index already carries the properties
// sends no write at all at start-up and needs no manage privilege for it; a property mapped
// with another type surfaces as an error rather than being closed and forgotten, because a
// mapping that silently failed to apply is exactly the drift this exists to prevent.
func (ec *elasticClient) CheckAndUpdateMapping(index string, properties templates.Object) error {
	missing, err := ec.CheckFieldMapping(index, properties)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}

	all, ok := properties["properties"].(templates.Object)
	if !ok {
		return fmt.Errorf("%w: properties must be an object under \"properties\"", ErrCouldNotUpdateMapping)
	}
	subset := templates.Object{}
	for _, name := range missing {
		subset[name] = all[name]
	}
	body := templates.Object{"properties": subset}

	res, err := ec.es.Indices.PutMapping([]string{index}, body.ToBuffer())
	if err != nil {
		return err
	}

	defer closeResponseBody(res, "elasticClient.CheckAndUpdateMapping")

	if res.IsError() {
		return fmt.Errorf("%w: index %s: %s", ErrCouldNotUpdateMapping, index, res.String())
	}

	return nil
}

// PutTemplate writes an index template whether or not one exists under that name, so a
// template that gained a property reaches clusters that already carry the old one. A
// template only shapes indices created after it, which is why the live index is handled
// by CheckAndUpdateMapping separately.
func (ec *elasticClient) PutTemplate(templateName string, template *bytes.Buffer) error {
	res, err := ec.es.Indices.PutTemplate(templateName, template)
	if err != nil {
		return err
	}

	defer closeResponseBody(res, "elasticClient.PutTemplate")

	if res.IsError() {
		return fmt.Errorf("%w: template %s: %s", ErrCouldNotCreateTemplate, templateName, res.String())
	}

	return nil
}

// mappingTypes flattens {"properties": {name: {"type": t}}} into name -> t.
func mappingTypes(properties templates.Object) (map[string]string, error) {
	all, ok := properties["properties"].(templates.Object)
	if !ok {
		return nil, fmt.Errorf("%w: properties must be an object under \"properties\"", ErrCouldNotUpdateMapping)
	}

	types := make(map[string]string, len(all))
	for name, raw := range all {
		field, ok := raw.(templates.Object)
		if !ok {
			return nil, fmt.Errorf("%w: property %s must be an object", ErrCouldNotUpdateMapping, name)
		}
		fieldType, ok := field["type"].(string)
		if !ok {
			return nil, fmt.Errorf("%w: property %s must name a type", ErrCouldNotUpdateMapping, name)
		}
		types[name] = fieldType
	}

	return types, nil
}

// DoRequest will do a request to elastic server
func (ec *elasticClient) DoRequest(req *esapi.IndexRequest) error {
	res, err := req.Do(context.Background(), ec.es)
	if err != nil {
		return err
	}

	defer closeResponseBody(res, "elasticClient.DoRequest")

	return nil
}

// DoBulkRequest will do a bulk of request to elastic server
func (ec *elasticClient) DoBulkRequest(ctx context.Context, buff *bytes.Buffer, index string) error {
	reader := bytes.NewReader(buff.Bytes())

	options := make([]func(*esapi.BulkRequest), 0)
	if index != "" {
		options = append(options, ec.es.Bulk.WithIndex(index))
	}

	options = append(options, ec.es.Bulk.WithContext(ctx))

	res, err := ec.es.Bulk(
		reader,
		options...,
	)
	if err != nil {
		log.Warn("elasticClient.DoBulkRequest",
			"indexer do bulk request no response", err.Error())
		return err
	}

	return elasticBulkRequestResponseHandler(res)
}

// DoMultiGet wil do a multi get request to elaticsearch server
func (ec *elasticClient) DoMultiGet(obj templates.Object, index string) (templates.Object, error) {
	body, err := encode(obj)
	if err != nil {
		return nil, err
	}

	res, err := ec.es.Mget(
		&body,
		ec.es.Mget.WithIndex(index),
	)
	if err != nil {
		log.Warn("elasticClient.DoMultiGet",
			"cannot do multi get no response", err.Error())
		return nil, err
	}

	var decodedBody templates.Object
	err = parseResponse(res, &decodedBody, elasticDefaultErrorResponseHandler)
	if err != nil {
		log.Warn("elasticClient.DoMultiGet",
			ErrorParseResponse, err.Error())
		return nil, err
	}

	return decodedBody, nil
}

// Get wil do a get request to elaticsearch server
func (ec *elasticClient) Get(index string, id string) (templates.Object, error) {
	res, err := ec.es.Get(index, id)
	if err != nil {
		return nil, fmt.Errorf("cannot get data from database: %w", err)
	}

	defer func() {
		_ = res.Body.Close()
	}()
	if res.IsError() {
		return nil, fmt.Errorf("cannot get data from database: %v", res)
	}

	var decodedBody templates.Object
	err = parseResponse(res, &decodedBody, elasticDefaultErrorResponseHandler)
	if err != nil {
		log.Warn("elasticClient.DoMultiGet",
			ErrorParseResponse, err.Error())
		return nil, err
	}

	return decodedBody, nil
}

// DoBulkRemove will do a bulk remove to elasticsearch server
func (ec *elasticClient) DoBulkRemove(index string, hashes []string) error {
	obj := prepareHashesForBulkRemove(hashes)
	body, err := encode(obj)
	if err != nil {
		return err
	}

	res, err := ec.es.DeleteByQuery(
		[]string{index},
		&body,
		ec.es.DeleteByQuery.WithIgnoreUnavailable(true),
	)

	if err != nil {
		log.Warn("elasticClient.DoBulkRemove",
			"cannot do bulk remove", err.Error())
		return err
	}

	defer closeResponseBody(res, "elasticClient.DoBulkRemove")

	return nil
}

// DoBulkRemoveByTimestamp will do a bulk remove by timestamp to elasticsearch server
func (ec *elasticClient) DoBulkRemoveByTimestamp(index string, timestamp time.Duration) error {
	obj := prepareTimestampForBulkRemove(timestamp)
	body, err := encode(obj)
	if err != nil {
		return err
	}

	res, err := ec.es.DeleteByQuery(
		[]string{index},
		&body,
		ec.es.DeleteByQuery.WithIgnoreUnavailable(true),
	)

	if err != nil {
		log.Warn("elasticClient.DoBulkRemoveByTimestamp",
			"cannot do bulk remove by timestamp", err.Error())
		return err
	}

	defer closeResponseBody(res, "elasticClient.DoBulkRemoveByTimestamp")

	return nil
}

// DoSearch performs a search query on elasticsearch
func (ec *elasticClient) DoSearch(index string, body *bytes.Buffer) (templates.Object, error) {
	res, err := ec.es.Search(
		ec.es.Search.WithIndex(index),
		ec.es.Search.WithBody(body),
	)
	if err != nil {
		return nil, err
	}

	defer closeResponseBody(res, "elasticClient.DoSearch")

	var decodedBody templates.Object
	err = parseResponse(res, &decodedBody, elasticDefaultErrorResponseHandler)
	if err != nil {
		return nil, err
	}

	return decodedBody, nil
}

// DoUpdate performs an update operation on a document
func (ec *elasticClient) DoUpdate(index string, id string, body *bytes.Buffer) error {
	res, err := ec.es.Update(index, id, body)
	if err != nil {
		log.Warn("elasticClient.DoUpdate", "error", err.Error())
		return err
	}

	defer closeResponseBody(res, "elasticClient.DoUpdate")

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("error updating document: %s - %s", res.Status(), string(bodyBytes))
	}

	return nil
}

// TemplateExists checks weather a template is already created
func (ec *elasticClient) templateExists(index string) bool {
	res, err := ec.es.Indices.ExistsTemplate([]string{index})
	return exists(res, err)
}

// IndexExists checks if a given index already exists
func (ec *elasticClient) indexExists(index string) bool {
	res, err := ec.es.Indices.Exists([]string{index})
	return exists(res, err)
}

// DocExists checks if a given document already exists
func (ec *elasticClient) DocExists(index string, id string) bool {
	res, err := ec.es.Exists(index, id)
	return exists(res, err)
}

// PolicyExists checks if a policy was already created
func (ec *elasticClient) PolicyExists(policy string) bool {
	policyRoute := fmt.Sprintf(
		"%s/%s/ism/policies/%s",
		ec.elasticBaseUrl,
		kibanaPluginPath,
		policy,
	)

	req, err := newRequest(http.MethodGet, policyRoute, nil)
	if err != nil {
		log.Warn("elasticClient.PolicyExists",
			"could not create request objectsMap", err.Error())
		return false
	}

	res, err := ec.es.Transport.Perform(req)
	if err != nil {
		log.Warn("elasticClient.PolicyExists",
			"error performing request", err.Error())
		return false
	}
	defer func() { _ = res.Body.Close() }()

	response := &esapi.Response{
		StatusCode: res.StatusCode,
		Body:       res.Body,
		Header:     res.Header,
	}

	existsRes := &kibanaResponse{}
	err = parseResponse(response, existsRes, kibanaResponseErrorHandler)
	if err != nil {
		log.Warn("elasticClient.PolicyExists",
			"error returned by kibana api", err.Error())
		return false
	}

	return existsRes.Status == http.StatusConflict
}

// AliasExists checks if an index alias already exists
func (ec *elasticClient) aliasExists(alias string) bool {
	aliasRoute := fmt.Sprintf(
		"/_alias/%s",
		alias,
	)

	req, err := newRequest(http.MethodHead, aliasRoute, nil)
	if err != nil {
		log.Warn("elasticClient.AliasExists",
			"could not create request objectsMap", err.Error())
		return false
	}

	res, err := ec.es.Transport.Perform(req)
	if err != nil {
		log.Warn("elasticClient.AliasExists",
			"error performing request", err.Error())
		return false
	}
	defer func() { _ = res.Body.Close() }()

	response := &esapi.Response{
		StatusCode: res.StatusCode,
		Body:       res.Body,
		Header:     res.Header,
	}

	return exists(response, nil)
}

// CreateIndex creates an elasticsearch index
func (ec *elasticClient) createIndex(index string) error {
	res, err := ec.es.Indices.Create(index)
	if err != nil {
		return err
	}

	defer closeResponseBody(res, "elasticClient.createIndex")

	return nil
}

// CreatePolicy creates a new policy for elastic indexes. Policies define rollover parameters
func (ec *elasticClient) createPolicy(policyName string, policy *bytes.Buffer) error {
	policyRoute := fmt.Sprintf(
		"%s/_opendistro/_ism/policies/%s",
		ec.elasticBaseUrl,
		policyName,
	)

	req, err := newRequest(http.MethodPut, policyRoute, policy)
	if err != nil {
		return err
	}

	req.Header[headerContentType] = headerContentTypeJSON
	req.Header[headerXSRF] = []string{"false"}
	res, err := ec.es.Transport.Perform(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	response := &esapi.Response{
		StatusCode: res.StatusCode,
		Body:       res.Body,
		Header:     res.Header,
	}

	existsRes := &kibanaResponse{}
	err = parseResponse(response, existsRes, kibanaResponseErrorHandler)
	if err != nil {
		return err
	}

	errStr := fmt.Sprintf("%v", existsRes.Error)
	if existsRes.Status == http.StatusConflict && !strings.Contains(errStr, errPolicyAlreadyExists) {
		return ErrCouldNotCreatePolicy
	}

	return nil
}

// CreateIndexTemplate creates an elasticsearch index template
func (ec *elasticClient) createIndexTemplate(templateName string, template io.Reader) error {
	res, err := ec.es.Indices.PutTemplate(templateName, template)
	if err != nil {
		return err
	}

	defer closeResponseBody(res, "elasticClient.createIndexTemplate")

	return nil
}

// CreateAlias creates an index alias
func (ec *elasticClient) createAlias(alias string, index string) error {
	res, err := ec.es.Indices.PutAlias([]string{index}, alias)
	if err != nil {
		return err
	}

	defer closeResponseBody(res, "elasticClient.createAlias")

	return nil
}

type object = map[string]interface{}

func (ec *elasticClient) ConvertObjectToOrder(obj object) (*data.Order, error) {
	marshalizedOrder, _ := json.Marshal(obj["_source"])
	var order *data.Order
	err := json.Unmarshal(marshalizedOrder, &order)
	if err != nil {
		return nil, errors.New("cannot unmarshal order")
	}

	return order, nil
}

func (ec *elasticClient) ConvertObjectToData(obj object, data any) error {
	marshalizedOrder, _ := json.Marshal(obj["_source"])

	err := json.Unmarshal(marshalizedOrder, &data)
	if err != nil {
		return errors.New("cannot unmarshal order")
	}

	return nil
}

// closeResponseBody closes the response body and logs any errors
func closeResponseBody(res *esapi.Response, callerName string) {
	if res != nil && res.Body != nil {
		err := res.Body.Close()
		if err != nil {
			log.Warn(callerName, ErrorCouldNotCloseBody, err.Error())
		}
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (ec *elasticClient) IsInterfaceNil() bool {
	return ec == nil
}
