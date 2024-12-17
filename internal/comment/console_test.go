package comment

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"golang.org/x/tools/go/packages"
)

func TestAddComment(t *testing.T) {
	dstNode1 := &dst.Ident{Name: "hi"}
	astNode1 := &ast.Ident{Name: "hi"}

	pkg := &decorator.Package{
		Decorator: &decorator.Decorator{
			Map: decorator.Map{
				Ast: decorator.AstMap{
					Nodes: map[dst.Node]ast.Node{
						dstNode1: astNode1,
					},
				},
			},
		},
		Package: &packages.Package{
			Fset: token.NewFileSet(),
		},
	}

	testPrinter := &ConsolePrinter{
		comments: []string{},
	}

	testPrinter.add(pkg, dstNode1, InfoConsoleHeader, "message", "additionalInfo")
	if len(testPrinter.comments) != 1 {
		t.Errorf("Expected 1 comment, got %d", len(testPrinter.comments))
	} else {
		expected := "Info: message\nadditionalInfo"
		if testPrinter.comments[0] != expected {
			t.Errorf("Expected %s, got %s", expected, testPrinter.comments[0])
		}
	}
}
