package indexer

import (
	"testing"

	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
)

func Test_convertSFTMeta(t *testing.T) {
	cases := []struct {
		name  string
		input *kapps.MetaV2
		want  *data.Meta
	}{
		{
			"empty input",
			&kapps.MetaV2{},
			&data.Meta{
				Metadata: data.Metadata{
					ContentType: "text/plain",
				},
			},
		},
		{
			"non empty metadata hex data",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte{20, 20, 20, 255},
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "application/x-hex",
					Attributes:  "141414ff",
				},
			},
		},
		{
			"non empty metadata text/plain with {} in attributes",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte("{aaa}"),
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  "{aaa}",
				},
			},
		},

		{
			"non empty metadata text/plain",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte{75, 76, 86},
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  "KLV",
				},
			},
		},
		{
			"non empty metadata hex with {} in attributes",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: append([]byte("{}"), 255),
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "application/x-hex",
					Attributes:  "7b7dff",
				},
			},
		},
		{
			"non empty metadata text/plain",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte(`{"name":"KLV","hash":"282828","attributes":"KLV"}`),
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  `{"name":"KLV","hash":"282828","attributes":"KLV"}`,
				},
			},
		},
		{
			"non empty array metadata text/plain",
			&kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte(`[{"name":"KLV","hash":"282828","attributes":"KLV"}]`),
				},
			},
			&data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  `[{"name":"KLV","hash":"282828","attributes":"KLV"}]`,
				},
			},
		},
		{
			name: "metadata with special characters UTF-8",
			input: &kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte("KLV©"),
					Hash:       []byte{40, 40, 40},
					Attributes: []byte("áéíóú"),
				},
			},
			want: &data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV©",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  "áéíóú",
				},
			},
		},
		{
			name: "metadata with invalid JSON",
			input: &kapps.MetaV2{
				Metadata: &kapps.MetaV2Data{
					Name:       []byte{75, 76, 86},
					Hash:       []byte{40, 40, 40},
					Attributes: []byte(`{"name":"KLV`), // JSON incompleto
				},
			},
			want: &data.Meta{
				Metadata: data.Metadata{
					Name:        "KLV",
					Hash:        "282828",
					ContentType: "text/plain",
					Attributes:  `{"name":"KLV`,
				},
			},
		},
	}

	for _, tt := range cases {
		c := commonProcessor{}
		t.Run(tt.name, func(t *testing.T) {
			as := assert.New(t)
			got := c.convertSFTMeta(tt.input)
			as.Equal(tt.want, got)
		})
	}
}
