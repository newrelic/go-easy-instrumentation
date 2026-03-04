// grpc_internal_test.go contains tests for gRPC integration that require access
// to InstrumentationManager internal state for construction.
package parser_test

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/integrations/nrgrpc"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/parser"
	"github.com/newrelic/go-easy-instrumentation/parser/facts"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate/traceobject"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestGetTxnFromGrpcServer(t *testing.T) {
	grpcServerStreamType := types.NewNamed(
		types.NewTypeName(0, types.NewPackage("github.com/example/testapp", "testapp"), "TestApp_StreamServer", nil), // Main Type
		types.NewInterfaceType( // Underlying Type
			nil,
			[]types.Type{
				types.NewNamed(
					types.NewTypeName(0, types.NewPackage("google.golang.org/grpc", "grpc"), "ServerStream", nil),
					nil,
					nil,
				),
			},
		),
		nil,
	)

	contextParamName := &dst.Ident{Name: "ctx"}
	astContext := &ast.Ident{Name: "ctx"}
	serverStreamParamName := &dst.Ident{Name: "stream"}
	astServerStream := &ast.Ident{Name: "stream"}
	manager := &parser.InstrumentationManager{}
	manager.SetCurrentPackage("test")
	manager.SetPackageState("test", &parser.PackageState{
		Pkg: &decorator.Package{
			Package: &packages.Package{
				TypesInfo: &types.Info{
					Types: map[ast.Expr]types.TypeAndValue{
						astContext: {
							Type: types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage("context", "context"), "Context", nil), nil, nil),
						},
						astServerStream: {
							Type: grpcServerStreamType,
						},
					},
				},
			},
			Decorator: &decorator.Decorator{
				Map: decorator.Map{
					Ast: decorator.AstMap{
						Nodes: map[dst.Node]ast.Node{contextParamName: astContext, serverStreamParamName: astServerStream},
					},
				},
			},
		},
	})
	manager.SetFacts(facts.Keeper{
		"github.com/example/testapp.TestApp_StreamServer": facts.GrpcServerStream,
	})

	type args struct {
		manager *parser.InstrumentationManager
		params  []*dst.Field
	}
	tests := []struct {
		name string
		args
		want   *nrgrpc.GrpcServerTxnData
		expect bool
	}{
		{
			name: "grpc server stream",
			args: args{
				manager: manager,
				params: []*dst.Field{
					{
						Names: []*dst.Ident{serverStreamParamName},
					},
				},
			},
			want: &nrgrpc.GrpcServerTxnData{
				TxnAssignment: codegen.TxnFromContext("txn", nrgrpc.GrpcStreamContext(serverStreamParamName)),
				TraceObject:   traceobject.NewTransaction(),
			},
			expect: true,
		},
		{
			name: "grpc context",
			args: args{
				manager: manager,
				params: []*dst.Field{
					{
						Names: []*dst.Ident{contextParamName},
					},
				},
			},
			want: &nrgrpc.GrpcServerTxnData{
				TraceObject: traceobject.NewContext(contextParamName.Name),
			},
			expect: true,
		},
		{
			name: "empty params",
			args: args{
				manager: manager,
				params:  []*dst.Field{},
			},
			want:   nil,
			expect: false,
		},
		{
			name: "no context or stream",
			args: args{
				manager: manager,
				params: []*dst.Field{
					{
						Names: []*dst.Ident{
							{Name: "notContext"},
						},
					},
				},
			},
			want:   nil,
			expect: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nrgrpc.GetTxnFromGrpcServer(tt.args.manager, tt.args.params, "txn")
			if tt.expect {
				if !ok {
					t.Error("expected a transaction to be gotten from grpc server agrument")
				} else {
					assert.Equal(t, tt.want, got)
				}
			} else {
				if ok {
					t.Errorf("expected no transaction to be gotten from grpc server agrument, but got %+v", got)
				}
			}
		})
	}
}

func TestIsGrpcServerMethod(t *testing.T) {
	serverRecv := &dst.Ident{
		Name: "srv",
	}
	astServer := &ast.Ident{
		Name: "srv",
	}

	manager := &parser.InstrumentationManager{}
	manager.SetCurrentPackage("test")
	manager.SetPackageState("test", &parser.PackageState{
		Pkg: &decorator.Package{
			Package: &packages.Package{
				TypesInfo: &types.Info{
					Types: map[ast.Expr]types.TypeAndValue{
						astServer: {
							Type: types.NewPointer(types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage("github.com/example/testapp", "testapp"), "Server", types.NewInterfaceType(nil, nil)), nil, nil)),
						},
					},
				},
			},
			Decorator: &decorator.Decorator{
				Map: decorator.Map{
					Ast: decorator.AstMap{
						Nodes: map[dst.Node]ast.Node{serverRecv: astServer},
					},
				},
			},
		},
	})
	manager.SetFacts(facts.Keeper{
		"*github.com/example/testapp.Server": facts.GrpcServerType,
	})

	type args struct {
		manager *parser.InstrumentationManager
		decl    *dst.FuncDecl
	}
	tests := []struct {
		name string
		args
		want bool
	}{
		{
			name: "grpc server method",
			args: args{
				manager: manager,
				decl: &dst.FuncDecl{
					Recv: &dst.FieldList{
						List: []*dst.Field{
							{
								Names: []*dst.Ident{serverRecv},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "reciever is not grpc server",
			args: args{
				manager: manager,
				decl: &dst.FuncDecl{
					Recv: &dst.FieldList{
						List: []*dst.Field{
							{
								Names: []*dst.Ident{
									{
										Name: "notServer",
									},
								},
							},
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nrgrpc.IsGrpcServerMethod(tt.args.manager, tt.args.decl); got != tt.want {
				t.Errorf("isGrpcServerMethod() = %v, want %v", got, tt.want)
			}
		})
	}
}
