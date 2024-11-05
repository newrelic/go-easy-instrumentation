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
	knownContext := &dst.Ident{Name: "ctx", Path: "context"}
	astContext := &ast.Ident{Name: "context"}
	defaultDecorator := &decorator.Package{
		Decorator: &decorator.Decorator{
			Map: decorator.Map{
				Ast: decorator.AstMap{
					Nodes: map[dst.Node]ast.Node{
						knownContext: astContext,
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
