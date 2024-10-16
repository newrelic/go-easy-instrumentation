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
	astNode1 := &ast.Ident{Name: "hi", NamePos: token.Pos(0)}

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

	testPrinter.Add(pkg, dstNode1, InfoHeader, "message", "additionalInfo")
	if len(testPrinter.comments) != 1 {
		t.Errorf("Expected 1 comment, got %d", len(testPrinter.comments))
	} else {
		expected := "NR INFO - message\n\tadditionalInfo"
		if testPrinter.comments[0] != expected {
			t.Errorf("Expected %s, got %s", expected, testPrinter.comments[0])
		}
	}
}
