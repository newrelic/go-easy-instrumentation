package util

import (
	"go/types"
	"testing"
)

func Test_isNamedError(t *testing.T) {
	type args struct {
		n types.Type
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test is named error",
			args: args{
				n: types.NewNamed(types.NewTypeName(0, nil, "error", nil), nil, nil),
			},
			want: true,
		},
		{
			name: "Test is not error",
			args: args{
				n: types.NewNamed(types.NewTypeName(0, nil, "foo", nil), nil, nil),
			},
			want: false,
		},
		{
			name: "Nil Named Error",
			args: args{
				n: nil,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsError(tt.args.n); got != tt.want {
				t.Errorf("isNamedError() = %v, want %v", got, tt.want)
			}
		})
	}
}
