package pathmanager_test

import (
	"testing"

	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/pathmanager"
	"github.com/stretchr/testify/assert"
)

func TestNewPathManager_EmptyPruningPathTemplateShouldErr(t *testing.T) {
	t.Parallel()

	pm, err := pathmanager.NewPathManager("", "[I]", "db")
	assert.Nil(t, pm)
	assert.Equal(t, storage.ErrEmptyPruningPathTemplate, err)
}

func TestNewPathManager_EmptyStaticPathTemplateShouldErr(t *testing.T) {
	t.Parallel()

	pm, err := pathmanager.NewPathManager("epoch_[E]/[I]", "", "db")
	assert.Nil(t, pm)
	assert.Equal(t, storage.ErrEmptyStaticPathTemplate, err)
}

func TestNewPathManager_InvalidPruningPathTemplate_NoEpochPlaceholder_ShouldErr(t *testing.T) {
	t.Parallel()

	pm, err := pathmanager.NewPathManager("epoch/[I]", "[I]", "db")
	assert.Nil(t, pm)
	assert.Equal(t, storage.ErrInvalidPruningPathTemplate, err)
}

func TestNewPathManager_InvalidPathPruningTemplate_NoIdentifierPlaceholder_ShouldErr(t *testing.T) {
	t.Parallel()

	pm, err := pathmanager.NewPathManager("epoch_[E]", "[I]", "db")
	assert.Nil(t, pm)
	assert.Equal(t, storage.ErrInvalidPruningPathTemplate, err)
}

func TestNewPathManager_InvalidStaticPathTemplate_NoIdentifierPlaceholder_ShouldErr(t *testing.T) {
	t.Parallel()

	pm, err := pathmanager.NewPathManager("epoch_[E]/[I]", "./", "db")
	assert.Nil(t, pm)
	assert.Equal(t, storage.ErrInvalidStaticPathTemplate, err)
}

func TestNewPathManager_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	pm, err := pathmanager.NewPathManager("epoch_[E]/[I]", "[I]", "db")
	assert.NotNil(t, pm)
	assert.Nil(t, err)
}

func TestPathManager_PathForEpoch(t *testing.T) {
	t.Parallel()

	type args struct {
		epoch      uint32
		identifier string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			args: args{epoch: 2, identifier: "table"},
			want: "Epoch_2/table",
		},
		{
			args: args{epoch: 2654, identifier: "table23"},
			want: "Epoch_2654/table23",
		},
		{
			args: args{epoch: 0, identifier: ""},
			want: "Epoch_0/",
		},
		{
			args: args{epoch: 25839, identifier: "table1"},
			want: "Epoch_25839/table1",
		},
	}
	pruningPathTemplate := "Epoch_[E]/[I]"
	staticPathTemplate := "[I]"
	pm, _ := pathmanager.NewPathManager(pruningPathTemplate, staticPathTemplate, "db")
	for _, tt := range tests {
		ttCopy := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := pm.PathForEpoch(ttCopy.args.epoch, ttCopy.args.identifier); got != ttCopy.want {
				t.Errorf("PathForEpoch() = %v, want %v", got, ttCopy.want)
			}
		})
	}
}

func TestPathManager_PathForStatic(t *testing.T) {
	t.Parallel()

	type args struct {
		identifier string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			args: args{identifier: "table"},
			want: "Static/table",
		},
		{
			args: args{identifier: "table23"},
			want: "Static/table23",
		},
		{
			args: args{identifier: ""},
			want: "Static/",
		},
	}
	pruningPathTemplate := "Epoch_[E]/[I]"
	staticPathTemplate := "Static/[I]"
	pm, _ := pathmanager.NewPathManager(pruningPathTemplate, staticPathTemplate, "db")
	for _, tt := range tests {
		ttCopy := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := pm.PathForStatic(ttCopy.args.identifier); got != ttCopy.want {
				t.Errorf("PathForEpoch() = %v, want %v", got, ttCopy.want)
			}
		})
	}
}
