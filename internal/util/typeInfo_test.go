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

func Test_checkUnderlyingType(t *testing.T) {
	const grpcServerStreamType = "google.golang.org/grpc.ServerStream"

	type args struct {
		n types.Type
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test with underlying type",
			args: args{
				n: types.NewNamed(
					types.NewTypeName(0, nil, "mainType", nil), // Main Type
					types.NewInterfaceType( // Underlying Type
						[]*types.Func{
							types.NewFunc(0, nil, "Recv", types.NewSignatureType(nil, nil, nil, types.NewTuple(
								types.NewVar(0, nil, "", types.NewPointer(types.NewNamed(
									types.NewTypeName(0, nil, "Message", nil),
									nil,
									nil,
								))),
							), nil, false)),
							types.NewFunc(0, nil, "SendAndClose", types.NewSignatureType(nil, nil, nil, types.NewTuple(
								types.NewVar(0, nil, "", types.NewPointer(types.NewNamed(
									types.NewTypeName(0, nil, "Message", nil),
									nil,
									nil,
								))),
							), nil, false)),
						},
						[]types.Type{
							types.NewNamed(
								types.NewTypeName(0, nil, grpcServerStreamType, nil),
								nil,
								nil,
							),
						},
					),
					nil,
				),
			},
			want: true,
		},
		{
			name: "Test with empty underlying type",
			args: args{
				n: types.NewNamed(
					types.NewTypeName(0, nil, "mainType", nil), // Main Type
					nil,
					nil,
				),
			},
			want: false,
		},
		{
			name: "Test with underlying type -> not grpcServerStreamType",
			args: args{
				n: types.NewNamed(
					types.NewTypeName(0, nil, "mainType", nil), // Main Type
					types.NewInterfaceType( // Underlying Type
						[]*types.Func{
							types.NewFunc(0, nil, "Recv", types.NewSignatureType(nil, nil, nil, types.NewTuple(
								types.NewVar(0, nil, "", types.NewPointer(types.NewNamed(
									types.NewTypeName(0, nil, "Message", nil),
									nil,
									nil,
								))),
							), nil, false)),
							types.NewFunc(0, nil, "SendAndClose", types.NewSignatureType(nil, nil, nil, types.NewTuple(
								types.NewVar(0, nil, "", types.NewPointer(types.NewNamed(
									types.NewTypeName(0, nil, "Message", nil),
									nil,
									nil,
								))),
							), nil, false)),
						},
						[]types.Type{
							types.NewNamed(
								types.NewTypeName(0, nil, "foo", nil),
								nil,
								nil,
							),
						},
					),
					nil,
				),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnderlyingType(tt.args.n.Underlying(), grpcServerStreamType); got != tt.want {
				t.Errorf("IsUnderlyingType() = %v, want %v", got, tt.want)
			}
		})
	}
}
