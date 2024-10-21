package tracecontext

import (
	"testing"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
)

func TestTransaction_Pass(t *testing.T) {
	tests := []passTest{
		{
			name: "func decl has txn param, call does not have txn arg",
			tc:   NewTransaction("tx", nil),
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
			wantArgs: []dst.Expr{dst.NewIdent("tx")},
			wantTc:   NewTransaction("tx", nil),
		},
		{
			name: "func decl has txn param, and func call has txn arg",
			tc:   NewTransaction("tx", nil),
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
			wantArgs: []dst.Expr{dst.NewIdent("tx")},
			wantTc:   NewTransaction("tx", nil),
		},
		{
			name: "func decl has compound parameters, and counts correctly",
			tc:   NewTransaction("tx", nil),
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
			wantArgs: []dst.Expr{dst.NewIdent("1"), dst.NewIdent("2"), dst.NewIdent("tx")},
			wantTc:   NewTransaction("tx", nil),
		},
		{
			name: "func decl has context parameter, context argument gets wrapped",
			tc:   NewTransaction("tx", nil),
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
					Args: []dst.Expr{dst.NewIdent("ctx")},
				},
			},
			wantArgs: []dst.Expr{codegen.WrapContextExpression(dst.NewIdent("ctx"), dst.NewIdent("tx"))},
			wantTc:   NewContext("ctx_param", nil),
		},
		{
			name: "func decl has context parameter, context argument gets wrapped",
			tc:   NewTransaction("tx", nil),
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
					Args: []dst.Expr{codegen.WrapContextExpression(dst.NewIdent("ctx"), dst.NewIdent("tx"))},
				},
			},
			wantArgs: []dst.Expr{codegen.WrapContextExpression(dst.NewIdent("ctx"), dst.NewIdent("tx"))},
			wantTc:   NewContext("ctx_param", nil),
		},
		{
			name: "function with no params",
			tc:   NewTransaction("tx", nil),
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
			wantArgs: []dst.Expr{dst.NewIdent("tx")},
			wantParams: []*dst.Field{
				{
					Names: []*dst.Ident{dst.NewIdent("tx")},
					Type:  transactionArgumentType(),
				},
			},
			wantTc: NewTransaction("tx", nil),
		},
	}

	testPass(t, tests)
}
