package tracecontext

import (
	"testing"

	"github.com/dave/dst"
)

func TestContext_Pass(t *testing.T) {
	tests := []passTest{
		{
			name: "pass context to function",
			tc:   NewContext("ctx", nil),
			args: passTestArgs{
				decl: &dst.FuncDecl{
					Name: dst.NewIdent("foo"),
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{
								{
									Names: []*dst.Ident{dst.NewIdent("ctx")},
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
					Args: []dst.Expr{},
				},
			},
			wantArgs: []dst.Expr{dst.NewIdent("ctx")},
			wantTc:   NewContext("ctx", nil),
		},
		{
			name: "pass context to function with compound parameters",
			tc:   NewContext("ctx", nil),
			args: passTestArgs{
				decl: &dst.FuncDecl{
					Name: dst.NewIdent("foo"),
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{
								{
									Names: []*dst.Ident{dst.NewIdent("foo"), dst.NewIdent("bar")},
									Type:  &dst.Ident{Name: "int"},
								},
								{
									Names: []*dst.Ident{dst.NewIdent("ctx")},
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
					Args: []dst.Expr{
						dst.NewIdent("1"),
						dst.NewIdent("2"),
						dst.NewIdent("ctx"),
					},
				},
			},
			wantArgs: []dst.Expr{
				dst.NewIdent("1"),
				dst.NewIdent("2"),
				dst.NewIdent("ctx"),
			},
			wantTc: NewContext("ctx", nil),
		},
		{
			name: "pass context to function with compound parameters and no context argument",
			tc:   NewContext("ctx", nil),
			args: passTestArgs{
				decl: &dst.FuncDecl{
					Name: dst.NewIdent("foo"),
					Type: &dst.FuncType{
						Params: &dst.FieldList{
							List: []*dst.Field{
								{
									Names: []*dst.Ident{dst.NewIdent("foo"), dst.NewIdent("bar")},
									Type:  &dst.Ident{Name: "int"},
								},
								{
									Names: []*dst.Ident{dst.NewIdent("ctx")},
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
					Args: []dst.Expr{
						dst.NewIdent("1"),
						dst.NewIdent("2"),
					},
				},
			},
			wantArgs: []dst.Expr{
				dst.NewIdent("1"),
				dst.NewIdent("2"),
				dst.NewIdent("ctx"),
			},
			wantTc: NewContext("ctx", nil),
		},
		{
			name: "pass context to function without context parameter",
			tc:   NewContext("ctx", nil),
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
			wantArgs: []dst.Expr{dst.NewIdent("ctx")},
			wantTc:   NewContext("ctx", nil),
		},
	}

	testPass(t, tests)
}
