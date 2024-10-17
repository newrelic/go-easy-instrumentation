package tracecontext

import (
	"testing"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
)

func TestTransaction_Pass(t *testing.T) {
	tests := []passTest{
		{
			name: "pass transaction to function",
			tc:   NewTransaction("tx"),
			args: passTestArgs{
				decl: &dst.FuncDecl{
					Name: dst.NewIdent("foo"),
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{
								{
									Names: []*dst.Ident{dst.NewIdent("tx")},
									Type:  transactionArgumentType(),
								},
							},
						},
					},
				},
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "foo",
						Path: "baz",
					},
					Args: []dst.Expr{},
				},
			},
			wantStatements: nil,
			wantArgs:       []dst.Expr{dst.NewIdent("tx")},
			wantTc:         NewTransaction("tx"),
			wantErr:        nil,
		},
		{
			name: "pass transaction to function call wih argument",
			tc:   NewTransaction("tx"),
			args: passTestArgs{
				decl: &dst.FuncDecl{
					Name: dst.NewIdent("foo"),
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{
								{
									Names: []*dst.Ident{dst.NewIdent("tx")},
									Type:  transactionArgumentType(),
								},
							},
						},
					},
				},
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "foo",
						Path: "baz",
					},
					Args: []dst.Expr{dst.NewIdent("tx")},
				},
			},
			wantStatements: nil,
			wantArgs:       []dst.Expr{dst.NewIdent("tx")},
			wantTc:         NewTransaction("tx"),
			wantErr:        nil,
		},
		{
			name: "pass transaction to function call with compound parameters",
			tc:   NewTransaction("tx"),
			args: passTestArgs{
				decl: &dst.FuncDecl{
					Name: dst.NewIdent("foo"),
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{
								{
									Names: []*dst.Ident{dst.NewIdent("foo"), dst.NewIdent("bar")},
									Type:  dst.NewIdent("int"),
								},
								{
									Names: []*dst.Ident{dst.NewIdent("tx")},
									Type:  transactionArgumentType(),
								},
							},
						},
					},
				},
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "foo",
						Path: "baz",
					},
					Args: []dst.Expr{dst.NewIdent("1"), dst.NewIdent("2"), dst.NewIdent("tx")},
				},
			},
			wantStatements: nil,
			wantArgs:       []dst.Expr{dst.NewIdent("1"), dst.NewIdent("2"), dst.NewIdent("tx")},
			wantTc:         NewTransaction("tx"),
			wantErr:        nil,
		},
		{
			name: "pass transaction to function with a context parameter",
			tc:   NewTransaction("tx"),
			args: passTestArgs{
				decl: &dst.FuncDecl{
					Name: dst.NewIdent("foo"),
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{
								{
									Names: []*dst.Ident{dst.NewIdent("ctx_param")},
									Type:  contextParameterType(),
								},
							},
						},
					},
				},
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "foo",
						Path: "baz",
					},
					Args: []dst.Expr{dst.NewIdent("context")},
				},
			},
			wantStatements: []dst.Stmt{
				codegen.WrapContext(dst.NewIdent("context"), dst.NewIdent("tx"), codegen.DefaultContextVariableName),
			},
			wantArgs: []dst.Expr{dst.NewIdent(codegen.DefaultContextVariableName)},
			wantTc:   NewContext("ctx_param"),
			wantErr:  nil,
		},
		{
			name: "pass transaction to function without valid parameter",
			tc:   NewTransaction("tx"),
			args: passTestArgs{
				decl: &dst.FuncDecl{
					Name: dst.NewIdent("foo"),
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{},
						},
					},
				},
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "foo",
						Path: "baz",
					},
					Args: []dst.Expr{},
				},
			},
			wantStatements: []dst.Stmt{},
			wantArgs:       []dst.Expr{},
			wantTc:         nil,
			wantErr:        NewPassError(NewTransaction("tx"), "no context or transaction argument found in function declaration foo"),
		},
	}

	testPass(t, tests)
}
