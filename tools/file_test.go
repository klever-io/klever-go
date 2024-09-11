package tools_test

import (
	"os"
	"testing"

	"github.com/klever-io/klever-go/tools"
	"github.com/stretchr/testify/assert"
)

func Test_SanitizePath(t *testing.T) {

	currentDir, _ := os.Getwd()
	currentDir += "/"

	tests := []struct {
		name string
		path string
		want string
		err  error
	}{
		{
			name: "empty path",
			path: "",
			want: "",
			err:  tools.ErrInvalidFileName,
		},
		{
			name: "path with spaces",
			path: "test test",
			want: "",
			err:  tools.ErrInvalidFileName,
		},
		{
			name: "path with special characters",
			path: "test&test",
			want: "",
			err:  tools.ErrInvalidFileName,
		},
		{
			name: "valid file name no extension",
			path: "test",
			want: currentDir + "test",
			err:  nil,
		},
		{
			name: "valid file name with extension",
			path: "test.png",
			want: currentDir + "test.png",
			err:  nil,
		},
		{
			name: "valid path name no extension",
			path: "/home/test",
			want: "/home/test",
			err:  nil,
		},
		{
			name: "valid path name with extension",
			path: "/home/test.png",
			want: "/home/test.png",
			err:  nil,
		},
		{
			name: "valid file name unknown extension",
			path: "test.ab",
			want: currentDir + "test.ab",
			err:  nil,
		},
		{
			name: "valid file name invalid extension",
			path: "test.a&b",
			want: "",
			err:  tools.ErrInvalidFileName,
		},
		{
			name: "valid file name invalid extension",
			path: "test.a/b",
			want: "",
			err:  tools.ErrInvalidFileName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tools.SanitizePath(tt.path)
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.want, got, "SanitizePath() = %v, want %v", got, tt.want)
		})
	}
}
