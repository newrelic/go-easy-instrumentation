package nrgrpc_test

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"testing"

	"github.com/newrelic/go-easy-instrumentation/integrations/nrgrpc"
	"github.com/newrelic/go-easy-instrumentation/parser"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/parser/facts"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestInstrumentGrpcDial(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "detect and trace grpc dial",
			code: `package main

import "google.golang.org/grpc"

func main() {
	conn, err := grpc.Dial(
		"localhost:8080",
		grpc.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
}
`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/nrgrpc"
	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial(
		"localhost:8080",
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(nrgrpc.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(nrgrpc.StreamClientInterceptor),
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nrgrpc.InstrumentGrpcDial)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestInstrumentGrpcServer(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "detect and trace grpc dial",
			code: `package main

import "google.golang.org/grpc"

func main() {
	lis, err := net.Listen("tcp", "localhost:8080")
	grpcServer := grpc.NewServer()
	grpcServer.Serve(lis)
}
`,
			expect: `package main

import (
	"github.com/newrelic/go-agent/v3/integrations/nrgrpc"
	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", "localhost:8080")
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(nrgrpc.UnaryServerInterceptor(app)),
		grpc.StreamInterceptor(nrgrpc.StreamServerInterceptor(app)),
	)
	grpcServer.Serve(lis)
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatefulTracingFunction(t, tt.code, nrgrpc.InstrumentGrpcServer, false)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestGrpcDialCall(t *testing.T) {
	type args struct {
		node dst.Node
	}
	tests := []struct {
		name  string
		args  args
		want  *dst.CallExpr
		want1 bool
	}{
		{
			name: "grpc Dial Assign Statement",
			args: args{
				node: &dst.AssignStmt{
					Rhs: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "Dial",
								Path: nrgrpc.GrpcImportPath,
							},
							Args: []dst.Expr{
								&dst.BasicLit{
									Value: `"localhost:8080"`,
									Kind:  token.STRING,
								},
							},
						},
					},
					Lhs: []dst.Expr{
						&dst.Ident{
							Name: "conn",
						},
						&dst.Ident{
							Name: "err",
						},
					},
				},
			},
			want: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "Dial",
					Path: nrgrpc.GrpcImportPath,
				},
				Args: []dst.Expr{
					&dst.BasicLit{
						Value: `"localhost:8080"`,
						Kind:  token.STRING,
					},
				},
			},
			want1: true,
		},
		{
			name: "grpc Dial Expression Statement",
			args: args{
				node: &dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.Ident{
							Name: "Dial",
							Path: nrgrpc.GrpcImportPath,
						},
						Args: []dst.Expr{
							&dst.BasicLit{
								Value: `"localhost:8080"`,
								Kind:  token.STRING,
							},
						},
					},
				},
			},
			want: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "Dial",
					Path: nrgrpc.GrpcImportPath,
				},
				Args: []dst.Expr{
					&dst.BasicLit{
						Value: `"localhost:8080"`,
						Kind:  token.STRING,
					},
				},
			},
			want1: true,
		},
		{
			name: "non grpc dial expression",
			args: args{
				node: &dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.Ident{
							Name: "Dial",
							Path: "github.com/confluentinc/confluent-kafka-go",
						},
						Args: []dst.Expr{
							&dst.BasicLit{
								Value: `"localhost:8080"`,
								Kind:  token.STRING,
							},
						},
					},
				},
			},
			want:  nil,
			want1: false,
		},
		{
			name: "non grpc dial assignment",
			args: args{
				node: &dst.AssignStmt{
					Rhs: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "Dial",
								Path: "github.com/confluentinc/confluent-kafka-go",
							},
							Args: []dst.Expr{
								&dst.BasicLit{
									Value: `"localhost:8080"`,
									Kind:  token.STRING,
								},
							},
						},
					},
					Lhs: []dst.Expr{
						&dst.Ident{
							Name: "conn",
						},
						&dst.Ident{
							Name: "err",
						},
					},
				},
			},
			want:  nil,
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := nrgrpc.GrpcDialCall(tt.args.node)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nrgrpc.GrpcDialCall() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("nrgrpc.GrpcDialCall() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGrpcNewServerCall(t *testing.T) {
	type args struct {
		node dst.Node
	}
	tests := []struct {
		name  string
		args  args
		want  *dst.CallExpr
		want1 bool
	}{
		{
			name: "grpc NewServer Assign Statement",
			args: args{
				node: &dst.AssignStmt{
					Rhs: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "NewServer",
								Path: nrgrpc.GrpcImportPath,
							},
							Args: []dst.Expr{},
						},
					},
					Lhs: []dst.Expr{
						dst.NewIdent("grpcServer"),
					},
				},
			},
			want: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "NewServer",
					Path: nrgrpc.GrpcImportPath,
				},
				Args: []dst.Expr{},
			},
			want1: true,
		},
		{
			name: "grpc NewServer Expression Statement",
			args: args{
				node: &dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewServer",
							Path: nrgrpc.GrpcImportPath,
						},
						Args: []dst.Expr{},
					},
				},
			},
			want: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "NewServer",
					Path: nrgrpc.GrpcImportPath,
				},
				Args: []dst.Expr{},
			},
			want1: true,
		},
		{
			name: "non grpc Assign Statement",
			args: args{
				node: &dst.AssignStmt{
					Rhs: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "NewServer",
								Path: "github.com/confluentinc/confluent-kafka-go",
							},
							Args: []dst.Expr{},
						},
					},
					Lhs: []dst.Expr{
						dst.NewIdent("grpcServer"),
					},
				},
			},
			want:  nil,
			want1: false,
		},
		{
			name: "grpc NewServer Expression Statement",
			args: args{
				node: &dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewServer",
							Path: "github.com/confluentinc/confluent-kafka-go",
						},
						Args: []dst.Expr{},
					},
				},
			},
			want:  nil,
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := nrgrpc.GrpcNewServerCall(tt.args.node)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nrgrpc.GrpcNewServerCall() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("nrgrpc.GrpcNewServerCall() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestIsGrpcRegisterServerCall(t *testing.T) {
	serverArg := &dst.Ident{
		Name: "grpcServer",
	}
	astServer := &ast.Ident{
		Name: "grpcServer",
	}
	functionCallExpr := &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "RegisterTestServer",
			Path: "testGrpcPackage",
		},
		Args: []dst.Expr{
			serverArg,
			&dst.Ident{}, // not relevant
		},
	}

	pkg := &decorator.Package{
		Package: &packages.Package{
			TypesInfo: &types.Info{
				Types: map[ast.Expr]types.TypeAndValue{
					astServer: {
						Type: types.NewPointer(types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage("google.golang.org/grpc", "google.golang.org/grpc"), "Server", nil), nil, nil)),
					},
				},
			},
		},
		Decorator: &decorator.Decorator{
			Map: decorator.Map{
				Ast: decorator.AstMap{
					Nodes: map[dst.Node]ast.Node{serverArg: astServer},
				},
			},
		},
	}

	ok := nrgrpc.IsGrpcRegisterServerCall(functionCallExpr, pkg)
	if !ok {
		t.Error("expected valid server to return true")
	}

	// long call name
	functionCallExpr.Fun = &dst.Ident{
		Name: "RegisterTestFooBarServer",
		Path: "testGrpcPackage",
	}
	ok = nrgrpc.IsGrpcRegisterServerCall(functionCallExpr, pkg)
	if !ok {
		t.Error("expected valid server to return true")
	}

	// invalid call name
	functionCallExpr.Fun = &dst.Ident{
		Name: "RegisterTestService",
		Path: "testGrpcPackage",
	}
	ok = nrgrpc.IsGrpcRegisterServerCall(functionCallExpr, pkg)
	if ok {
		t.Error("expected invalid call to return false")
	}

}

func TestGetRegisteredServerIdent(t *testing.T) {
	type args struct {
		call *dst.CallExpr
	}
	tests := []struct {
		name   string
		args   args
		want   *dst.Ident
		expect bool
	}{
		{
			name: "server object ident",
			args: args{
				call: &dst.CallExpr{
					Args: []dst.Expr{
						&dst.Ident{},
						&dst.Ident{
							Name: "ServerHandler",
						},
					},
				},
			},
			want:   &dst.Ident{Name: "ServerHandler"},
			expect: true,
		},
		{
			name: "server object literal",
			args: args{
				call: &dst.CallExpr{
					Args: []dst.Expr{
						&dst.Ident{},
						&dst.UnaryExpr{
							Op: token.AND,
							X: &dst.CompositeLit{
								Type: &dst.Ident{
									Name: "ServerHandler",
								},
							},
						},
					},
				},
			},
			want:   &dst.Ident{Name: "ServerHandler"},
			expect: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nrgrpc.GetRegisteredServerIdent(tt.args.call)
			if ok && tt.expect {
				assert.Equal(t, tt.want, got)
			}
			assert.Equal(t, tt.expect, ok)
		})
	}
}

func TestFindGrpcServerObject(t *testing.T) {
	serverArg := &dst.Ident{
		Name: "grpcServer",
	}
	astServer := &ast.Ident{
		Name: "grpcServer",
	}
	handlerIdent := &dst.Ident{
		Name: "ServerHandler",
	}
	astHandler := &ast.Ident{
		Name: "ServerHandler",
	}

	functionCallExpr := &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.Ident{
				Name: "RegisterTestServer",
				Path: "testGrpcPackage",
			},
			Args: []dst.Expr{
				serverArg,
				handlerIdent,
			},
		},
	}

	pkg := &decorator.Package{
		Package: &packages.Package{
			TypesInfo: &types.Info{
				Types: map[ast.Expr]types.TypeAndValue{
					astServer: {
						Type: types.NewPointer(types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage("google.golang.org/grpc", "google.golang.org/grpc"), "Server", nil), nil, nil)),
					},
					astHandler: {
						Type: types.NewPointer(types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage("github.com/example/testpackage", "testpackage"), "ServerHandler", nil), nil, nil)),
					},
				},
			},
		},
		Decorator: &decorator.Decorator{
			Map: decorator.Map{
				Ast: decorator.AstMap{
					Nodes: map[dst.Node]ast.Node{serverArg: astServer, handlerIdent: astHandler},
				},
			},
		},
	}

	fact, ok := nrgrpc.FindGrpcServerObject(pkg, functionCallExpr)
	if !ok {
		t.Error("expected valid server to return true")
	} else {
		if fact.Name != "*github.com/example/testpackage.ServerHandler" {
			t.Errorf("expected server object to be *github.com/example/testpackage.ServerHandler, got %s", fact.Name)
		}
		if fact.Fact != facts.GrpcServerType {
			t.Errorf("expected fact to be GrpcServerType, got %s", fact.Fact)
		}
	}
}

