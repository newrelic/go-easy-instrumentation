package util

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// PackagePath returns the package path of the ident according to go types info
func PackagePath(ident *dst.Ident, pkg *decorator.Package) string {
	if ident == nil || pkg == nil {
		return ""
	}
	astNode := pkg.Decorator.Ast.Nodes[ident]
	var astIdent *ast.Ident
	switch v := astNode.(type) {
	case *ast.SelectorExpr:
		if v != nil {
			astIdent = v.Sel
		}
	case *ast.Ident:
		astIdent = v
	default:
		return ""
	}

	if pkg.TypesInfo != nil {
		uses, ok := pkg.TypesInfo.Uses[astIdent]
		if ok && uses.Pkg() != nil {
			return uses.Pkg().Path()
		}
	}
	return ""
}

// TypeOf returns the types.Type of the ident according to go types info
func TypeOf(expr dst.Expr, pkg *decorator.Package) types.Type {
	astNode := pkg.Decorator.Ast.Nodes[expr]
	astExpr := astNode.(ast.Expr)
	return pkg.TypesInfo.TypeOf(astExpr)
}

// FunctionName returns the name of the function being invoked in a call expression
func FunctionName(call *dst.CallExpr) string {
	if call == nil {
		return ""
	}
	switch v := call.Fun.(type) {
	case *dst.Ident:
		return v.Name
	case *dst.SelectorExpr:
		return v.Sel.Name
	}
	return ""
}

func Position(node dst.Node, pkg *decorator.Package) *token.Position {
	if node == nil || pkg == nil {
		return nil
	}

	astNode := pkg.Decorator.Ast.Nodes[node]
	if astNode == nil {
		return nil
	}

	pos := pkg.Fset.Position(astNode.Pos())
	return &pos
}

func WriteExpr(expr dst.Expr, pkg *decorator.Package) string {
	if expr == nil || pkg == nil {
		return ""
	}

	astExpr := pkg.Decorator.Ast.Nodes[expr].(ast.Expr)
	if astExpr == nil {
		return ""
	}

	return types.ExprString(astExpr)
}
