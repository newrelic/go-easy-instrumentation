package parser

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
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
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentGrpcDial)
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
			defer panicRecovery(t)
			got := testStatefulTracingFunction(t, tt.code, InstrumentGrpcServer, false)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func Test_grpcDialCall(t *testing.T) {
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
								Path: codegen.GrpcImportPath,
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
					Path: codegen.GrpcImportPath,
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
							Path: codegen.GrpcImportPath,
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
					Path: codegen.GrpcImportPath,
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
			got, got1 := grpcDialCall(tt.args.node)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("grpcDialCall() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("grpcDialCall() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_grpcNewServerCall(t *testing.T) {
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
								Path: codegen.GrpcImportPath,
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
					Path: codegen.GrpcImportPath,
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
							Path: codegen.GrpcImportPath,
						},
						Args: []dst.Expr{},
					},
				},
			},
			want: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "NewServer",
					Path: codegen.GrpcImportPath,
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
			got, got1 := grpcNewServerCall(tt.args.node)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("grpcNewServerCall() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("grpcNewServerCall() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_isGrpcRegisterServerCall(t *testing.T) {
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

	ok := isGrpcRegisterServerCall(functionCallExpr, pkg)
	if !ok {
		t.Error("expected valid server to return true")
	}

	// long call name
	functionCallExpr.Fun = &dst.Ident{
		Name: "RegisterTestFooBarServer",
		Path: "testGrpcPackage",
	}
	ok = isGrpcRegisterServerCall(functionCallExpr, pkg)
	if !ok {
		t.Error("expected valid server to return true")
	}

	// invalid call name
	functionCallExpr.Fun = &dst.Ident{
		Name: "RegisterTestService",
		Path: "testGrpcPackage",
	}
	ok = isGrpcRegisterServerCall(functionCallExpr, pkg)
	if ok {
		t.Error("expected invalid call to return false")
	}

}

func Test_getRegisteredServerIdent(t *testing.T) {
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
			got, ok := getRegisteredServerIdent(tt.args.call)
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

	fact, ok := FindGrpcServerObject(pkg, functionCallExpr)
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

func TestFindGrpcServerStreamInterface(t *testing.T) {
	validTypeSpec := &dst.TypeSpec{
		Name: &dst.Ident{
			Name: "TestApp_StreamServer",
			Path: "github.com/example/testapp",
		},
		Type: &dst.InterfaceType{
			Methods: &dst.FieldList{
				List: []*dst.Field{
					{
						Type: &dst.Ident{
							Name: "ServerStream",
							Path: "google.golang.org/grpc",
						},
					},
				},
			},
		},
	}

	astTypeSpecIdent := &ast.Ident{
		Name: "TestApp_StreamServer",
	}

	testPkg := &decorator.Package{
		Package: &packages.Package{
			TypesInfo: &types.Info{
				Types: map[ast.Expr]types.TypeAndValue{
					astTypeSpecIdent: {
						Type: types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage("github.com/example/testapp", "testapp"), "TestApp_StreamServer", types.NewInterfaceType(nil, nil)), nil, nil),
					},
				},
			},
		},
		Decorator: &decorator.Decorator{
			Map: decorator.Map{
				Ast: decorator.AstMap{
					Nodes: map[dst.Node]ast.Node{validTypeSpec.Name: astTypeSpecIdent},
				},
			},
		},
	}

	type args struct {
		pkg  *decorator.Package
		node dst.Node
	}
	tests := []struct {
		name     string
		args     args
		wantFact facts.Entry
		want     bool
	}{
		{
			name: "valid stream server",
			args: args{
				pkg: testPkg,
				node: &dst.GenDecl{
					Specs: []dst.Spec{validTypeSpec},
				},
			},
			wantFact: facts.Entry{
				Name: "github.com/example/testapp.TestApp_StreamServer",
				Fact: facts.GrpcServerStream,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotOk := FindGrpcServerStreamInterface(tt.args.pkg, tt.args.node)
			if tt.want {
				if !gotOk {
					t.Error("expected FindGrpcServerStreamInterface to return a fact entry")
				} else {
					assert.Equal(t, tt.wantFact, got)
				}
			} else if gotOk {
				t.Errorf("expected FindGrpcServerStreamInterface to return false, but got true and %s", got)
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

	manager := &InstrumentationManager{
		currentPackage: "test",
		packages: map[string]*packageState{
			"test": {
				pkg: &decorator.Package{
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
			},
		},
		facts: facts.Keeper{
			"*github.com/example/testapp.Server": facts.GrpcServerType,
		},
	}

	type args struct {
		manager *InstrumentationManager
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
			if got := isGrpcServerMethod(tt.args.manager, tt.args.decl); got != tt.want {
				t.Errorf("isGrpcServerMethod() = %v, want %v", got, tt.want)
			}
		})
	}
}
