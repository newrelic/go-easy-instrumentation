package traceobject

import (
	"go/ast"
	"go/types"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestTransaction_AddToCall(t *testing.T) {
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

	type args struct {
		pkg                 *decorator.Package
		call                *dst.CallExpr
		transactionVariable string
		async               bool
	}
	tests := []struct {
		name        string
		args        args
		wantTO      TraceObject
		wantImport  string
		wantNeedTxn bool
		wantArgs    []dst.Expr
	}{
		{
			name: "function call without any arguments gets a transaction added",
			args: args{
				pkg: defaultDecorator,
				call: &dst.CallExpr{
					Fun:  &dst.Ident{Name: "foo", Path: "bar"},
					Args: []dst.Expr{},
				},
				transactionVariable: codegen.DefaultTransactionVariable,
				async:               false,
			},
			wantTO:      NewTransaction(),
			wantImport:  codegen.NewRelicAgentImportPath,
			wantNeedTxn: true,
			wantArgs: []dst.Expr{
				&dst.Ident{Name: codegen.DefaultTransactionVariable},
			},
		},
		{
			name: "async function call without any arguments gets a transaction added",
			args: args{
				pkg: defaultDecorator,
				call: &dst.CallExpr{
					Fun:  &dst.Ident{Name: "foo", Path: "bar"},
					Args: []dst.Expr{},
				},
				transactionVariable: codegen.DefaultTransactionVariable,
				async:               true,
			},
			wantTO:      NewTransaction(),
			wantImport:  codegen.NewRelicAgentImportPath,
			wantNeedTxn: true,
			wantArgs: []dst.Expr{
				codegen.TxnNewGoroutine(dst.NewIdent(codegen.DefaultTransactionVariable)),
			},
		},
		{
			name: "function call with context argument gets transaction injected",
			args: args{
				pkg: defaultDecorator,
				call: &dst.CallExpr{
					Fun:  &dst.Ident{Name: "foo", Path: "bar"},
					Args: []dst.Expr{knownContext},
				},
				transactionVariable: codegen.DefaultTransactionVariable,
				async:               false,
			},
			wantTO:      NewContext(),
			wantImport:  codegen.NewRelicAgentImportPath,
			wantNeedTxn: true,
			wantArgs: []dst.Expr{
				codegen.WrapContextExpression(knownContext, codegen.DefaultTransactionVariable, false),
			},
		},
		{
			name: "async function call with context argument gets transaction injected",
			args: args{
				pkg: defaultDecorator,
				call: &dst.CallExpr{
					Fun:  &dst.Ident{Name: "foo", Path: "bar"},
					Args: []dst.Expr{knownContext},
				},
				transactionVariable: codegen.DefaultTransactionVariable,
				async:               true,
			},
			wantTO:      NewContext(),
			wantImport:  codegen.NewRelicAgentImportPath,
			wantNeedTxn: true,
			wantArgs: []dst.Expr{
				codegen.WrapContextExpression(knownContext, codegen.DefaultTransactionVariable, true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txn := NewTransaction()
			got := txn.AddToCall(tt.args.pkg, tt.args.call, tt.args.transactionVariable, tt.args.async)
			assert.Equal(t, tt.wantTO, got.TraceObject, "retuned trace object is not the same as the expected type")
			assert.Equal(t, tt.wantImport, got.Import, "import path is not the same as the expected import path")
			assert.Equal(t, tt.wantNeedTxn, got.NeedsTx, "needs transaction is not the same as the expected value")
			assert.Equal(t, tt.wantArgs, tt.args.call.Args, "args are not the same as the expected args")
		})
	}
}

func TestTransaction_AddToFuncDecl(t *testing.T) {
	contextType := contextParameterType()
	astContextType := &ast.SelectorExpr{X: &ast.Ident{Name: "context"}, Sel: &ast.Ident{Name: "Context"}}
	contextFieldName := dst.NewIdent("ctx")
	astContextFieldName := &ast.Ident{Name: "ctx"}
	contextField := &dst.Field{
		Names: []*dst.Ident{contextFieldName},
		Type:  contextType,
	}
	astContextField := &ast.Field{
		Names: []*ast.Ident{astContextFieldName},
		Type:  astContextType,
	}

	defaultDecorator := &decorator.Package{
		Decorator: &decorator.Decorator{
			Map: decorator.Map{
				Ast: decorator.AstMap{
					Nodes: map[dst.Node]ast.Node{
						contextType:      astContextType,
						contextField:     astContextField,
						contextFieldName: astContextFieldName,
					},
				},
			},
		},
		Package: &packages.Package{
			TypesInfo: &types.Info{
				Types: map[ast.Expr]types.TypeAndValue{
					astContextType: {
						Type: types.NewNamed(types.NewTypeName(0, types.NewPackage("context", "context"), "Context", nil), nil, nil),
					},
					astContextFieldName: {
						Type: types.NewNamed(types.NewTypeName(0, types.NewPackage("context", "context"), "Context", nil), nil, nil),
					},
				},
			},
		},
	}
	type args struct {
		pkg  *decorator.Package
		decl *dst.FuncDecl
	}
	tests := []struct {
		name       string
		args       args
		wantTO     TraceObject
		wantImport string
		wantParams []*dst.Field
	}{
		{
			name: "function declaration with context argument",
			args: args{
				pkg: defaultDecorator,
				decl: &dst.FuncDecl{
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{contextField},
						},
					},
				},
			},
			wantTO:     &Context{contextParameterName: "ctx"},
			wantImport: "",
			wantParams: []*dst.Field{
				contextField,
			},
		},
		{
			name: "function declaration without argument",
			args: args{
				pkg: defaultDecorator,
				decl: &dst.FuncDecl{
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{},
						},
					},
				},
			},
			wantTO:     NewTransaction(),
			wantImport: codegen.NewRelicAgentImportPath,
			wantParams: []*dst.Field{
				codegen.NewTransactionParameter(codegen.DefaultTransactionVariable),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txn := NewTransaction()
			gotTO, gotImport := txn.AddToFuncDecl(tt.args.pkg, tt.args.decl)
			assert.Equal(t, tt.wantTO, gotTO, "AddToFuncDecl() TraceObject incorrect")
			assert.Equal(t, tt.wantImport, gotImport, "AddToFuncDecl() Import incorrect")
			assert.Equal(t, tt.wantParams, tt.args.decl.Type.Params.List, "AddToFuncDecl() did not add the correct trace object to the function declaration")
		})
	}
}

func TestTransaction_AddToFuncLit(t *testing.T) {
	contextType := contextParameterType()
	astContextType := &ast.SelectorExpr{X: &ast.Ident{Name: "context"}, Sel: &ast.Ident{Name: "Context"}}
	contextFieldName := dst.NewIdent("ctx")
	astContextFieldName := &ast.Ident{Name: "ctx"}
	contextField := &dst.Field{
		Names: []*dst.Ident{contextFieldName},
		Type:  contextType,
	}
	astContextField := &ast.Field{
		Names: []*ast.Ident{astContextFieldName},
		Type:  astContextType,
	}

	defaultDecorator := &decorator.Package{
		Decorator: &decorator.Decorator{
			Map: decorator.Map{
				Ast: decorator.AstMap{
					Nodes: map[dst.Node]ast.Node{
						contextType:      astContextType,
						contextField:     astContextField,
						contextFieldName: astContextFieldName,
					},
				},
			},
		},
		Package: &packages.Package{
			TypesInfo: &types.Info{
				Types: map[ast.Expr]types.TypeAndValue{
					astContextType: {
						Type: types.NewNamed(types.NewTypeName(0, types.NewPackage("context", "context"), "Context", nil), nil, nil),
					},
					astContextFieldName: {
						Type: types.NewNamed(types.NewTypeName(0, types.NewPackage("context", "context"), "Context", nil), nil, nil),
					},
				},
			},
		},
	}
	type args struct {
		pkg  *decorator.Package
		decl *dst.FuncLit
	}
	tests := []struct {
		name       string
		args       args
		wantTO     TraceObject
		wantImport string
		wantParams []*dst.Field
	}{
		{
			name: "function declaration with context argument",
			args: args{
				pkg: defaultDecorator,
				decl: &dst.FuncLit{
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{contextField},
						},
					},
				},
			},
			wantTO:     &Context{contextParameterName: "ctx"},
			wantImport: "",
			wantParams: []*dst.Field{
				contextField,
			},
		},
		{
			name: "function declaration without argument",
			args: args{
				pkg: defaultDecorator,
				decl: &dst.FuncLit{
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{},
						},
					},
				},
			},
			wantTO:     NewTransaction(),
			wantImport: codegen.NewRelicAgentImportPath,
			wantParams: []*dst.Field{
				codegen.NewTransactionParameter(codegen.DefaultTransactionVariable),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txn := NewTransaction()
			gotTO, gotImport := txn.AddToFuncLit(tt.args.pkg, tt.args.decl)
			assert.Equal(t, tt.wantTO, gotTO, "AddToFuncDecl() TraceObject incorrect")
			assert.Equal(t, tt.wantImport, gotImport, "AddToFuncDecl() Import incorrect")
			assert.Equal(t, tt.wantParams, tt.args.decl.Type.Params.List, "AddToFuncDecl() did not add the correct trace object to the function declaration")
		})
	}
}
