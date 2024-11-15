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

// contextParameterType returns a new type object for a context argmuent
func contextParameterType() *dst.Ident {
	return &dst.Ident{
		Name: "Context",
		Path: "context",
	}
}

func TestContext_AddToCall(t *testing.T) {
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
		contextParameterName string
	}
	type wantArg struct {
		arg   dst.Expr
		index int
	}
	type args struct {
		pkg                     *decorator.Package
		call                    *dst.CallExpr
		transactionVariableName string
		async                   bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    AddToCallReturn
		wantArg wantArg
	}{
		{
			name: "function call with context argument",
			fields: fields{
				contextParameterName: "ctx",
			},
			args: args{
				pkg:                     defaultDecorator,
				call:                    &dst.CallExpr{Args: []dst.Expr{knownContext}},
				transactionVariableName: codegen.DefaultTransactionVariable,
				async:                   false,
			},
			want: AddToCallReturn{
				TraceObject: NewContext(),
				Import:      "",
				NeedsTx:     false,
			},
			wantArg: wantArg{
				arg:   knownContext,
				index: 0,
			},
		},
		{
			name: "function call with unrecognized context argument",
			fields: fields{
				contextParameterName: "ctx",
			},
			args: args{
				pkg:                     defaultDecorator,
				call:                    &dst.CallExpr{Args: []dst.Expr{unrecognizedContext}},
				transactionVariableName: codegen.DefaultTransactionVariable,
				async:                   false,
			},
			want: AddToCallReturn{
				TraceObject: NewContext(),
				Import:      codegen.NewRelicAgentImportPath,
				NeedsTx:     true,
			},
			wantArg: wantArg{
				arg:   codegen.WrapContextExpression(unrecognizedContext, codegen.DefaultTransactionVariable, false),
				index: 0,
			},
		},
		{
			name: "function call with context argument async",
			fields: fields{
				contextParameterName: "ctx",
			},
			args: args{
				pkg:                     defaultDecorator,
				call:                    &dst.CallExpr{Args: []dst.Expr{knownContext}},
				transactionVariableName: codegen.DefaultTransactionVariable,
				async:                   true,
			},
			want: AddToCallReturn{
				TraceObject: NewContext(),
				Import:      codegen.NewRelicAgentImportPath,
				NeedsTx:     true,
			},
			wantArg: wantArg{
				arg:   codegen.WrapContextExpression(knownContext, codegen.DefaultTransactionVariable, true),
				index: 0,
			},
		},
		{
			name: "function call with no arguments gets a transaction added",
			fields: fields{
				contextParameterName: "ctx",
			},
			args: args{
				pkg:                     defaultDecorator,
				call:                    &dst.CallExpr{Args: []dst.Expr{}},
				transactionVariableName: codegen.DefaultTransactionVariable,
				async:                   false,
			},
			want: AddToCallReturn{
				TraceObject: NewTransaction(),
				Import:      codegen.NewRelicAgentImportPath,
				NeedsTx:     true,
			},
			wantArg: wantArg{
				arg:   dst.NewIdent(codegen.DefaultTransactionVariable),
				index: 0,
			},
		},
		{
			name: "function call with no arguments gets a transaction async added",
			fields: fields{
				contextParameterName: "ctx",
			},
			args: args{
				pkg:                     defaultDecorator,
				call:                    &dst.CallExpr{Args: []dst.Expr{}},
				transactionVariableName: codegen.DefaultTransactionVariable,
				async:                   true,
			},
			want: AddToCallReturn{
				TraceObject: NewTransaction(),
				Import:      codegen.NewRelicAgentImportPath,
				NeedsTx:     true,
			},
			wantArg: wantArg{
				arg:   codegen.TxnNewGoroutine(dst.NewIdent(codegen.DefaultTransactionVariable)),
				index: 0,
			},
		},
		{
			name: "function call with many argunents and context argument",
			fields: fields{
				contextParameterName: "ctx",
			},
			args: args{
				pkg:                     defaultDecorator,
				call:                    &dst.CallExpr{Args: []dst.Expr{dst.NewIdent("hi"), dst.NewIdent("hello"), knownContext, dst.NewIdent("goodbye")}},
				transactionVariableName: codegen.DefaultTransactionVariable,
				async:                   false,
			},
			want: AddToCallReturn{
				TraceObject: NewContext(),
				Import:      "",
				NeedsTx:     false,
			},
			wantArg: wantArg{
				arg:   knownContext,
				index: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				contextParameterName: tt.fields.contextParameterName,
			}
			ret := ctx.AddToCall(tt.args.pkg, tt.args.call, tt.args.transactionVariableName, tt.args.async)

			if ret.Import != tt.want.Import {
				t.Errorf("AddToCall() import incorrect: got = %v, want %v", ret.Import, tt.want.Import)
			}
			if ret.NeedsTx != tt.want.NeedsTx {
				t.Errorf("AddToCall() NeedsTx incorrect: got = %v, want %v", ret.NeedsTx, tt.want.NeedsTx)
			}

			assert.Equal(t, tt.want.TraceObject, ret.TraceObject, "AddToCall() TraceObject incorrect")

			if len(tt.args.call.Args) == 0 {
				t.Fatal("AddToCall() did not add a trace object to the call")
			}
			if len(tt.args.call.Args) < tt.wantArg.index+1 {
				t.Fatalf("AddToCall() did not add a trace object to the correct index: len(args) = %v, want index %d", len(tt.args.call.Args), tt.wantArg.index+1)
			}

			assert.Equal(t, tt.wantArg.arg, tt.args.call.Args[tt.wantArg.index], "AddToCall() did not add the correct trace object to the call")
		})
	}
}

func TestContext_AddToFuncDecl(t *testing.T) {
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
			ctx := NewContext()
			gotTO, gotImport := ctx.AddToFuncDecl(tt.args.pkg, tt.args.decl)
			assert.Equal(t, tt.wantTO, gotTO, "AddToFuncDecl() TraceObject incorrect")
			assert.Equal(t, tt.wantImport, gotImport, "AddToFuncDecl() Import incorrect")
			assert.Equal(t, tt.wantParams, tt.args.decl.Type.Params.List, "AddToFuncDecl() did not add the correct trace object to the function declaration")
		})
	}
}

func TestContext_AddToFuncLit(t *testing.T) {
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
			ctx := NewContext()
			gotTO, gotImport := ctx.AddToFuncLit(tt.args.pkg, tt.args.decl)
			assert.Equal(t, tt.wantTO, gotTO, "AddToFuncDecl() TraceObject incorrect")
			assert.Equal(t, tt.wantImport, gotImport, "AddToFuncDecl() Import incorrect")
			assert.Equal(t, tt.wantParams, tt.args.decl.Type.Params.List, "AddToFuncDecl() did not add the correct trace object to the function declaration")
		})
	}
}
