package tracestate

import (
	"go/ast"
	"go/types"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
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
				needsSegment:    true,
				addTracingParam: true,
				txnVariable:     codegen.DefaultTransactionVariable,
				object:          traceobject.NewTransaction(),
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
				needsSegment:    true,
				addTracingParam: true,
				txnVariable:     codegen.DefaultTransactionVariable,
				object:          traceobject.NewTransaction(),
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
				needsSegment:    true,
				addTracingParam: true,
				txnVariable:     codegen.DefaultTransactionVariable,
				object:          traceobject.NewContext(),
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
			name:  "create segments when tracing functions called from a chain of function calls",
			state: Main("foo").functionCall(traceobject.NewTransaction()).functionCall(traceobject.NewContext()),
			args: args{node: &dst.FuncDecl{
				Name: &dst.Ident{Name: "bar"},
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: []*dst.Field{
							codegen.ContextParameter("ctx"),
						},
					},
				},
				Body: &dst.BlockStmt{},
			}},
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
							codegen.ContextParameter("ctx"),
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
