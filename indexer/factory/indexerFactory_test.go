package factory

import (
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/klever-io/klever-go/common"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/indexer/mock"
	"github.com/stretchr/testify/require"
)

func createMockIndexerFactoryArgs() *ArgsIndexerFactory {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	return &ArgsIndexerFactory{
		Enabled:                  true,
		IndexerCacheSize:         100,
		Url:                      ts.URL,
		UserName:                 "",
		Password:                 "",
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		EpochStartNotifier:       &mock.EpochStartNotifierStub{},
		NodesCoordinator:         &mock.NodesCoordinatorMock{},
		AddressPubkeyConverter:   &mock.PubkeyConverterMock{},
		ValidatorPubkeyConverter: &mock.PubkeyConverterMock{},
		EnabledIndexes:           []string{"transactions", "blocks"}, //!
		AccountsDB:               &mock.AccountsStub{},
		KappsDB:                  &mock.KappsDBMock{},
		KAppController:           &mock.KappsControllerMock{},
		IsInImportDBMode:         false,
	}
}

func TestNewIndexerFactory(t *testing.T) {
	tests := []struct {
		name     string
		argsFunc func() *ArgsIndexerFactory
		exError  error
	}{
		{
			name: "InvalidCacheSize",
			argsFunc: func() *ArgsIndexerFactory {
				args := createMockIndexerFactoryArgs()
				args.IndexerCacheSize = -1
				return args
			},
			exError: indexer.ErrNegativeCacheSize,
		},
		{
			name: "NilAddressPubkeyConverter",
			argsFunc: func() *ArgsIndexerFactory {
				args := createMockIndexerFactoryArgs()
				args.AddressPubkeyConverter = nil
				return args
			},
			exError: indexer.ErrNilPubkeyConverter,
		},
		{
			name: "NilValidatorPubkeyConverter",
			argsFunc: func() *ArgsIndexerFactory {
				args := createMockIndexerFactoryArgs()
				args.ValidatorPubkeyConverter = nil
				return args
			},
			exError: indexer.ErrNilPubkeyConverter,
		},
		{
			name: "NilMarshalizer",
			argsFunc: func() *ArgsIndexerFactory {
				args := createMockIndexerFactoryArgs()
				args.Marshalizer = nil
				return args
			},
			exError: common.ErrNilMarshalizer,
		},
		{
			name: "NilHasher",
			argsFunc: func() *ArgsIndexerFactory {
				args := createMockIndexerFactoryArgs()
				args.Hasher = nil
				return args
			},
			exError: common.ErrNilHasher,
		},
		{
			name: "NilNodesCoordinator",
			argsFunc: func() *ArgsIndexerFactory {
				args := createMockIndexerFactoryArgs()
				args.NodesCoordinator = nil
				return args
			},
			exError: common.ErrNilNodesCoordinator,
		},
		{
			name: "NilEpochStartNotifier",
			argsFunc: func() *ArgsIndexerFactory {
				args := createMockIndexerFactoryArgs()
				args.EpochStartNotifier = nil
				return args
			},
			exError: common.ErrNilEpochStartNotifier,
		},
		{
			name: "NilAccountsDB",
			argsFunc: func() *ArgsIndexerFactory {
				args := createMockIndexerFactoryArgs()
				args.AccountsDB = nil
				return args
			},
			exError: indexer.ErrNilAccountsDB,
		},
		{
			name: "EmptyUrl",
			argsFunc: func() *ArgsIndexerFactory {
				args := createMockIndexerFactoryArgs()
				args.Url = ""
				return args
			},
			exError: common.ErrNilUrl,
		},
		{
			name: "All arguments ok",
			argsFunc: func() *ArgsIndexerFactory {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-Elastic-Product", "Elasticsearch")
				}))
				args := createMockIndexerFactoryArgs()
				args.Url = ts.URL
				return args
			},
			exError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIndexer(tt.argsFunc())
			if !errors.Is(err, tt.exError) {
				log.Println(err)
				log.Println(tt.exError)
			}

			require.True(t, errors.Is(err, tt.exError))
		})
	}
}

func TestIndexerFactoryCreate_NilIndexer(t *testing.T) {
	t.Parallel()

	args := createMockIndexerFactoryArgs()
	args.Enabled = false
	nilIndexer, err := NewIndexer(args)
	require.NoError(t, err)

	_, ok := nilIndexer.(*indexer.NilIndexer)
	require.True(t, ok)
}

func TestIndexerFactoryCreate_ElasticIndexer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
	}))
	args := createMockIndexerFactoryArgs()
	args.UseKibana = false
	args.Url = ts.URL

	_, err := NewIndexer(args)
	require.Nil(t, err)
}
