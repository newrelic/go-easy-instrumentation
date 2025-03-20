package util

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

const (
	ErrorType = "error"
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

	if astNode == nil {
		return nil
	}
	astExpr := astNode.(ast.Expr)
	return pkg.TypesInfo.TypeOf(astExpr)
}

func IsUnderlyingType(underlyingType types.Type, name string) bool {
	if underlyingType == nil {
		return false
	}

	typeString := underlyingType.String()
	return strings.Contains(typeString, name) || typeString == name
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

// Position returns the position of the node in the file
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

// WriteExpr returns a shortened string representation of the expression
// as go code.
//
// Warning: This may not be equivilent to how it appears in the source code!
func WriteExpr(expr dst.Expr, pkg *decorator.Package) string {
	if expr == nil || pkg == nil {
		return ""
	}

	astExpr := pkg.Decorator.Ast.Nodes[expr]
	if astExpr == nil {
		return ""
	}

	return types.ExprString(astExpr.(ast.Expr))
}

// IsError returns true if the type is an error type
func IsError(t types.Type) bool {
	if t == nil {
		return false
	}
	// if the variable is an error type, return it
	if t.String() == ErrorType {
		return true
	}

	// if the variable is a named error type, return it
	name, ok := t.(*types.Named)
	if !ok {
		return false
	}

	o := name.Obj()
	return o != nil && o.Pkg() == nil && o.Name() == "error"
}

// PrintNode returns a string representation of the node
// as go code.
//
// Warning: `gofmt` is applied to the output.
func PrintNode(pkg *decorator.Package, node dst.Node) string {
	if node == nil || pkg == nil {
		return ""
	}

	astNode := pkg.Decorator.Ast.Nodes[node]
	if astNode == nil {
		return fmt.Sprintf("%+v", node)
	}

	buf := &bytes.Buffer{}
	err := printer.Fprint(buf, pkg.Fset, astNode)
	if err != nil {
		return fmt.Sprintf("%+v", astNode)
	}

	return buf.String()
}

func IsUnitTest(pkg *decorator.Package, decl *dst.FuncDecl) bool {
	if decl == nil {
		return false
	}
	if decl.Name == nil {
		return false
	}
	if !strings.HasPrefix(decl.Name.Name, "Test") {
		return false
	}

	// Check if the function has a single parameter of type *testing.T
	if len(decl.Type.Params.List) != 1 {
		return false
	}

	if len(decl.Type.Params.List[0].Names) != 1 {
		return false
	}

	// Check if the parameter is of type *testing.T
	return IsUnderlyingType(TypeOf(decl.Type.Params.List[0].Type, pkg), "*testing.T")
}

func IsBenchmark(pkg *decorator.Package, decl *dst.FuncDecl) bool {
	if decl == nil {
		return false
	}
	if decl.Name == nil {
		return false
	}
	if !strings.HasPrefix(decl.Name.Name, "Benchmark") {
		return false
	}

	// Check if the function has a single parameter of type *testing.T
	if len(decl.Type.Params.List) != 1 {
		return false
	}

	if len(decl.Type.Params.List[0].Names) != 1 {
		return false
	}

	// Check if the parameter is of type *testing.T
	return IsUnderlyingType(TypeOf(decl.Type.Params.List[0].Type, pkg), "*testing.B")
}

func IsGenerated(decorator *decorator.Decorator, file *dst.File) bool {
	astNode := decorator.Ast.Nodes[file]
	if astFile, ok := astNode.(*ast.File); astFile != nil && ok {
		return ast.IsGenerated(astFile)
	}

	return false
}

func IsTestPackage(pkg *decorator.Package) bool {
	return strings.HasSuffix(pkg.ID, ".test") || pkg.ForTest != ""
}
