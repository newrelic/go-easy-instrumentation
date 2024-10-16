package tracecontext

import (
	"testing"

	"github.com/dave/dst"
)

func TestContext_AddParam(t *testing.T) {
	tests := []addParamTest{
		{
			name: "don't add argument to function with context parameter",
			tc:   NewContext("ctx"),
			funcDecl: &dst.FuncDecl{
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
		},
		{
			name: "add parameter to function without context parameter",
			tc:   NewContext("ctx"),
			funcDecl: &dst.FuncDecl{
				Type: &dst.FuncType{
					Params: &dst.FieldList{
						List: []*dst.Field{},
					},
				},
			},
			expect: &dst.Field{
				Names: []*dst.Ident{dst.NewIdent("ctx")},
				Type:  contextParameterType(),
			},
		},
	}
	testAddParam(t, tests)
}

func TestContext_Pass(t *testing.T) {
	tests := []passTest{
		{
			name: "pass context to function",
			tc:   NewContext("ctx"),
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
			wantStatements: nil,
			wantArgs:       []dst.Expr{dst.NewIdent("ctx")},
			wantTc:         NewContext("ctx"),
			wantErr:        nil,
		},
		{
			name: "pass context to function with compound parameters",
			tc:   NewContext("ctx"),
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
			wantStatements: nil,
			wantArgs: []dst.Expr{
				dst.NewIdent("1"),
				dst.NewIdent("2"),
				dst.NewIdent("ctx"),
			},
			wantTc:  NewContext("ctx"),
			wantErr: nil,
		},
		{
			name: "pass context to function with compound parameters and no context argument",
			tc:   NewContext("ctx"),
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
			wantStatements: nil,
			wantArgs: []dst.Expr{
				dst.NewIdent("1"),
				dst.NewIdent("2"),
				dst.NewIdent("ctx"),
			},
			wantTc:  NewContext("ctx"),
			wantErr: nil,
		},
		{
			name: "pass context to function without context parameter",
			tc:   NewContext("ctx"),
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
			wantStatements: nil,
			wantArgs:       []dst.Expr{},
			wantTc:         nil,
			wantErr:        NewPassError(NewContext("ctx"), "no context argument found in function declaration foo"),
		},
	}

	testPass(t, tests)
}
