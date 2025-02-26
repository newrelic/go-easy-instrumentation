package tracestate

import (
	"go/ast"
	"go/types"
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate/traceobject"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestState_AddToCall(t *testing.T) {
	unrecognizedContext := &dst.Ident{Name: "unknownCtx"}
	astUnrecognizedContext := &ast.Ident{Name: "unknownCtx"}
	knownContext := &dst.Ident{Name: "ctx", Path: "context"}
	astContext := &ast.Ident{Name: "context"}
	defaultDecorator := &decorator.Package{
		Decorator: &decorator.Decorator{
			Map: decorator.Map{
				Ast: decorator.AstMap{
					Nodes: map[dst.Node]ast.Node{
						knownContext:        astContext,
						unrecognizedContext: astUnrecognizedContext,
					},
				},
			},
		},
		Package: &packages.Package{
			TypesInfo: &types.Info{
				Types: map[ast.Expr]types.TypeAndValue{
					astContext: {
						Type: types.NewNamed(types.NewTypeName(0, types.NewPackage("context", "context"), "Context", nil), nil, nil),
					},
					astUnrecognizedContext: {
						Type: types.NewNamed(types.NewTypeName(0, types.NewPackage("context", "context"), "Context", nil), nil, nil),
					},
				},
			},
		},
	}
	type fields struct {
		main            bool
		definedTxn      bool
		async           bool
		needsSegment    bool
		addTracingParam bool
		agentVariable   string
		txnVariable     string
		object          traceobject.TraceObject
	}
	type args struct {
		pkg   *decorator.Package
		call  *dst.CallExpr
		async bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *State
		want1  string
		want2  *dst.CallExpr
	}{
		{
			name: "function call with transaction doesnt have transaction added",
			fields: fields{
				txnVariable: codegen.DefaultTransactionVariable,
				object:      traceobject.NewTransaction(),
			},
			args: args{
				pkg:   defaultDecorator,
				call:  &dst.CallExpr{Args: []dst.Expr{dst.NewIdent(codegen.DefaultTransactionVariable)}},
				async: false,
			},
			want: &State{
				needsSegment:     true,
				addTracingParam:  true,
				txnVariable:      codegen.DefaultTransactionVariable,
				object:           traceobject.NewTransaction(),
				funcLitVariables: make(map[string]*dst.FuncLit),
			},
			want1: codegen.NewRelicAgentImportPath,
			want2: &dst.CallExpr{Args: []dst.Expr{dst.NewIdent(codegen.DefaultTransactionVariable)}},
		},
		{
			name: "function call without transaction has transaction added",
			fields: fields{
				txnVariable: codegen.DefaultTransactionVariable,
				object:      traceobject.NewTransaction(),
			},
			args: args{
				pkg:   defaultDecorator,
				call:  &dst.CallExpr{Args: []dst.Expr{}},
				async: false,
			},
			want: &State{
				needsSegment:     true,
				addTracingParam:  true,
				txnVariable:      codegen.DefaultTransactionVariable,
				object:           traceobject.NewTransaction(),
				funcLitVariables: make(map[string]*dst.FuncLit),
			},
			want1: codegen.NewRelicAgentImportPath,
			want2: &dst.CallExpr{Args: []dst.Expr{dst.NewIdent(codegen.DefaultTransactionVariable)}},
		},
		{
			name: "function call with context has transaction added to context",
			fields: fields{
				txnVariable: codegen.DefaultTransactionVariable,
				object:      traceobject.NewTransaction(),
			},
			args: args{
				pkg:   defaultDecorator,
				call:  &dst.CallExpr{Args: []dst.Expr{knownContext}},
				async: false,
			},
			want: &State{
				needsSegment:     true,
				addTracingParam:  true,
				txnVariable:      codegen.DefaultTransactionVariable,
				object:           traceobject.NewContext(),
				funcLitVariables: make(map[string]*dst.FuncLit),
			},
			want1: codegen.NewRelicAgentImportPath,
			want2: &dst.CallExpr{Args: []dst.Expr{codegen.WrapContextExpression(knownContext, codegen.DefaultTransactionVariable, false)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &State{
				main:            tt.fields.main,
				definedTxn:      tt.fields.definedTxn,
				async:           tt.fields.async,
				needsSegment:    tt.fields.needsSegment,
				addTracingParam: tt.fields.addTracingParam,
				agentVariable:   tt.fields.agentVariable,
				txnVariable:     tt.fields.txnVariable,
				object:          tt.fields.object,
			}
			state, got1 := tc.AddToCall(tt.args.pkg, tt.args.call, tt.args.async)
			if got1 != tt.want1 {
				t.Errorf("State.AddToCall() got1 = %v, want %v", got1, tt.want1)
			}
			assert.Equal(t, tt.want, state, "expected state")
			assert.Equal(t, tt.want2.Args, tt.args.call.Args, "expected call arguments")
		})
	}
}

func TestState_AddParameterToDeclaration(t *testing.T) {
	knownContext := codegen.NewContextParameter("ctx")
	astContext := &ast.Field{Names: []*ast.Ident{{Name: "ctx"}}, Type: &ast.Ident{Name: "Context"}}
	defaultDecorator := &decorator.Package{
		Decorator: &decorator.Decorator{
			Map: decorator.Map{
				Ast: decorator.AstMap{
					Nodes: map[dst.Node]ast.Node{
						knownContext:      astContext,
						knownContext.Type: astContext.Type,
					},
				},
			},
		},
		Package: &packages.Package{
			TypesInfo: &types.Info{
				Types: map[ast.Expr]types.TypeAndValue{
					astContext.Type: {
						Type: types.NewNamed(types.NewTypeName(0, types.NewPackage("context", "context"), "Context", nil), nil, nil),
					},
				},
			},
		},
	}

	createTestFunction := func(params ...*dst.Field) dst.Node {
		fun := &dst.FuncDecl{Name: &dst.Ident{Name: "foo"}, Type: &dst.FuncType{Params: &dst.FieldList{List: []*dst.Field{}}}, Body: &dst.BlockStmt{}}
		for _, param := range params {
			fun.Type.Params.List = append(fun.Type.Params.List, param)
		}
		return fun
	}

	type args struct {
		pkg  *decorator.Package
		node dst.Node
	}
	tests := []struct {
		name       string
		args       args
		state      *State
		wantImport string
		expect     dst.Node
	}{
		{
			name: "empty function declaration in function call",
			args: args{
				pkg:  defaultDecorator,
				node: createTestFunction(),
			},
			state:      FunctionBody(codegen.DefaultTransactionVariable, traceobject.NewTransaction()).functionCall(traceobject.NewTransaction()),
			wantImport: codegen.NewRelicAgentImportPath,
			expect:     createTestFunction(codegen.NewTransactionParameter(codegen.DefaultTransactionVariable)),
		},
		{
			name: "function declaration with ctx in function call",
			args: args{
				pkg:  defaultDecorator,
				node: createTestFunction(knownContext),
			},
			state:      FunctionBody(codegen.DefaultTransactionVariable, traceobject.NewContext()).functionCall(traceobject.NewContext()),
			wantImport: "",
			expect:     createTestFunction(knownContext),
		},
		{
			name: "empty function declaration in Main",
			args: args{
				pkg:  defaultDecorator,
				node: createTestFunction(),
			},
			state:      Main(codegen.DefaultTransactionVariable),
			wantImport: "",
			expect:     createTestFunction(),
		},
		{
			name: "empty function declaration in function body",
			args: args{
				pkg:  defaultDecorator,
				node: createTestFunction(),
			},
			state:      FunctionBody(codegen.DefaultTransactionVariable),
			wantImport: "",
			expect:     createTestFunction(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := tt.state.AddParameterToDeclaration(tt.args.pkg, tt.args.node)
			if got != tt.wantImport {
				t.Errorf("State.AddParameterToDeclaration() import string = \"%v\", want \"%v\"", got, tt.wantImport)
			}
			assert.Equal(t, tt.expect, tt.args.node, "expected node after adding parameter")
		})
	}
}

func TestState_CreateSegment(t *testing.T) {
	type args struct {
		node dst.Node
	}
	tests := []struct {
		name  string
		state *State
		args  args
		want  string
		want1 bool
	}{
		{
			name:  "do not create segments when tracing main",
			state: Main("foo"),
			args: args{node: &dst.FuncDecl{
				Name: &dst.Ident{Name: "bar"},
				Body: &dst.BlockStmt{},
			}},
			want:  "",
			want1: false,
		},
		{
			name:  "create segments when tracing functions called from main",
			state: Main("foo").functionCall(traceobject.NewTransaction()),
			args: args{node: &dst.FuncDecl{
				Name: &dst.Ident{Name: "bar"},
				Body: &dst.BlockStmt{},
			}},
			want:  codegen.NewRelicAgentImportPath,
			want1: true,
		},
		{
			name:  "create segments when tracing func literals called from main",
			state: Main("foo").goroutine(traceobject.NewTransaction()),
			args: args{node: &dst.FuncLit{
				Body: &dst.BlockStmt{},
			}},
			want:  codegen.NewRelicAgentImportPath,
			want1: true,
		},
		{
			name:  "create segments when tracing functions called from a chain of function calls",
			state: FunctionBody(codegen.DefaultTransactionVariable).functionCall(traceobject.NewContext()),
			args: args{node: &dst.FuncDecl{
				Name: &dst.Ident{Name: "bar"},
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: []*dst.Field{
							codegen.NewContextParameter("ctx"),
						},
					},
				},
				Body: &dst.BlockStmt{},
			}},
			want:  codegen.NewRelicAgentImportPath,
			want1: true,
		},
		{
			name:  "async function call in a chain of function calls",
			state: FunctionBody(codegen.DefaultTransactionVariable).goroutine(traceobject.NewContext()),
			args: args{
				node: &dst.FuncDecl{
					Name: &dst.Ident{Name: "bar"},
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{
								codegen.NewContextParameter("ctx"),
							},
						},
					},
					Body: &dst.BlockStmt{},
				},
			},
			want:  codegen.NewRelicAgentImportPath,
			want1: true,
		},
		{
			name:  "async function literal in a chain of function calls",
			state: Main("foo").functionCall(traceobject.NewTransaction()).goroutine(traceobject.NewContext()),
			args: args{
				node: &dst.FuncLit{
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{
								codegen.NewContextParameter("ctx"),
							},
						},
					},
					Body: &dst.BlockStmt{},
				},
			},
			want:  codegen.NewRelicAgentImportPath,
			want1: true,
		},
		{
			name:  "create segments from context when tracing functions called from a traced function",
			state: Main("foo").functionCall(traceobject.NewContext()),
			args: args{node: &dst.FuncDecl{
				Name: &dst.Ident{Name: "bar"},
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: []*dst.Field{
							codegen.NewContextParameter("ctx"),
						},
					},
				},
				Body: &dst.BlockStmt{},
			}},
			want:  codegen.NewRelicAgentImportPath,
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := tt.state.CreateSegment(tt.args.node)
			if got != tt.want {
				t.Errorf("State.CreateSegment() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("State.CreateSegment() got1 = %v, want %v", got1, tt.want1)
			}

			if tt.want1 && tt.state.txnUsed != true {
				t.Errorf("Creating a segment requires a transaction variable, so \"txnUsed\" should be true, got %t", tt.state.txnUsed)
			}

			switch n := tt.args.node.(type) {
			case *dst.FuncDecl:
				if tt.state.IsMain() {
					if len(n.Body.List) != 0 {
						t.Fatalf("Segments should never be created in the main method")
					}
				} else {
					if len(n.Body.List) == 0 {
						t.Fatalf("no statement for creating a segment added to the function body: %+v", n.Body.List)
					}
					name := n.Name.Name
					if tt.state.async {
						name = "async " + name
					}
					assert.Equal(t, codegen.DeferSegment(name, dst.NewIdent(codegen.DefaultTransactionVariable)), n.Body.List[0], "expected segment statement")
				}
			case *dst.FuncLit:
				if tt.state.IsMain() {
					if len(n.Body.List) != 0 {
						t.Fatalf("Segments should never be created in the main method")
					}
				} else {
					if len(n.Body.List) == 0 {
						t.Fatalf("no statement for creating a segment added to the function body: %+v", n.Body.List)
					}
					name := "function literal"
					if tt.state.async {
						name = "async " + name
					}
					assert.Equal(t, codegen.DeferSegment(name, dst.NewIdent(codegen.DefaultTransactionVariable)), n.Body.List[0])
				}
			}
		})
	}
}

func TestState_TransactionVariable(t *testing.T) {
	tests := []struct {
		name  string
		state *State
		want  dst.Expr
	}{
		{
			name:  "default transaction variable",
			state: Main("foo"),
			want:  dst.NewIdent(codegen.DefaultTransactionVariable),
		},
		{
			name:  "custom transaction variable",
			state: FunctionBody("txn", traceobject.NewTransaction()),
			want:  dst.NewIdent("txn"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.TransactionVariable()
			if got == nil {
				t.Fatalf("State.TransactionVariable() = nil, want a transaction variable expression")
			}
			if tt.state.txnUsed != true {
				t.Errorf("TransactionVariable() should set txnUsed to true, got %t", tt.state.txnUsed)
			}

			assert.Equal(t, tt.want, got, "expected transaction variable")
		})
	}
}

func TestState_AgentVariable(t *testing.T) {
	tests := []struct {
		name  string
		state *State
		want  dst.Expr
	}{
		{
			name:  "Main Method",
			state: Main("foo"),
			want:  dst.NewIdent("foo"),
		},
		{
			name:  "Function Body",
			state: FunctionBody(codegen.DefaultTransactionVariable, traceobject.NewTransaction()),
			want:  codegen.GetApplication(dst.NewIdent(codegen.DefaultTransactionVariable)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.AgentVariable()
			if got == nil {
				t.Fatalf("State.TransactionVariable() = nil, want a transaction variable expression")
			}
			if !tt.state.IsMain() && !tt.state.txnUsed {
				t.Errorf("AgentVariable() should set txnUsed to true when not in main, got %t", tt.state.txnUsed)
			}

			assert.Equal(t, tt.want, got, "expected transaction variable")
		})
	}
}

func TestState_WrapWithTransaction(t *testing.T) {
	appName := "testApp"
	tests := []struct {
		name  string
		state *State
	}{
		{
			name:  "Main Method",
			state: Main(appName),
		},
		{
			name:  "Function Body",
			state: FunctionBody(codegen.DefaultTransactionVariable, traceobject.NewTransaction()),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFunc := &dst.FuncDecl{
				Name: &dst.Ident{Name: "testFunc"},
				Type: &dst.FuncType{},
				Body: &dst.BlockStmt{
					List: []dst.Stmt{
						&dst.ExprStmt{X: &dst.CallExpr{Fun: dst.NewIdent("foo")}},
						&dst.ExprStmt{X: &dst.CallExpr{Fun: dst.NewIdent("bar")}},
					},
				},
			}

			dstutil.Apply(testFunc, func(cursor *dstutil.Cursor) bool {
				switch n := cursor.Node().(type) {
				case *dst.ExprStmt:
					call := n.X.(*dst.CallExpr)
					fun := call.Fun.(*dst.Ident)
					tt.state.WrapWithTransaction(cursor, fun.Name, codegen.DefaultTransactionVariable)
					return false
				}
				return true
			}, nil)

			if tt.state.IsMain() {
				if len(testFunc.Body.List) != 6 {
					t.Fatalf("not enough statements in the function body: %+v", testFunc.Body.List)
				}
				assert.Equal(t, codegen.StartTransaction(appName, codegen.DefaultTransactionVariable, "foo", false), testFunc.Body.List[0])
				assert.Equal(t, codegen.EndTransaction(codegen.DefaultTransactionVariable), testFunc.Body.List[2])
				assert.Equal(t, codegen.StartTransaction(appName, codegen.DefaultTransactionVariable, "bar", true), testFunc.Body.List[3])
				assert.Equal(t, codegen.EndTransaction(codegen.DefaultTransactionVariable), testFunc.Body.List[5])
			} else {
				if len(testFunc.Body.List) != 2 {
					t.Fatalf("transaction wrapping should not occur outside of main methods: %+v", testFunc.Body.List)
				}
			}

		})
	}
}

func TestState_AssignTransactionVariable(t *testing.T) {
	appName := "testApp"
	tests := []struct {
		name             string
		state            *State
		getTxn           bool
		node             dst.Node
		wantImportString string
	}{
		{
			name:             "Main Method",
			state:            Main(appName),
			node:             &dst.FuncDecl{Name: &dst.Ident{Name: "foo"}, Body: &dst.BlockStmt{}},
			wantImportString: "",
		},
		{
			name:             "Function Body, unused txn",
			state:            FunctionBody(codegen.DefaultTransactionVariable, traceobject.NewContext()),
			node:             &dst.FuncDecl{Name: &dst.Ident{Name: "foo"}, Body: &dst.BlockStmt{}},
			wantImportString: "",
		},
		{
			name:             "Function Body, context parameter",
			state:            FunctionBody(codegen.DefaultTransactionVariable, traceobject.NewContext()),
			node:             &dst.FuncDecl{Name: &dst.Ident{Name: "foo"}, Body: &dst.BlockStmt{}},
			getTxn:           true,
			wantImportString: "",
		},
		{
			name:             "Function Body, txn parameter",
			state:            FunctionBody(codegen.DefaultTransactionVariable, traceobject.NewTransaction()),
			node:             &dst.FuncDecl{Name: &dst.Ident{Name: "foo"}, Body: &dst.BlockStmt{}},
			getTxn:           true,
			wantImportString: "",
		},
		{
			name:             "Function Body, txn parameter",
			state:            FunctionBody(codegen.DefaultTransactionVariable, traceobject.NewTransaction()),
			node:             &dst.FuncDecl{Name: &dst.Ident{Name: "foo"}, Body: &dst.BlockStmt{}},
			getTxn:           true,
			wantImportString: "",
		},
		{
			name:             "Function Lit Body, ctx parameter",
			state:            FunctionBody(codegen.DefaultTransactionVariable, traceobject.NewContext()),
			node:             &dst.FuncLit{Body: &dst.BlockStmt{}},
			getTxn:           true,
			wantImportString: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.getTxn {
				tt.state.TransactionVariable()
			}
			importString := tt.state.AssignTransactionVariable(tt.node)
			assert.Equal(t, tt.wantImportString, importString, "expected import string")
			if tt.state.IsMain() {
				switch n := tt.node.(type) {
				case *dst.FuncDecl:
					if len(n.Body.List) != 0 {
						t.Fatalf("no txn assignment statements should be added to the main function body: %+v", n.Body.List)
					}
				case *dst.FuncLit:
					if len(n.Body.List) != 0 {
						t.Fatalf("no txn assignment statements should be added to the main function body: %+v", n.Body.List)
					}
				}
			} else {
				switch n := tt.node.(type) {
				case *dst.FuncDecl:
					if reflect.TypeOf(tt.state.object) == reflect.TypeOf(traceobject.NewTransaction()) {
						if len(n.Body.List) != 0 {
							t.Fatalf("no txn assignment statements should be added when the txn is not used: %+v", n.Body.List)
						}
					} else if tt.getTxn {
						if len(n.Body.List) != 1 {
							t.Fatalf("expected one statement in the function body, got %+v", n.Body.List)
						}
					} else {
						if len(n.Body.List) != 0 {
							t.Fatalf("no txn assignment statements should be added when the txn is not used: %+v", n.Body.List)
						}
					}
				case *dst.FuncLit:
					if reflect.TypeOf(tt.state.object) == reflect.TypeOf(traceobject.NewTransaction()) {
						if len(n.Body.List) != 0 {
							t.Fatalf("no txn assignment statements should be added when the txn is not used: %+v", n.Body.List)
						}
					} else if tt.getTxn {
						if len(n.Body.List) != 1 {
							t.Fatalf("expected one statement in the function body, got %+v", n.Body.List)
						}
					} else {
						if len(n.Body.List) != 0 {
							t.Fatalf("no txn assignment statements should be added when the txn is not used: %+v", n.Body.List)
						}
					}
				}
			}
		})
	}
}
